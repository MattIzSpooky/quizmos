# syntax=docker/dockerfile:1

# --- build ------------------------------------------------------------
FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO is off: the pgx postgres driver is pure Go, and a static binary is
# all the distroless runtime stage below needs.
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

# --- runtime ------------------------------------------------------------
# distroless/static: no shell, no package manager, just the binary and the
# CA certs / tzdata it needs — nothing here can regenerate code or run
# migrations, both of which happen elsewhere (see the root docker-compose.yml).
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/server ./server

EXPOSE 8080
ENTRYPOINT ["/app/server"]
