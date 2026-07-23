package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/config"
	"github.com/mattizspooky/quizmos/backend/internal/handlers"
	"github.com/mattizspooky/quizmos/backend/internal/httpserver"
	"github.com/mattizspooky/quizmos/backend/internal/service"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	keycloak := auth.NewKeycloak(cfg.KeycloakIssuer, cfg.AdminRole)
	if err := keycloak.StartRefresh(ctx, 10*time.Minute); err != nil {
		log.Fatalf("keycloak JWKS: %v", err)
	}

	svc := service.New(pool)
	hub := ws.NewHub(svc, cfg.AllowedOrigins)
	handler := handlers.New(svc, hub)

	router, err := httpserver.New(httpserver.Options{
		StrictHandler:  handler,
		Keycloak:       keycloak,
		Hub:            hub,
		AllowedOrigins: cfg.AllowedOrigins,
	})
	if err != nil {
		log.Fatalf("build router: %v", err)
	}

	srv := &http.Server{Addr: cfg.Addr, Handler: router}

	go func() {
		log.Printf("quizmos backend listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
