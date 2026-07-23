package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

// pgForeignKeyViolation is Postgres's SQLSTATE for a foreign-key
// constraint violation (23503).
const pgForeignKeyViolation = "23503"

type QuizWithCount struct {
	db.Quiz
	QuestionCount int
}

func (s *Service) CreateQuiz(ctx context.Context, createdBy, title, description string, timed bool) (QuizWithCount, error) {
	q, err := s.q.CreateQuiz(ctx, db.CreateQuizParams{Title: title, Description: description, CreatedBy: createdBy, Timed: timed})
	if err != nil {
		return QuizWithCount{}, err
	}
	return QuizWithCount{Quiz: q}, nil
}

func (s *Service) ListQuizzes(ctx context.Context) ([]QuizWithCount, error) {
	quizzes, err := s.q.ListQuizzes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]QuizWithCount, 0, len(quizzes))
	for _, q := range quizzes {
		count, err := s.q.CountQuizQuestions(ctx, q.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, QuizWithCount{Quiz: q, QuestionCount: int(count)})
	}
	return out, nil
}

func (s *Service) GetQuiz(ctx context.Context, id uuid.UUID) (QuizWithCount, error) {
	q, err := s.q.GetQuiz(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return QuizWithCount{}, ErrNotFound
		}
		return QuizWithCount{}, err
	}
	count, err := s.q.CountQuizQuestions(ctx, q.ID)
	if err != nil {
		return QuizWithCount{}, err
	}
	return QuizWithCount{Quiz: q, QuestionCount: int(count)}, nil
}

func (s *Service) GetQuizDetail(ctx context.Context, id uuid.UUID) (QuizWithCount, []QuestionWithOptions, error) {
	quiz, err := s.GetQuiz(ctx, id)
	if err != nil {
		return QuizWithCount{}, nil, err
	}
	questions, err := s.ListQuestions(ctx, id)
	if err != nil {
		return QuizWithCount{}, nil, err
	}
	return quiz, questions, nil
}

func (s *Service) UpdateQuiz(ctx context.Context, id uuid.UUID, title, description *string, timed *bool) (QuizWithCount, error) {
	params := db.UpdateQuizParams{
		ID:          id,
		Title:       textParam(title),
		Description: textParam(description),
		Timed:       boolParam(timed),
	}
	q, err := s.q.UpdateQuiz(ctx, params)
	if err != nil {
		if err == pgx.ErrNoRows {
			return QuizWithCount{}, ErrNotFound
		}
		return QuizWithCount{}, err
	}
	count, err := s.q.CountQuizQuestions(ctx, q.ID)
	if err != nil {
		return QuizWithCount{}, err
	}
	return QuizWithCount{Quiz: q, QuestionCount: int(count)}, nil
}

// DeleteQuiz removes a quiz and its questions (cascading). A quiz that
// still has any game created from it — lobby, in-progress, or ended —
// can't be deleted: games.quiz_id is ON DELETE RESTRICT, since a game is
// a historical record of a play session and shouldn't silently lose the
// quiz it was played from.
func (s *Service) DeleteQuiz(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteQuiz(ctx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return ErrConflict
		}
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) requireQuiz(ctx context.Context, id uuid.UUID) error {
	if _, err := s.q.GetQuiz(ctx, id); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("quiz %s: %w", id, ErrNotFound)
		}
		return err
	}
	return nil
}
