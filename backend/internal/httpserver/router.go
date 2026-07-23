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

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

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
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Logger, chimw.Recoverer)
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

	return r, nil
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
