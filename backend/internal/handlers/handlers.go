// Package handlers implements api.StrictServerInterface for the game and
// quiz domains, mapping between service-layer results and generated
// OpenAPI response types, and triggering websocket broadcasts after
// game-lifecycle mutations. Question's REST handlers live in
// internal/question instead (see question.Handler, embedded below) since
// that domain has no websocket involvement and can safely combine its
// service and handler layers in one package.
package handlers

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/game"
	"github.com/mattizspooky/quizmos/backend/internal/question"
	"github.com/mattizspooky/quizmos/backend/internal/quiz"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// Handlers implements api.StrictServerInterface. It holds the game and
// quiz services (persistence + business rules), the websocket hub (to
// broadcast the effects of admin mutations to connected players), and
// embeds question.Handler for the question-related methods.
type Handlers struct {
	*question.Handler

	games   *game.Service
	quizzes *quiz.Service
	hub     *ws.Hub
}

func New(games *game.Service, quizzes *quiz.Service, questions *question.Handler, hub *ws.Hub) *Handlers {
	return &Handlers{Handler: questions, games: games, quizzes: quizzes, hub: hub}
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

// logQuizAction mirrors logGameAction (see its doc comment) for quiz
// mutations. Quiz authoring is admin-only — nothing a player does ever
// mutates a quiz — so unlike logGameAction there's no actor type to vary.
func logQuizAction(ctx context.Context, action string, quizID uuid.UUID, extra ...any) {
	args := append([]any{"action", action, "quiz.id", quizID, "actor.type", actorAdmin, "actor.id", auth.Actor(ctx)}, extra...)
	slog.InfoContext(ctx, "quiz.action", args...)
}
