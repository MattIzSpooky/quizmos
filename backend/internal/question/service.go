// Package question owns question authoring: CRUD, option validation, and
// media (image/audio) upload — both the business logic and the REST
// handlers that expose it. It has no websocket involvement (nothing in
// api/asyncapi.yaml mutates a question directly), so unlike game and quiz
// it can safely combine both layers in one package without risking an
// import cycle with internal/ws.
package question

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/mattizspooky/quizmos/backend/internal/core"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

// tracer is safe to use before telemetry.Setup runs: otel.Tracer returns a
// lazily-delegating wrapper that resolves the real TracerProvider at each
// Start call, not at the time this var is initialized.
var tracer = otel.Tracer("quizmos/question")

type Service struct {
	pool  *pgxpool.Pool
	q     *db.Queries
	store core.MediaStorage
}

func New(pool *pgxpool.Pool, store core.MediaStorage) *Service {
	return &Service{pool: pool, q: db.New(pool), store: store}
}

// withTx runs fn inside a transaction, giving it a *Service bound to the tx.
func (s *Service) withTx(ctx context.Context, fn func(*Service) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txService := &Service{pool: s.pool, q: s.q.WithTx(tx), store: s.store}
	if err := fn(txService); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
