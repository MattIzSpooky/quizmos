package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

type QuizWithCount struct {
	db.Quiz
	QuestionCount int
}

func (s *Service) CreateQuiz(ctx context.Context, createdBy, title, description string, timed bool) (QuizWithCount, error) {
	ctx, span := tracer.Start(ctx, "service.CreateQuiz")
	defer span.End()

	q, err := s.q.CreateQuiz(ctx, db.CreateQuizParams{Title: title, Description: description, CreatedBy: createdBy, Timed: timed})
	if err != nil {
		return QuizWithCount{}, err
	}
	return QuizWithCount{Quiz: q}, nil
}

func (s *Service) ListQuizzes(ctx context.Context) ([]QuizWithCount, error) {
	ctx, span := tracer.Start(ctx, "service.ListQuizzes")
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "service.GetQuiz")
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "service.GetQuizDetail")
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "service.UpdateQuiz")
	defer span.End()

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

// DeleteQuiz removes a quiz along with everything created from it: its
// questions (and their options), and — now that games.quiz_id cascades
// rather than restricting — every game ever played from it, including
// their players and answers. The one thing a SQL cascade can't reach is
// question media sitting in MinIO, so that's cleaned up here explicitly
// after the row is gone; a failed object delete is logged rather than
// failing the request, since the quiz itself is already gone by then and
// an orphaned blob is a storage-cleanup concern, not a correctness one.
//
// The returned game IDs are for the caller (the REST handler, which
// alone holds the websocket hub) to close out any live connections still
// pointing at a game that no longer exists.
func (s *Service) DeleteQuiz(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	ctx, span := tracer.Start(ctx, "service.DeleteQuiz")
	defer span.End()

	questions, err := s.q.ListQuestionsByQuiz(ctx, id)
	if err != nil {
		return nil, err
	}
	games, err := s.q.ListGamesByQuiz(ctx, id)
	if err != nil {
		return nil, err
	}

	n, err := s.q.DeleteQuiz(ctx, id)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}

	for _, q := range questions {
		if !q.MediaKey.Valid {
			continue
		}
		if err := s.store.Delete(ctx, q.MediaKey.String); err != nil {
			slog.ErrorContext(ctx, "service.delete_media_failed", "media_key", q.MediaKey.String, "quiz.id", id, "error", err)
		}
	}

	gameIDs := make([]uuid.UUID, len(games))
	for i, g := range games {
		gameIDs[i] = g.ID
	}
	return gameIDs, nil
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
