package config

import (
	"fmt"
	"os"
)

type Config struct {
	Addr           string
	DatabaseURL    string
	KeycloakIssuer string
	AdminRole      string
	AllowedOrigins []string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:           getEnv("ADDR", ":8080"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		KeycloakIssuer: getEnv("KEYCLOAK_ISSUER", "http://localhost:8081/realms/quizmos"),
		AdminRole:      getEnv("ADMIN_ROLE", "quiz-admin"),
		AllowedOrigins: []string{getEnv("FRONTEND_ORIGIN", "http://localhost:5173")},
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
