// Package e2e runs Gherkin/godog feature scenarios against a real
// quizmos backend: an httptest server wired to the actual chi router,
// service layer and websocket hub, backed by a real Postgres and a real
// Keycloak, both started as testcontainers. Nothing here is mocked —
// this is the same code path that runs in production, just pointed at
// disposable infrastructure.
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/handlers"
	"github.com/mattizspooky/quizmos/backend/internal/httpserver"
	"github.com/mattizspooky/quizmos/backend/internal/service"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

const (
	e2eAdminUsername = "admin@quizmos.dev"
	e2eAdminPassword = "quizmos-dev"
	e2eClientID      = "quizmos-e2e"
	e2eAdminRole     = "quiz-admin"
)

// environment owns every long-lived resource for one test run: both
// containers, the DB pool, and the httptest server. One environment is
// shared across all scenarios in the suite; each scenario gets a clean
// slate via truncateAll in a Before hook instead of paying container
// startup cost per scenario.
type environment struct {
	pgContainer *tcpostgres.PostgresContainer
	kcContainer testcontainers.Container

	pool   *pgxpool.Pool
	server *httptest.Server

	baseURL     string // http://.../api
	wsBaseURL   string // ws://.../ws
	keycloakURL string // http://.../realms/quizmos
}

func repoRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// backend/e2e/environment.go -> repo root
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func startEnvironment(ctx context.Context) (*environment, error) {
	root := repoRoot()
	migrationSQL := filepath.Join(root, "backend", "internal", "db", "migrations", "000001_init.up.sql")
	realmExport := filepath.Join(root, "deploy", "keycloak", "realm-export.json")

	pgC, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("quizmos"),
		tcpostgres.WithUsername("quizmos"),
		tcpostgres.WithPassword("quizmos-dev"),
		tcpostgres.WithInitScripts(migrationSQL),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	kcContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "quay.io/keycloak/keycloak:26.4",
			Cmd:          []string{"start-dev", "--import-realm"},
			ExposedPorts: []string{"8080/tcp"},
			Env: map[string]string{
				"KC_BOOTSTRAP_ADMIN_USERNAME": "admin",
				"KC_BOOTSTRAP_ADMIN_PASSWORD": "admin",
			},
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      realmExport,
					ContainerFilePath: "/opt/keycloak/data/import/realm-export.json",
					FileMode:          0o444,
				},
			},
			WaitingFor: wait.ForHTTP("/realms/quizmos/.well-known/openid-configuration").
				WithPort("8080/tcp").
				WithStartupTimeout(2 * time.Minute),
		},
	})
	if err != nil {
		_ = pgC.Terminate(ctx)
		return nil, fmt.Errorf("start keycloak container: %w", err)
	}

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("postgres connection string: %w", err)
	}
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	kcHost, err := kcContainer.Host(ctx)
	if err != nil {
		return nil, err
	}
	kcPort, err := kcContainer.MappedPort(ctx, "8080/tcp")
	if err != nil {
		return nil, err
	}
	keycloakBase := fmt.Sprintf("http://%s:%s", kcHost, kcPort.Port())
	issuer := keycloakBase + "/realms/quizmos"

	kcAuth := auth.NewKeycloak(issuer, e2eAdminRole)
	if err := kcAuth.StartRefresh(ctx, time.Minute); err != nil {
		return nil, fmt.Errorf("fetch JWKS from test keycloak: %w", err)
	}

	svc := service.New(pool)
	hub := ws.NewHub(svc, []string{"*"})
	h := handlers.New(svc, hub)
	router, err := httpserver.New(httpserver.Options{
		StrictHandler:  h,
		Keycloak:       kcAuth,
		Hub:            hub,
		AllowedOrigins: []string{"*"},
	})
	if err != nil {
		return nil, fmt.Errorf("build router: %w", err)
	}

	server := httptest.NewServer(router)

	return &environment{
		pgContainer: pgC,
		kcContainer: kcContainer,
		pool:        pool,
		server:      server,
		baseURL:     server.URL + "/api",
		wsBaseURL:   "ws" + strings.TrimPrefix(server.URL, "http") + "/ws",
		keycloakURL: keycloakBase,
	}, nil
}

func (e *environment) shutdown(ctx context.Context) {
	e.server.Close()
	e.pool.Close()
	_ = e.kcContainer.Terminate(ctx)
	_ = e.pgContainer.Terminate(ctx)
}

// truncateAll resets all tables between scenarios so each one starts
// from a clean database without paying container startup cost again.
func (e *environment) truncateAll(ctx context.Context) error {
	_, err := e.pool.Exec(ctx,
		"TRUNCATE answers, players, question_options, questions, games, quizzes RESTART IDENTITY CASCADE")
	return err
}

// adminToken exchanges the seeded admin user's credentials for an access
// token via Keycloak's real token endpoint (direct grant, test-only
// client), exercising the same tokens the production auth middleware
// validates.
func (e *environment) adminToken(ctx context.Context) (string, error) {
	form := url.Values{
		"client_id":  {e2eClientID},
		"grant_type": {"password"},
		"username":   {e2eAdminUsername},
		"password":   {e2eAdminPassword},
		"scope":      {"openid roles"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.keycloakURL+"/realms/quizmos/protocol/openid-connect/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := decodeJSON(resp.Body, &body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("keycloak token request failed: %s: %s", body.Error, body.ErrorDesc)
	}
	return body.AccessToken, nil
}
