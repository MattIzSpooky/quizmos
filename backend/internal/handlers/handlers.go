package handlers

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/service"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// Handlers implements api.StrictServerInterface. It holds the domain
// service (persistence + business rules) and the websocket hub (to
// broadcast the effects of admin mutations to connected players).
type Handlers struct {
	svc      *service.Service
	hub      *ws.Hub
	keycloak *auth.Keycloak
}

func New(svc *service.Service, hub *ws.Hub, keycloak *auth.Keycloak) *Handlers {
	return &Handlers{svc: svc, hub: hub, keycloak: keycloak}
}

// adminSubject returns the Keycloak subject of the authenticated caller,
// used only for the created_by audit column.
func adminSubject(ctx context.Context) string {
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		return claims.Subject
	}
	return ""
}

// adminActor returns a human-readable identifier for the authenticated
// admin (their Keycloak username) for use in logGameAction calls — unlike
// adminSubject, which is the DB's stable but opaque created_by value, this
// is what actually tells one admin apart from another when reading logs.
func adminActor(ctx context.Context) string {
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		return claims.DisplayName()
	}
	return ""
}

// actorType values for logGameAction's actorType parameter.
const (
	actorAdmin  = "admin"
	actorPlayer = "player"
)

// logGameAction records one line of audit-style logging for a game
// mutation — who (an admin's Keycloak subject, or a player's client ID)
// did what to which game — in a shape consistent across every call site,
// so they're all findable with the same Loki query regardless of which
// specific action it was. Call it only after a mutation actually
// succeeds; failed attempts are already visible via the request/message's
// own log line and (for websocket messages) span status.
func logGameAction(ctx context.Context, action string, gameID uuid.UUID, actorType, actorID string, extra ...any) {
	args := append([]any{"action", action, "game.id", gameID, "actor.type", actorType, "actor.id", actorID}, extra...)
	slog.InfoContext(ctx, "game.action", args...)
}
