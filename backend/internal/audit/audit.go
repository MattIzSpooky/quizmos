// Package audit provides one small, uniform helper for logging mutations
// across domains ("game.action", "quiz.action", "question.action", ...),
// so every domain's audit trail is shaped the same way and findable with
// the same Loki query regardless of which domain or specific action
// fired. It has no dependency on any domain package (only on auth, for
// the LogAdmin convenience), so internal/handlers, internal/question, and
// internal/ws — which can't import handlers, see its package doc comment
// — can all share one implementation instead of hand-copying it.
package audit

import (
	"context"
	"log/slog"

	"github.com/mattizspooky/quizmos/backend/internal/auth"
)

// Actor type values for Domain.Log's actorType parameter.
const (
	Admin  = "admin"
	Player = "player"
)

// Domain scopes an audit logger to one resource type — every record it
// logs uses "<domain>.action" as the message and "<domain>.id" as the
// resource-ID key.
type Domain string

// Log records one line of audit-style logging for a mutation to a
// resource of this domain — who did what to which resource. Call it only
// after a mutation actually succeeds; failed attempts are already
// visible via the request/message's own log line (and, for websocket
// messages, span status).
func (d Domain) Log(ctx context.Context, action string, resourceID any, actorType, actorID string, extra ...any) {
	args := append([]any{"action", action, string(d) + ".id", resourceID, "actor.type", actorType, "actor.id", actorID}, extra...)
	slog.InfoContext(ctx, string(d)+".action", args...)
}

// LogAdmin is Log with actorType fixed to Admin and actorID read from
// ctx's authenticated claims — for domains (quiz, question) that are
// always admin-mutated, so call sites don't need to plumb auth.Actor
// themselves.
func (d Domain) LogAdmin(ctx context.Context, action string, resourceID any, extra ...any) {
	d.Log(ctx, action, resourceID, Admin, auth.Actor(ctx), extra...)
}
