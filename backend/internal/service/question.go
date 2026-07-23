package service

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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
	// MediaURL is the public URL of the question's attached image/audio,
	// derived from Question.MediaKey via the storage client — empty when
	// there's no media. Computed here (not left to callers) since only
	// the service layer holds the storage dependency needed to build it.
	MediaURL string
}

// questionWithMedia builds a QuestionWithOptions, filling in MediaURL if
// q has media attached. Every place that constructs one should go
// through this rather than a bare struct literal.
func (s *Service) questionWithMedia(q db.Question, options []db.QuestionOption) QuestionWithOptions {
	qwo := QuestionWithOptions{Question: q, Options: options}
	if q.MediaKey.Valid {
		qwo.MediaURL = s.store.PublicURL(q.MediaKey.String)
	}
	return qwo
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

		result = tx.questionWithMedia(question, opts)
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
		out[i] = s.questionWithMedia(q, byQuestion[q.ID])
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
	return s.questionWithMedia(q, options), nil
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

		result = tx.questionWithMedia(q, opts)
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

// Question media (questions.media_key/media_type): the small, fixed set
// of content types accepted for an image or audio fragment, and the
// upload size cap for each. Audio fragments run longer than an image
// takes bytes to encode, hence the higher cap.
const (
	MaxImageMediaBytes int64 = 8 << 20  // 8 MiB
	MaxAudioMediaBytes int64 = 20 << 20 // 20 MiB
)

var mediaContentTypes = map[string]string{
	"image/png":   "image",
	"image/jpeg":  "image",
	"image/webp":  "image",
	"image/gif":   "image",
	"audio/mpeg":  "audio",
	"audio/mp3":   "audio",
	"audio/wav":   "audio",
	"audio/x-wav": "audio",
	"audio/ogg":   "audio",
	"audio/mp4":   "audio",
	"audio/x-m4a": "audio",
	"audio/webm":  "audio",
}

var mediaExtensions = map[string]string{
	"image/png":   ".png",
	"image/jpeg":  ".jpg",
	"image/webp":  ".webp",
	"image/gif":   ".gif",
	"audio/mpeg":  ".mp3",
	"audio/mp3":   ".mp3",
	"audio/wav":   ".wav",
	"audio/x-wav": ".wav",
	"audio/ogg":   ".ogg",
	"audio/mp4":   ".m4a",
	"audio/x-m4a": ".m4a",
	"audio/webm":  ".weba",
}

// MediaLimitBytes classifies contentType as "image" or "audio" question
// media and returns the max upload size that applies to it. ok is false
// if contentType isn't an accepted question-media type at all — callers
// should reject the upload outright rather than read any of it.
func MediaLimitBytes(contentType string) (mediaType string, maxBytes int64, ok bool) {
	mt, known := mediaContentTypes[contentType]
	if !known {
		return "", 0, false
	}
	if mt == "image" {
		return mt, MaxImageMediaBytes, true
	}
	return mt, MaxAudioMediaBytes, true
}

// UploadQuestionMedia stores r (an already-size-checked image or audio
// fragment) as questionID's media, replacing any existing media, and
// returns the updated question. contentType must be one MediaLimitBytes
// accepts; size is the exact byte count of r.
func (s *Service) UploadQuestionMedia(ctx context.Context, quizID, questionID uuid.UUID, contentType string, r io.Reader, size int64) (QuestionWithOptions, error) {
	mediaType, _, ok := MediaLimitBytes(contentType)
	if !ok {
		return QuestionWithOptions{}, ErrValidation
	}

	question, err := s.q.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, QuizID: quizID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return QuestionWithOptions{}, ErrNotFound
		}
		return QuestionWithOptions{}, err
	}

	key := fmt.Sprintf("questions/%s/%s%s", questionID, uuid.NewString(), mediaExtensions[contentType])
	if err := s.store.Put(ctx, key, r, size, contentType); err != nil {
		return QuestionWithOptions{}, err
	}
	// Best-effort: drop the previous object so uploads don't pile up in
	// the bucket. Not fatal if it fails — the DB row (updated next) is
	// the source of truth for what's current, not the bucket's contents.
	if question.MediaKey.Valid {
		_ = s.store.Delete(ctx, question.MediaKey.String)
	}

	updated, err := s.q.SetQuestionMedia(ctx, db.SetQuestionMediaParams{
		ID: questionID, QuizID: quizID,
		MediaKey:  pgtype.Text{String: key, Valid: true},
		MediaType: pgtype.Text{String: mediaType, Valid: true},
	})
	if err != nil {
		return QuestionWithOptions{}, err
	}
	options, err := s.q.ListOptionsByQuestion(ctx, updated.ID)
	if err != nil {
		return QuestionWithOptions{}, err
	}
	return s.questionWithMedia(updated, options), nil
}

// DeleteQuestionMedia removes a question's attached media, if any — a
// no-op (not an error) if it had none.
func (s *Service) DeleteQuestionMedia(ctx context.Context, quizID, questionID uuid.UUID) (QuestionWithOptions, error) {
	question, err := s.q.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, QuizID: quizID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return QuestionWithOptions{}, ErrNotFound
		}
		return QuestionWithOptions{}, err
	}
	if question.MediaKey.Valid {
		_ = s.store.Delete(ctx, question.MediaKey.String)
	}
	updated, err := s.q.ClearQuestionMedia(ctx, db.ClearQuestionMediaParams{ID: questionID, QuizID: quizID})
	if err != nil {
		return QuestionWithOptions{}, err
	}
	options, err := s.q.ListOptionsByQuestion(ctx, updated.ID)
	if err != nil {
		return QuestionWithOptions{}, err
	}
	return s.questionWithMedia(updated, options), nil
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
	return s.ListQuestions(ctx, quizID)
}
