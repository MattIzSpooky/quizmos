package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Addr           string
	DatabaseURL    string
	KeycloakIssuer string
	AdminRole      string
	AllowedOrigins []string

	// S3* configures the MinIO/S3-compatible bucket question media
	// (images, audio) is stored in. S3Endpoint is where the backend
	// itself reaches it (host:port, no scheme); S3PublicURL is the
	// scheme+host a browser uses to fetch objects directly — usually a
	// different address than S3Endpoint.
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool
	S3PublicURL string
}

func Load() (Config, error) {
	useSSL, err := strconv.ParseBool(getEnv("S3_USE_SSL", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("S3_USE_SSL: %w", err)
	}
	cfg := Config{
		Addr:           getEnv("ADDR", ":8080"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		KeycloakIssuer: getEnv("KEYCLOAK_ISSUER", "http://localhost:8081/realms/quizmos"),
		AdminRole:      getEnv("ADMIN_ROLE", "quiz-admin"),
		AllowedOrigins: []string{getEnv("FRONTEND_ORIGIN", "http://localhost:5173")},
		S3Endpoint:     getEnv("S3_ENDPOINT", "localhost:9000"),
		S3AccessKey:    getEnv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:    getEnv("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:       getEnv("S3_BUCKET", "quizmos-media"),
		S3UseSSL:       useSSL,
		S3PublicURL:    getEnv("S3_PUBLIC_URL", "http://localhost:9000"),
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
