package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

// Question type discriminators (questions.type). multiple_choice is scored
// automatically against a stored correct option; free_text has no correct
// answer on file at all — the admin grades each submission by hand (see
// Service.GradeAnswer).
const (
	QuestionTypeMultipleChoice = "multiple_choice"
	QuestionTypeFreeText       = "free_text"
)

type QuestionOptionInput struct {
	Text      string
	IsCorrect bool
}

type QuestionWithOptions struct {
	db.Question
	Options []db.QuestionOption
}

// validateOptionsForType enforces that multiple_choice questions have 2+
// options and free_text questions have none — free_text answers are
// always graded manually, so a stored "correct" option would be
// meaningless.
func validateOptionsForType(questionType string, options []QuestionOptionInput) error {
	switch questionType {
	case QuestionTypeMultipleChoice:
		if len(options) < 2 {
			return ErrValidation
		}
	case QuestionTypeFreeText:
		if len(options) != 0 {
			return ErrValidation
		}
	default:
		return ErrValidation
	}
	return nil
}

func (s *Service) CreateQuestion(ctx context.Context, quizID uuid.UUID, questionType, prompt string, timeLimitSeconds, points int, options []QuestionOptionInput) (QuestionWithOptions, error) {
	if err := s.requireQuiz(ctx, quizID); err != nil {
		return QuestionWithOptions{}, err
	}
	if err := validateOptionsForType(questionType, options); err != nil {
		return QuestionWithOptions{}, err
	}

	var result QuestionWithOptions
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

		result = QuestionWithOptions{Question: question, Options: opts}
		return nil
	})
	return result, err
}

func (s *Service) ListQuestions(ctx context.Context, quizID uuid.UUID) ([]QuestionWithOptions, error) {
	questions, err := s.q.ListQuestionsByQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}
	if len(questions) == 0 {
		return []QuestionWithOptions{}, nil
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

	out := make([]QuestionWithOptions, len(questions))
	for i, q := range questions {
		out[i] = QuestionWithOptions{Question: q, Options: byQuestion[q.ID]}
	}
	return out, nil
}

func (s *Service) GetQuestion(ctx context.Context, quizID, questionID uuid.UUID) (QuestionWithOptions, error) {
	q, err := s.q.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, QuizID: quizID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return QuestionWithOptions{}, ErrNotFound
		}
		return QuestionWithOptions{}, err
	}
	options, err := s.q.ListOptionsByQuestion(ctx, q.ID)
	if err != nil {
		return QuestionWithOptions{}, err
	}
	return QuestionWithOptions{Question: q, Options: options}, nil
}

func (s *Service) UpdateQuestion(ctx context.Context, quizID, questionID uuid.UUID, prompt *string, timeLimitSeconds, points *int, options []QuestionOptionInput) (QuestionWithOptions, error) {
	var result QuestionWithOptions
	err := s.withTx(ctx, func(tx *Service) error {
		q, err := tx.q.UpdateQuestion(ctx, db.UpdateQuestionParams{
			ID:               questionID,
			QuizID:           quizID,
			Prompt:           textParam(prompt),
			TimeLimitSeconds: int4Param(timeLimitSeconds),
			Points:           int4Param(points),
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrNotFound
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

		result = QuestionWithOptions{Question: q, Options: opts}
		return nil
	})
	return result, err
}

func (s *Service) DeleteQuestion(ctx context.Context, quizID, questionID uuid.UUID) error {
	n, err := s.q.DeleteQuestion(ctx, db.DeleteQuestionParams{ID: questionID, QuizID: quizID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ReorderQuestions(ctx context.Context, quizID uuid.UUID, questionIDs []uuid.UUID) ([]QuestionWithOptions, error) {
	existing, err := s.q.ListQuestionsByQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}
	if len(existing) != len(questionIDs) {
		return nil, ErrValidation
	}
	existingSet := make(map[uuid.UUID]bool, len(existing))
	for _, q := range existing {
		existingSet[q.ID] = true
	}
	for _, id := range questionIDs {
		if !existingSet[id] {
			return nil, ErrValidation
		}
	}

	err = s.withTx(ctx, func(tx *Service) error {
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
	return s.ListQuestions(ctx, quizID)
}
