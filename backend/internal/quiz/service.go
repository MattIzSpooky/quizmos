// Package quiz owns quiz authoring: create/list/get/update/delete, plus
// the question count and question list rolled into a quiz's detail view.
//
// Unlike question, this package holds only the service layer, not the
// REST handlers (see internal/handlers) — internal/ws.Hub calls
// Service.GetQuiz directly (to read a game's quiz.Timed flag) without
// going through the handler layer, and the quiz handler in turn calls
// ws.Hub.CloseRoom after deleting a quiz. Combining both layers in one
// package here would make quiz import ws and ws import quiz at the same
// time — an import cycle — so, same as game, the two layers stay in
// separate packages.
package quiz

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/mattizspooky/quizmos/backend/internal/core"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/question"
)

// tracer is safe to use before telemetry.Setup runs: otel.Tracer returns a
// lazily-delegating wrapper that resolves the real TracerProvider at each
// Start call, not at the time this var is initialized.
var tracer = otel.Tracer("quizmos/quiz")

type Service struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	store     core.MediaStorage
	questions *question.Service
}

func New(pool *pgxpool.Pool, store core.MediaStorage, questions *question.Service) *Service {
	return &Service{pool: pool, q: db.New(pool), store: store, questions: questions}
}
