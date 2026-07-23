// Package service holds Quizmos' domain logic: quiz/question authoring,
// game lifecycle, joining, and scoring. It is used by both the REST
// handlers and the websocket hub, and knows nothing about HTTP or
// websockets itself.
package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

type Service struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: db.New(pool)}
}

// withTx runs fn inside a transaction, giving it a *Service bound to the tx.
func (s *Service) withTx(ctx context.Context, fn func(*Service) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txService := &Service{pool: s.pool, q: s.q.WithTx(tx)}
	if err := fn(txService); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
