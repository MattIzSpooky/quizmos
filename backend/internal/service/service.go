// Package service holds Quizmos' domain logic: quiz/question authoring,
// game lifecycle, joining, and scoring. It is used by both the REST
// handlers and the websocket hub, and knows nothing about HTTP or
// websockets itself.
package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"

	"github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

// tracer is safe to use before telemetry.Setup runs: otel.Tracer returns a
// lazily-delegating wrapper that resolves the real TracerProvider at each
// Start call, not at the time this var is initialized. Every exported
// Service method starts a span with it, named "service.<Method>" — mostly
// to group and time the DB-query spans (see internal/telemetry, otelpgx)
// each one triggers, which otherwise appear as an unlabeled, flat pile of
// siblings under the enclosing HTTP request or websocket message span.
var tracer = otel.Tracer("quizmos/service")

type Service struct {
	pool  *pgxpool.Pool
	q     *db.Queries
	store MediaStorage
}

func New(pool *pgxpool.Pool, store MediaStorage) *Service {
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
