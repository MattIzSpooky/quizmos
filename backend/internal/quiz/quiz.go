package quiz

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mattizspooky/quizmos/backend/internal/core"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/question"
)

func (s *Service) Create(ctx context.Context, createdBy, title, description string, timed bool) (WithCount, error) {
	ctx, span := tracer.Start(ctx, "quiz.Create")
	defer span.End()

	q, err := s.q.CreateQuiz(ctx, db.CreateQuizParams{Title: title, Description: description, CreatedBy: createdBy, Timed: timed})
	if err != nil {
		return WithCount{}, err
	}
	return WithCount{Quiz: q}, nil
}

func (s *Service) List(ctx context.Context) ([]WithCount, error) {
	ctx, span := tracer.Start(ctx, "quiz.List")
	defer span.End()

	quizzes, err := s.q.ListQuizzes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]WithCount, 0, len(quizzes))
	for _, q := range quizzes {
		count, err := s.q.CountQuizQuestions(ctx, q.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, WithCount{Quiz: q, QuestionCount: int(count)})
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (WithCount, error) {
	ctx, span := tracer.Start(ctx, "quiz.Get")
	defer span.End()

	q, err := s.q.GetQuiz(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return WithCount{}, core.ErrNotFound
		}
		return WithCount{}, err
	}
	count, err := s.q.CountQuizQuestions(ctx, q.ID)
	if err != nil {
		return WithCount{}, err
	}
	return WithCount{Quiz: q, QuestionCount: int(count)}, nil
}

func (s *Service) GetDetail(ctx context.Context, id uuid.UUID) (WithCount, []question.WithOptions, error) {
	ctx, span := tracer.Start(ctx, "quiz.GetDetail")
	defer span.End()

	quiz, err := s.Get(ctx, id)
	if err != nil {
		return WithCount{}, nil, err
	}
	questions, err := s.questions.List(ctx, id)
	if err != nil {
		return WithCount{}, nil, err
	}
	return quiz, questions, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, title, description *string, timed *bool) (WithCount, error) {
	ctx, span := tracer.Start(ctx, "quiz.Update")
	defer span.End()

	params := db.UpdateQuizParams{
		ID:          id,
		Title:       core.TextParam(title),
		Description: core.TextParam(description),
		Timed:       core.BoolParam(timed),
	}
	q, err := s.q.UpdateQuiz(ctx, params)
	if err != nil {
		if err == pgx.ErrNoRows {
			return WithCount{}, core.ErrNotFound
		}
		return WithCount{}, err
	}
	count, err := s.q.CountQuizQuestions(ctx, q.ID)
	if err != nil {
		return WithCount{}, err
	}
	return WithCount{Quiz: q, QuestionCount: int(count)}, nil
}

// Delete removes a quiz along with everything created from it: its
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
func (s *Service) Delete(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	ctx, span := tracer.Start(ctx, "quiz.Delete")
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
		return nil, core.ErrNotFound
	}

	for _, q := range questions {
		if !q.MediaKey.Valid {
			continue
		}
		if err := s.store.Delete(ctx, q.MediaKey.String); err != nil {
			slog.ErrorContext(ctx, "quiz.delete_media_failed", "media_key", q.MediaKey.String, "quiz.id", id, "error", err)
		}
	}

	gameIDs := make([]uuid.UUID, len(games))
	for i, g := range games {
		gameIDs[i] = g.ID
	}
	return gameIDs, nil
}
