# Quizmos

A live trivia/pub-quiz app: an admin runs a quiz from a control panel and
players join on their phones with a code (or a QR scan) and answer
multiple-choice questions in real time. No player registration — a
`client_id` is minted in the browser on first visit and that's their whole
identity for the game.

- **Backend:** Go, chi router, Postgres (sqlc), websockets for live gameplay
- **Frontend:** React Router (framework mode), Tailwind v4
- **Auth:** Keycloak (OIDC/JWT) for the admin side only — players need no
  account at all
- **Spec-first codegen:** REST from OpenAPI (oapi-codegen + openapi-typescript),
  websocket payloads from AsyncAPI (quicktype)

## How it works

- Anyone can **join** a game by its 6-character code while it's in the
  lobby. Once the host starts the game, new joins are rejected.
- The **admin** authenticates via Keycloak (realm role `quiz-admin`),
  authors quizzes (multiple-choice questions, optionally timed), creates
  games from them, and runs the live session: start, advance, end, kick
  players from the lobby, review any earlier question read-only without
  disturbing live play, or reset a question's answers if something went
  wrong.
- Gameplay itself — question start/end, answer submission, scoring,
  leaderboard, presence — is driven entirely over a single websocket
  channel per game, described by `api/asyncapi.yaml`.
- Most everything else (auth, quiz authoring, game/player admin actions)
  is plain request/response REST, described by `api/openapi.yaml`.

## Repo layout

```
api/        OpenAPI (REST) and AsyncAPI (websocket) specs — the source of truth
backend/    Go server: chi handlers, service layer, sqlc queries, websocket hub
frontend/   React Router app: player join/play screens, admin console
tools/      Node-side codegen helpers (AsyncAPI → quicktype bridge)
deploy/     docker-compose for local Postgres + Keycloak
```

## Prerequisites

- Go (see `backend/go.mod` for the version)
- Node.js + npm
- Docker (for local Postgres/Keycloak, and for the e2e test suite, which
  spins up real containers via testcontainers-go)
- `make`

## Getting started

```bash
# 1. Install codegen tools and JS dependencies
make tools

# 2. Start Postgres + Keycloak
make dev-up

# 3. Apply database migrations
make migrate-up

# 4. Run the backend and frontend (in separate terminals)
make run-backend
make run-frontend
```

The frontend dev server runs at `http://localhost:5173`, the backend at
`http://localhost:8080`. Sign in to the admin console at `/admin/login`
with the seeded Keycloak user `admin@quizmos.dev` / `quizmos-dev` (from
`deploy/keycloak/realm-export.json`).

## Running the whole stack in Docker (demo)

For a quick demo, or anywhere you'd rather not install Go/Node locally,
`docker-compose.yml` at the repo root runs everything — Postgres,
Keycloak, a one-shot migration job, the backend, and the frontend — built
from `backend/Dockerfile` and `frontend/Dockerfile` (both multi-stage):

```bash
make demo-up      # builds images and starts the whole stack in the background
make demo-down    # tears it down
```

Once it's up, the frontend is at `http://localhost:5173` and the backend
at `http://localhost:8080`, same as local dev — sign in with the same
seeded Keycloak user. `make dev-up`/`dev-down` remain for the
Postgres+Keycloak-only workflow above, if you'd rather run the backend and
frontend natively while iterating.

## Code generation

Nothing under a `.gen.` filename (or `internal/api`, `internal/db/sqlc`,
`internal/ws/types.gen.go`, `app/lib/api`, `app/lib/ws/types.gen.ts`) is
hand-written — it's regenerated from `api/openapi.yaml`, `api/asyncapi.yaml`,
and the SQL under `backend/internal/db/queries`. After changing any of
those:

```bash
make generate            # regenerate everything
make generate-openapi    # just REST types/server + client
make generate-asyncapi   # just websocket types (Go + TypeScript)
make generate-sqlc       # just typed Postgres query code
make check-generated     # fail if generated output is stale (CI-friendly)
```

## Database migrations

```bash
make migrate-up
make migrate-down
make migrate-new name=add_foo   # scaffold a new migration pair
```

## Testing

```bash
make test         # everything: unit tests, e2e suite, frontend typecheck
make test-unit     # fast backend unit tests, no Docker required
make test-e2e      # Gherkin/godog e2e suite (needs a reachable Docker daemon)
```

The e2e suite (`backend/features/*.feature`, driven by `backend/e2e`) boots
real Postgres and Keycloak containers and drives the actual HTTP and
websocket server end to end — no mocks. Feature coverage includes quiz
authoring, game lifecycle (joining, kicking, reconnecting), live gameplay
and scoring, question navigation (going back/reviewing), per-quiz timing,
and resetting a question's answers.

If your Docker daemon isn't at the default socket (e.g. Docker Desktop),
point `DOCKER_HOST` at it first:

```bash
export DOCKER_HOST="unix:///path/to/docker.sock"
```

## Building

```bash
make build   # backend binary (backend/bin/server) + frontend production bundle
```

## Configuration

The backend reads its configuration from environment variables:

| Variable          | Default                                      | Purpose                              |
| ------------------ | --------------------------------------------- | ------------------------------------- |
| `ADDR`             | `:8080`                                       | Listen address                        |
| `DATABASE_URL`     | *(required)*                                  | Postgres connection string            |
| `KEYCLOAK_ISSUER`  | `http://localhost:8081/realms/quizmos`        | Keycloak realm issuer URL             |
| `ADMIN_ROLE`       | `quiz-admin`                                  | Realm role required for admin routes  |
| `FRONTEND_ORIGIN`  | `http://localhost:5173`                       | Allowed CORS origin                   |

The frontend reads these at build/dev time (Vite `import.meta.env`):

| Variable                 | Default                                | Purpose                    |
| ------------------------- | --------------------------------------- | ---------------------------- |
| `VITE_API_BASE_URL`       | `http://localhost:8080/api`            | REST API base URL           |
| `VITE_WS_BASE_URL`        | `ws://localhost:8080/ws`               | Websocket base URL          |
| `VITE_KEYCLOAK_ISSUER`    | `http://localhost:8081/realms/quizmos` | Keycloak realm issuer URL   |
| `VITE_KEYCLOAK_CLIENT_ID` | `quizmos-frontend`                     | Keycloak public client ID   |

## License

MIT, with an added restriction on AI/ML training use — see [LICENSE](LICENSE).
