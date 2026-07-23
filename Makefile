SHELL := /bin/bash

BACKEND := backend
FRONTEND := frontend
TOOLS := tools
API_DIR := api
DEPLOY := deploy

DATABASE_URL ?= postgres://quizmos:quizmos-dev@localhost:5432/quizmos?sslmode=disable

.PHONY: help
help:
	@echo "quizmos — common targets:"
	@echo "  make tools              install/pin codegen tool dependencies (Go + npm)"
	@echo "  make generate           regenerate all codegen output (openapi + asyncapi + sqlc)"
	@echo "  make generate-openapi   regenerate REST types/server (backend) and types/client (frontend)"
	@echo "  make generate-asyncapi  regenerate websocket types (Go + TypeScript) from AsyncAPI"
	@echo "  make generate-sqlc      regenerate typed Postgres query code"
	@echo "  make check-generated    fail if generated output is stale (regenerate + git diff)"
	@echo "  make migrate-up         apply database migrations"
	@echo "  make migrate-down       roll back the last migration"
	@echo "  make migrate-new name=add_foo   scaffold a new migration pair"
	@echo "  make dev-up / dev-down  start/stop local Postgres + Keycloak via docker compose"
	@echo "  make run-backend        run the Go backend"
	@echo "  make run-frontend       run the React dev server"
	@echo "  make build              build backend binary + frontend production bundle"
	@echo "  make test               run everything: unit tests, e2e feature tests, frontend typecheck"
	@echo "  make test-unit          run fast backend unit tests only (no Docker required)"
	@echo "  make test-e2e           run the Gherkin/godog e2e suite (needs a Docker daemon)"

# ---------------------------------------------------------------------------
# Tooling
# ---------------------------------------------------------------------------

.PHONY: tools
tools:
	cd $(BACKEND) && go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	cd $(BACKEND) && go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	cd $(BACKEND) && go get -tool github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	cd $(TOOLS) && npm install
	cd $(FRONTEND) && npm install

# ---------------------------------------------------------------------------
# Code generation
# ---------------------------------------------------------------------------

.PHONY: generate
generate: generate-openapi generate-asyncapi generate-sqlc

.PHONY: generate-openapi
generate-openapi:
	cd $(BACKEND) && go tool oapi-codegen -config internal/api/cfg.yaml ../$(API_DIR)/openapi.yaml
	cd $(FRONTEND) && npx openapi-typescript ../$(API_DIR)/openapi.yaml -o app/lib/api/schema.gen.ts

.PHONY: generate-asyncapi
generate-asyncapi:
	cd $(TOOLS) && node extract-asyncapi-schemas.mjs
	cd $(TOOLS) && npx quicktype --src-lang schema --lang go --package ws \
		--out ../$(BACKEND)/internal/ws/types.gen.go ../$(API_DIR)/asyncapi/generated-schemas/*.schema.json
	cd $(TOOLS) && npx quicktype --src-lang schema --lang typescript \
		--out ../$(FRONTEND)/app/lib/ws/types.gen.ts ../$(API_DIR)/asyncapi/generated-schemas/*.schema.json

.PHONY: generate-sqlc
generate-sqlc:
	cd $(BACKEND) && go tool sqlc generate

.PHONY: check-generated
check-generated: generate
	git diff --exit-code -- $(BACKEND)/internal/api $(BACKEND)/internal/db/sqlc $(BACKEND)/internal/ws/types.gen.go $(FRONTEND)/app/lib/api $(FRONTEND)/app/lib/ws/types.gen.ts

# ---------------------------------------------------------------------------
# Database migrations (golang-migrate; the postgres driver requires the
# 'postgres' build tag, which `go tool` alone can't pass through, so this
# uses `go run` against the version pinned in go.mod instead)
# ---------------------------------------------------------------------------

.PHONY: migrate-up
migrate-up:
	cd $(BACKEND) && go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		-database "$(DATABASE_URL)" -path internal/db/migrations up

.PHONY: migrate-down
migrate-down:
	cd $(BACKEND) && go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		-database "$(DATABASE_URL)" -path internal/db/migrations down 1

.PHONY: migrate-new
migrate-new:
	@if [ -z "$(name)" ]; then echo "usage: make migrate-new name=add_foo"; exit 1; fi
	cd $(BACKEND) && go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
		create -ext sql -dir internal/db/migrations -seq $(name)

# ---------------------------------------------------------------------------
# Local dev environment
# ---------------------------------------------------------------------------

.PHONY: dev-up
dev-up:
	docker compose -f $(DEPLOY)/docker-compose.yml up -d

.PHONY: dev-down
dev-down:
	docker compose -f $(DEPLOY)/docker-compose.yml down

.PHONY: run-backend
run-backend:
	cd $(BACKEND) && go run ./cmd/server

.PHONY: run-frontend
run-frontend:
	cd $(FRONTEND) && npm run dev

# ---------------------------------------------------------------------------
# Build & test
# ---------------------------------------------------------------------------

.PHONY: build
build:
	cd $(BACKEND) && go build -o bin/server ./cmd/server
	cd $(FRONTEND) && npm run build

.PHONY: test
test: test-unit test-e2e
	cd $(FRONTEND) && npm run typecheck

# Fast, hermetic unit tests: no Docker, no external services.
.PHONY: test-unit
test-unit:
	cd $(BACKEND) && go test $$(go list ./... | grep -v /e2e)

# Gherkin/godog end-to-end suite (backend/features/*.feature): boots real
# Postgres + Keycloak testcontainers and drives the actual HTTP/websocket
# server. Requires a reachable Docker daemon — set DOCKER_HOST if it's not
# at the default socket (e.g. Docker Desktop).
.PHONY: test-e2e
test-e2e:
	cd $(BACKEND) && go test ./e2e/... -v -timeout 5m
