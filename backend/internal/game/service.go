// Package game owns the game lifecycle: creating and running a game from a
// quiz, players joining and being kicked, answer submission and grading,
// and the leaderboard. These are modeled as one package rather than split
// further because a player, an answer, and a leaderboard entry only ever
// exist in the context of some game — the same aggregate boundary the
// original monolithic service/game.go file reflected, just now spread
// across several files by concern (lifecycle.go, player.go, answer.go)
// instead of one 900-line file.
//
// Only the service layer lives here, not the REST handlers (see
// internal/handlers) — internal/ws.Hub calls several of these methods
// directly (ReviewQuestion, Leaderboard, SubmitAnswer, and others) without
// going through the handler layer, and the game handlers in turn call
// ws.Hub.Broadcast after mutations. Combining both layers in one package
// would make game import ws and ws import game at the same time — an
// import cycle — so the two layers stay in separate packages.
package game

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/question"
)

// tracer is safe to use before telemetry.Setup runs: otel.Tracer returns a
// lazily-delegating wrapper that resolves the real TracerProvider at each
// Start call, not at the time this var is initialized.
var tracer = otel.Tracer("quizmos/game")

type Service struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	questions *question.Service
}

func New(pool *pgxpool.Pool, questions *question.Service) *Service {
	return &Service{pool: pool, q: db.New(pool), questions: questions}
}

// withTx runs fn inside a transaction, giving it a *Service bound to the tx.
func (s *Service) withTx(ctx context.Context, fn func(*Service) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txService := &Service{pool: s.pool, q: s.q.WithTx(tx), questions: s.questions}
	if err := fn(txService); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
