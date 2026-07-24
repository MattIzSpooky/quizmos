// Package httpserver wires the chi router: the OpenAPI-validated /api
// subtree (REST, oapi-codegen strict server) and the raw /ws subtree
// (websocket upgrade, outside the OpenAPI spec).
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/middleware"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

type Options struct {
	StrictHandler  api.StrictServerInterface
	Keycloak       *auth.Keycloak
	Hub            *ws.Hub
	AllowedOrigins []string

	// DB is pinged by GET /readyz to confirm the database is actually
	// reachable, not just that the process is running.
	DB *pgxpool.Pool
	// ShuttingDown, if set, is checked by GET /readyz — main.go flips it
	// the instant it catches SIGTERM/SIGINT, before graceful shutdown
	// even begins, so a Kubernetes readiness probe (or any load balancer
	// polling it) stops routing new traffic here as early as possible.
	ShuttingDown *atomic.Bool
}

func New(opts Options) (http.Handler, error) {
	spec, err := api.GetSwagger()
	if err != nil {
		return nil, err
	}
	// Host/server validation isn't useful behind a dev proxy or in
	// production behind a load balancer that rewrites Host.
	spec.Servers = nil

	r := chi.NewRouter()
	// otelhttp (wrapped around the whole router below) assigns each request
	// its trace ID before any of this runs, so RequestLogger — which reads
	// that trace ID — supersedes chimw.Logger, and chimw.RequestID's opaque
	// ID is redundant.
	r.Use(chimw.RealIP, chimw.Recoverer, middleware.RequestLogger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   opts.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Client-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api", func(apiRouter chi.Router) {
		apiRouter.Use(oapimiddleware.OapiRequestValidatorWithOptions(spec, &oapimiddleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: opts.Keycloak.AuthenticationFunc,
			},
			ErrorHandlerWithOpts: jsonErrorHandler,
			Prefix:               "/api",
			// The question-media upload/delete routes check auth
			// themselves (see internal/handlers/media.go) and skip
			// validation entirely: the standard validator fully reads a
			// multipart/form-data body to validate it, which would
			// consume the file before the handler could stream it to
			// storage.
			Skipper: func(r *http.Request) bool {
				return strings.HasSuffix(r.URL.Path, "/media")
			},
		}))
		strict := api.NewStrictHandler(opts.StrictHandler, nil)
		api.HandlerFromMux(strict, apiRouter)
	})

	r.With(middleware.ClientID).Get("/ws/games/{code}", opts.Hub.Upgrade)

	// Liveness/readiness for Kubernetes (or any load balancer health
	// check). healthz only confirms the process is up and serving —
	// no dependency checks — since a slow/unreachable dependency should
	// fail readiness, not get the pod killed and restarted by a failing
	// liveness probe. readyz is the one that actually checks dependencies.
	r.Get("/healthz", healthzHandler)
	r.Get("/readyz", readyzHandler(opts))

	// otelhttp starts (and, on response, ends) a span per request — the
	// trace ID everything else here reads (RequestLogger, the X-Trace-Id
	// header, and the websocket connection ID minted in ws.Hub.Upgrade)
	// comes from this span, so it must wrap everything else. Health/ready
	// checks are excluded: a load balancer polling every few seconds
	// would otherwise fill traces with high-volume, zero-value spans.
	return otelhttp.NewHandler(r, "quizmos-backend", otelhttp.WithFilter(func(r *http.Request) bool {
		return r.URL.Path != "/healthz" && r.URL.Path != "/readyz"
	})), nil
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// readyzHandler reports whether the backend is ready to serve real
// traffic: the database is reachable, Keycloak's JWKS is loaded, and the
// process isn't already draining for shutdown. Kubernetes (or any load
// balancer) should stop routing new requests here whenever this returns
// a non-2xx status.
func readyzHandler(opts Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]string{}
		ready := true

		if opts.ShuttingDown != nil && opts.ShuttingDown.Load() {
			checks["shutdown"] = "in progress"
			ready = false
		}

		if err := opts.DB.Ping(r.Context()); err != nil {
			checks["database"] = err.Error()
			ready = false
		} else {
			checks["database"] = "ok"
		}

		if !opts.Keycloak.Ready() {
			checks["keycloak"] = "JWKS not loaded yet"
			ready = false
		} else {
			checks["keycloak"] = "ok"
		}

		status := "ok"
		statusCode := http.StatusOK
		if !ready {
			status = "unavailable"
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "checks": checks})
	}
}

func jsonErrorHandler(ctx context.Context, err error, w http.ResponseWriter, r *http.Request, opts oapimiddleware.ErrorHandlerOpts) {
	status := opts.StatusCode

	var secErr *openapi3filter.SecurityRequirementsError
	if errors.As(err, &secErr) {
		for _, inner := range secErr.Errors {
			var withStatus interface{ HTTPStatus() int }
			if errors.As(inner, &withStatus) {
				status = withStatus.HTTPStatus()
			}
		}
	}

	code := "bad_request"
	switch status {
	case http.StatusUnauthorized:
		code = "unauthorized"
	case http.StatusForbidden:
		code = "forbidden"
	case http.StatusNotFound:
		code = "not_found"
	case http.StatusInternalServerError:
		code = "internal_error"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": err.Error(),
	})
}
