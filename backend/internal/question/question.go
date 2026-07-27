package question

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mattizspooky/quizmos/backend/internal/core"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

// withMedia builds a WithOptions, filling in MediaURL if q has media
// attached. Every place that constructs one should go through this rather
// than a bare struct literal.
func (s *Service) withMedia(q db.Question, options []db.QuestionOption) WithOptions {
	qwo := WithOptions{Question: q, Options: options}
	if q.MediaKey.Valid {
		qwo.MediaURL = s.store.PublicURL(q.MediaKey.String)
	}
	return qwo
}

// validateOptionsForType enforces that multiple_choice questions have 2+
// options and free_text questions have none — free_text answers are
// always graded manually, so a stored "correct" option would be
// meaningless.
func validateOptionsForType(questionType string, options []OptionInput) error {
	switch questionType {
	case TypeMultipleChoice:
		if len(options) < 2 {
			return core.ErrValidation
		}
	case TypeFreeText:
		if len(options) != 0 {
			return core.ErrValidation
		}
	default:
		return core.ErrValidation
	}
	return nil
}

// requireQuiz confirms quizID exists, without needing a dependency on the
// quiz package — this is the same lightweight existence check quiz.Service
// itself uses internally, just duplicated here to keep question a leaf
// package with no cross-domain imports.
func (s *Service) requireQuiz(ctx context.Context, quizID uuid.UUID) error {
	if _, err := s.q.GetQuiz(ctx, quizID); err != nil {
		if err == pgx.ErrNoRows {
			return core.ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) Create(ctx context.Context, quizID uuid.UUID, questionType, prompt string, timeLimitSeconds, points int, options []OptionInput) (WithOptions, error) {
	ctx, span := tracer.Start(ctx, "question.Create")
	defer span.End()

	if err := s.requireQuiz(ctx, quizID); err != nil {
		return WithOptions{}, err
	}
	if err := validateOptionsForType(questionType, options); err != nil {
		return WithOptions{}, err
	}

	var result WithOptions
	err := s.withTx(ctx, func(tx *Service) error {
		position, err := tx.q.NextQuestionPosition(ctx, quizID)
		if err != nil {
			return err
		}
		question, err := tx.q.CreateQuestion(ctx, db.CreateQuestionParams{
			QuizID:           quizID,
			Type:             questionType,
			Prompt:           prompt,
			Position:         position,
			TimeLimitSeconds: int32(timeLimitSeconds),
			Points:           int32(points),
		})
		if err != nil {
			return err
		}

		opts := make([]db.QuestionOption, 0, len(options))
		for i, o := range options {
			created, err := tx.q.CreateQuestionOption(ctx, db.CreateQuestionOptionParams{
				QuestionID: question.ID,
				Text:       o.Text,
				IsCorrect:  o.IsCorrect,
				Position:   int32(i),
			})
			if err != nil {
				return err
			}
			opts = append(opts, created)
		}

		result = tx.withMedia(question, opts)
		return nil
	})
	return result, err
}

func (s *Service) List(ctx context.Context, quizID uuid.UUID) ([]WithOptions, error) {
	ctx, span := tracer.Start(ctx, "question.List")
	defer span.End()

	questions, err := s.q.ListQuestionsByQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}
	if len(questions) == 0 {
		return []WithOptions{}, nil
	}

	ids := make([]uuid.UUID, len(questions))
	for i, q := range questions {
		ids[i] = q.ID
	}
	options, err := s.q.ListOptionsByQuestionIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byQuestion := make(map[uuid.UUID][]db.QuestionOption)
	for _, o := range options {
		byQuestion[o.QuestionID] = append(byQuestion[o.QuestionID], o)
	}

	out := make([]WithOptions, len(questions))
	for i, q := range questions {
		out[i] = s.withMedia(q, byQuestion[q.ID])
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, quizID, questionID uuid.UUID) (WithOptions, error) {
	ctx, span := tracer.Start(ctx, "question.Get")
	defer span.End()

	q, err := s.q.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, QuizID: quizID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return WithOptions{}, core.ErrNotFound
		}
		return WithOptions{}, err
	}
	options, err := s.q.ListOptionsByQuestion(ctx, q.ID)
	if err != nil {
		return WithOptions{}, err
	}
	return s.withMedia(q, options), nil
}

func (s *Service) Update(ctx context.Context, quizID, questionID uuid.UUID, prompt *string, timeLimitSeconds, points *int, options []OptionInput) (WithOptions, error) {
	ctx, span := tracer.Start(ctx, "question.Update")
	defer span.End()

	var result WithOptions
	err := s.withTx(ctx, func(tx *Service) error {
		q, err := tx.q.UpdateQuestion(ctx, db.UpdateQuestionParams{
			ID:               questionID,
			QuizID:           quizID,
			Prompt:           core.TextParam(prompt),
			TimeLimitSeconds: core.Int4Param(timeLimitSeconds),
			Points:           core.Int4Param(points),
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				return core.ErrNotFound
			}
			return err
		}

		opts, err := tx.q.ListOptionsByQuestion(ctx, q.ID)
		if err != nil {
			return err
		}
		if options != nil {
			if err := validateOptionsForType(q.Type, options); err != nil {
				return err
			}
			if err := tx.q.DeleteOptionsByQuestion(ctx, q.ID); err != nil {
				return err
			}
			opts = opts[:0]
			for i, o := range options {
				created, err := tx.q.CreateQuestionOption(ctx, db.CreateQuestionOptionParams{
					QuestionID: q.ID,
					Text:       o.Text,
					IsCorrect:  o.IsCorrect,
					Position:   int32(i),
				})
				if err != nil {
					return err
				}
				opts = append(opts, created)
			}
		}

		result = tx.withMedia(q, opts)
		return nil
	})
	return result, err
}

func (s *Service) Delete(ctx context.Context, quizID, questionID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "question.Delete")
	defer span.End()

	n, err := s.q.DeleteQuestion(ctx, db.DeleteQuestionParams{ID: questionID, QuizID: quizID})
	if err != nil {
		return err
	}
	if n == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (s *Service) Reorder(ctx context.Context, quizID uuid.UUID, questionIDs []uuid.UUID) ([]WithOptions, error) {
	ctx, span := tracer.Start(ctx, "question.Reorder")
	defer span.End()

	existing, err := s.q.ListQuestionsByQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}
	if len(existing) != len(questionIDs) {
		return nil, core.ErrValidation
	}
	existingSet := make(map[uuid.UUID]bool, len(existing))
	for _, q := range existing {
		existingSet[q.ID] = true
	}
	for _, id := range questionIDs {
		if !existingSet[id] {
			return nil, core.ErrValidation
		}
	}

	err = s.withTx(ctx, func(tx *Service) error {
		// Two passes: positions have a UNIQUE(quiz_id, position)
		// constraint, and going straight to the final order can ask to
		// set some question's position to one another still-unmoved
		// question currently holds (e.g. reversing [0,1] to [1,0] tries
		// to put the second question at position 0 while the first one
		// is still sitting there). Parking everything at a negative,
		// guaranteed-unused position first avoids that collision
		// regardless of the permutation.
		for i, id := range questionIDs {
			if err := tx.q.SetQuestionPosition(ctx, db.SetQuestionPositionParams{
				ID: id, QuizID: quizID, Position: int32(-(i + 1)),
			}); err != nil {
				return err
			}
		}
		for i, id := range questionIDs {
			if err := tx.q.SetQuestionPosition(ctx, db.SetQuestionPositionParams{
				ID: id, QuizID: quizID, Position: int32(i),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.List(ctx, quizID)
}
