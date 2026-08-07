// Package e2e runs Gherkin/godog feature scenarios against a real
// quizmos backend: an httptest server wired to the actual chi router,
// service layer and websocket hub, backed by a real Postgres, a real
// Keycloak, and a real MinIO, all started as testcontainers. Nothing
// here is mocked — this is the same code path that runs in production,
// just pointed at disposable infrastructure.
package e2e

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/game"
	"github.com/mattizspooky/quizmos/backend/internal/handlers"
	"github.com/mattizspooky/quizmos/backend/internal/httpserver"
	"github.com/mattizspooky/quizmos/backend/internal/question"
	"github.com/mattizspooky/quizmos/backend/internal/quiz"
	"github.com/mattizspooky/quizmos/backend/internal/storage"
	"github.com/mattizspooky/quizmos/backend/internal/telemetry"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

const (
	e2eAdminUsername  = "admin@quizmos.dev"
	e2eAdminPassword  = "quizmos-dev"
	e2eNoRoleUsername = "no-role@quizmos.dev"
	e2eNoRolePassword = "quizmos-dev"
	e2eClientID       = "quizmos-e2e"
	e2eAdminRole      = "quiz-admin"
	e2eMediaBucket    = "quizmos-media-test"
)

// environment owns every long-lived resource for one test run: both
// containers, the DB pool, and the httptest server. One environment is
// shared across all scenarios in the suite; each scenario gets a clean
// slate via truncateAll in a Before hook instead of paying container
// startup cost per scenario.
type environment struct {
	pgContainer    *tcpostgres.PostgresContainer
	kcContainer    testcontainers.Container
	minioContainer *tcminio.MinioContainer

	pool   *pgxpool.Pool
	server *httptest.Server

	baseURL      string // http://.../api
	wsBaseURL    string // ws://.../ws
	keycloakURL  string // http://.../realms/quizmos
	mediaBaseURL string // http://.../quizmos-media-test — where uploaded media is publicly fetchable
}

func repoRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// backend/e2e/environment.go -> repo root
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func startEnvironment(ctx context.Context) (*environment, error) {
	// The e2e suite never calls telemetry.Setup (there's no OTLP collector
	// to export to here), so without this slog.Default() would stay on
	// its bare built-in fallback — the old-style "2026/07/27 19:59:23
	// INFO msg key=value" format — instead of matching what the real
	// server actually logs. It's discarded by default rather than written
	// to stdout: every request the server handles logs at least one line,
	// which would otherwise bury godog's own --format=pretty scenario
	// output. Set E2E_LOG=1 to see it (e.g. while debugging a failure).
	logOutput := io.Discard
	if os.Getenv("E2E_LOG") != "" {
		logOutput = os.Stdout
	}
	slog.SetDefault(slog.New(telemetry.NewStdoutHandler("json", logOutput)))

	root := repoRoot()
	migrationsDir := filepath.Join(root, "backend", "internal", "db", "migrations")
	migrationFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("glob migration files: %w", err)
	}
	sort.Strings(migrationFiles) // apply in numeric-prefix order, same as golang-migrate
	realmExport := filepath.Join(root, "deploy", "keycloak", "realm-export.json")

	pgC, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("quizmos"),
		tcpostgres.WithUsername("quizmos"),
		tcpostgres.WithPassword("quizmos-dev"),
		tcpostgres.WithInitScripts(migrationFiles...),
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

	minioC, err := tcminio.Run(ctx, "minio/minio:RELEASE.2025-09-07T16-13-09Z")
	if err != nil {
		_ = pgC.Terminate(ctx)
		_ = kcContainer.Terminate(ctx)
		return nil, fmt.Errorf("start minio container: %w", err)
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

	kcAuth := auth.NewKeycloak(issuer, issuer, e2eAdminRole)
	if err := kcAuth.StartRefresh(ctx, time.Minute); err != nil {
		return nil, fmt.Errorf("fetch JWKS from test keycloak: %w", err)
	}

	minioEndpoint, err := minioC.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("minio connection string: %w", err)
	}
	mediaBaseURL := "http://" + minioEndpoint
	store, err := storage.New(storage.Config{
		Endpoint:  minioEndpoint,
		AccessKey: minioC.Username,
		SecretKey: minioC.Password,
		Bucket:    e2eMediaBucket,
		PublicURL: mediaBaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("build storage client: %w", err)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		return nil, fmt.Errorf("ensure media bucket: %w", err)
	}

	questionSvc := question.New(pool, store)
	quizSvc := quiz.New(pool, store, questionSvc)
	gameSvc := game.New(pool, questionSvc)
	questionHandler := question.NewHandler(questionSvc, kcAuth)
	hub := ws.NewHub(gameSvc, quizSvc, []string{"*"})
	h := handlers.New(gameSvc, quizSvc, questionHandler, hub)
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
		pgContainer:    pgC,
		kcContainer:    kcContainer,
		minioContainer: minioC,
		pool:           pool,
		server:         server,
		baseURL:        server.URL + "/api",
		wsBaseURL:      "ws" + strings.TrimPrefix(server.URL, "http") + "/ws",
		mediaBaseURL:   mediaBaseURL,
		keycloakURL:    keycloakBase,
	}, nil
}

func (e *environment) shutdown(ctx context.Context) {
	e.server.Close()
	e.pool.Close()
	_ = e.kcContainer.Terminate(ctx)
	_ = e.pgContainer.Terminate(ctx)
	_ = e.minioContainer.Terminate(ctx)
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
	return e.tokenFor(ctx, e2eAdminUsername, e2eAdminPassword)
}

// noRoleToken is the same real-Keycloak token exchange as adminToken, but
// for a seeded user with no realm roles at all — a validly-authenticated
// account that just isn't quiz-admin, for verifying admin endpoints
// reject it with 403 rather than 401.
func (e *environment) noRoleToken(ctx context.Context) (string, error) {
	return e.tokenFor(ctx, e2eNoRoleUsername, e2eNoRolePassword)
}

func (e *environment) tokenFor(ctx context.Context, username, password string) (string, error) {
	form := url.Values{
		"client_id":  {e2eClientID},
		"grant_type": {"password"},
		"username":   {username},
		"password":   {password},
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
