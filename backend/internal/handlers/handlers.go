// Package handlers implements api.StrictServerInterface for the game and
// quiz domains, mapping between service-layer results and generated
// OpenAPI response types, and triggering websocket broadcasts after
// game-lifecycle mutations. Question's REST handlers live in
// internal/question instead (see question.Handler, embedded below) since
// that domain has no websocket involvement and can safely combine its
// service and handler layers in one package.
package handlers

import (
	"github.com/mattizspooky/quizmos/backend/internal/audit"
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

// gameDomain and quizDomain log audit lines ("game.action"/"quiz.action")
// for mutations in their respective domains — see audit.Domain. Call
// sites live in game.go, player.go, answer.go, and quiz.go.
const (
	gameDomain audit.Domain = "game"
	quizDomain audit.Domain = "quiz"
)
