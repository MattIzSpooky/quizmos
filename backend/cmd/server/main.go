package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/config"
	"github.com/mattizspooky/quizmos/backend/internal/game"
	"github.com/mattizspooky/quizmos/backend/internal/handlers"
	"github.com/mattizspooky/quizmos/backend/internal/httpserver"
	"github.com/mattizspooky/quizmos/backend/internal/question"
	"github.com/mattizspooky/quizmos/backend/internal/quiz"
	"github.com/mattizspooky/quizmos/backend/internal/storage"
	"github.com/mattizspooky/quizmos/backend/internal/telemetry"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Telemetry must be set up before anything else logs, since it points
	// slog.Default() at the handler everything downstream uses.
	shutdownTelemetry, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			slog.Error("telemetry shutdown", "error", err)
		}
	}()

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		slog.Error("parse database url", "error", err)
		os.Exit(1)
	}
	// Gives every query, batch, and pool-acquire a span (see also
	// PoolStats — otelpgx.RecordStats — if pool exhaustion/latency metrics
	// are ever needed; out of scope for now, this is tracing only), nested
	// under whatever span is active on the ctx the query was made with —
	// the request's HTTP span, or a websocket message's span.
	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	keycloak := auth.NewKeycloak(cfg.KeycloakIssuer, cfg.KeycloakInternalIssuer, cfg.AdminRole)
	if err := keycloak.StartRefresh(ctx, 10*time.Minute); err != nil {
		slog.Error("keycloak JWKS", "error", err)
		os.Exit(1)
	}

	store, err := storage.New(storage.Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Bucket:    cfg.S3Bucket,
		UseSSL:    cfg.S3UseSSL,
		PublicURL: cfg.S3PublicURL,
	})
	if err != nil {
		slog.Error("storage client", "error", err)
		os.Exit(1)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		slog.Error("ensure media bucket", "error", err)
		os.Exit(1)
	}

	questionSvc := question.New(pool, store)
	quizSvc := quiz.New(pool, store, questionSvc)
	gameSvc := game.New(pool, questionSvc)
	questionHandler := question.NewHandler(questionSvc, keycloak)
	hub := ws.NewHub(gameSvc, quizSvc, cfg.AllowedOrigins)
	handler := handlers.New(gameSvc, quizSvc, questionHandler, hub)

	var shuttingDown atomic.Bool
	router, err := httpserver.New(httpserver.Options{
		StrictHandler:  handler,
		Keycloak:       keycloak,
		Hub:            hub,
		AllowedOrigins: cfg.AllowedOrigins,
		DB:             pool,
		ShuttingDown:   &shuttingDown,
	})
	if err != nil {
		slog.Error("build router", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{Addr: cfg.Addr, Handler: router}

	go func() {
		slog.Info("quizmos backend listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("serve", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	// Flip readyz to unready before srv.Shutdown even starts draining
	// in-flight requests, so Kubernetes (or any load balancer polling it)
	// stops sending new traffic here as early into termination as possible.
	shuttingDown.Store(true)
	slog.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "error", err)
	}
}
