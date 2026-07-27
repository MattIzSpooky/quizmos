package question

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mattizspooky/quizmos/backend/internal/core"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

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

// UploadMedia stores r (an already-size-checked image or audio fragment)
// as questionID's media, replacing any existing media, and returns the
// updated question. contentType must be one MediaLimitBytes accepts; size
// is the exact byte count of r.
func (s *Service) UploadMedia(ctx context.Context, quizID, questionID uuid.UUID, contentType string, r io.Reader, size int64) (WithOptions, error) {
	ctx, span := tracer.Start(ctx, "question.UploadMedia")
	defer span.End()

	mediaType, _, ok := MediaLimitBytes(contentType)
	if !ok {
		return WithOptions{}, core.ErrValidation
	}

	question, err := s.q.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, QuizID: quizID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return WithOptions{}, core.ErrNotFound
		}
		return WithOptions{}, err
	}

	key := fmt.Sprintf("questions/%s/%s%s", questionID, uuid.NewString(), mediaExtensions[contentType])
	if err := s.store.Put(ctx, key, r, size, contentType); err != nil {
		return WithOptions{}, err
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
		return WithOptions{}, err
	}
	options, err := s.q.ListOptionsByQuestion(ctx, updated.ID)
	if err != nil {
		return WithOptions{}, err
	}
	return s.withMedia(updated, options), nil
}

// DeleteMedia removes a question's attached media, if any — a no-op (not
// an error) if it had none.
func (s *Service) DeleteMedia(ctx context.Context, quizID, questionID uuid.UUID) (WithOptions, error) {
	ctx, span := tracer.Start(ctx, "question.DeleteMedia")
	defer span.End()

	question, err := s.q.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, QuizID: quizID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return WithOptions{}, core.ErrNotFound
		}
		return WithOptions{}, err
	}
	if question.MediaKey.Valid {
		_ = s.store.Delete(ctx, question.MediaKey.String)
	}
	updated, err := s.q.ClearQuestionMedia(ctx, db.ClearQuestionMediaParams{ID: questionID, QuizID: quizID})
	if err != nil {
		return WithOptions{}, err
	}
	options, err := s.q.ListOptionsByQuestion(ctx, updated.ID)
	if err != nil {
		return WithOptions{}, err
	}
	return s.withMedia(updated, options), nil
}
