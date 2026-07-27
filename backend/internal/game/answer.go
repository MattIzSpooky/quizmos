package game

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/mattizspooky/quizmos/backend/internal/core"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/question"
)

// MaxFreeTextAnswerLength is the longest answer text SubmitAnswer accepts
// for a free_text question, enforced here rather than at the schema level
// so a too-long submission gets the same ErrValidation handling (and
// resulting websocket error message) as any other invalid answer.
const MaxFreeTextAnswerLength = 500

// SubmitAnswer validates and records a player's answer to the game's
// current question. It is called from the websocket handler. Exactly one
// of optionID or text must be non-nil, matching the current question's
// type (multiple_choice or free_text respectively) — anything else is
// ErrValidation.
func (s *Service) SubmitAnswer(ctx context.Context, gameID uuid.UUID, clientID, questionID uuid.UUID, optionID *uuid.UUID, text *string) (AnswerResult, error) {
	ctx, span := tracer.Start(ctx, "game.SubmitAnswer")
	defer span.End()

	g, err := s.q.GetGame(ctx, gameID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return AnswerResult{}, core.ErrNotFound
		}
		return AnswerResult{}, err
	}
	if g.Status != "in_progress" || !g.CurrentQuestionIndex.Valid {
		return AnswerResult{}, core.ErrConflict
	}

	currentQuestion, _, err := s.QuestionAtIndex(ctx, g.QuizID, int(g.CurrentQuestionIndex.Int32))
	if err != nil {
		return AnswerResult{}, err
	}
	if currentQuestion.ID != questionID {
		return AnswerResult{}, core.ErrConflict
	}

	player, err := s.q.GetPlayer(ctx, db.GetPlayerParams{GameID: gameID, ClientID: clientID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return AnswerResult{}, core.ErrNotFound
		}
		return AnswerResult{}, err
	}

	if _, err := s.q.GetAnswer(ctx, db.GetAnswerParams{QuestionID: questionID, PlayerID: player.ID}); err == nil {
		return AnswerResult{}, core.ErrConflict
	} else if err != pgx.ErrNoRows {
		return AnswerResult{}, err
	}

	params := db.CreateAnswerParams{GameID: gameID, QuestionID: questionID, PlayerID: player.ID}
	var result AnswerResult

	switch currentQuestion.Type {
	case question.TypeMultipleChoice:
		if optionID == nil {
			return AnswerResult{}, core.ErrValidation
		}
		option, err := s.q.GetOption(ctx, *optionID)
		if err != nil {
			if err == pgx.ErrNoRows {
				return AnswerResult{}, core.ErrValidation
			}
			return AnswerResult{}, err
		}
		if option.QuestionID != questionID {
			return AnswerResult{}, core.ErrValidation
		}

		points := 0
		if option.IsCorrect {
			points = int(currentQuestion.Points)
		}
		params.SelectedOptionID = pgtype.UUID{Bytes: *optionID, Valid: true}
		params.IsCorrect = pgtype.Bool{Bool: option.IsCorrect, Valid: true}
		params.PointsAwarded = int32(points)
		result = AnswerResult{Correct: option.IsCorrect, PointsAwarded: points, TotalScore: int(player.Score) + points}

	case question.TypeFreeText:
		if text == nil {
			return AnswerResult{}, core.ErrValidation
		}
		trimmed := strings.TrimSpace(*text)
		if trimmed == "" || len([]rune(trimmed)) > MaxFreeTextAnswerLength {
			return AnswerResult{}, core.ErrValidation
		}
		// SelectedOptionID/IsCorrect are left at their zero value (SQL
		// NULL) — grading happens later, by hand, via GradeAnswer.
		params.AnswerText = pgtype.Text{String: trimmed, Valid: true}
		result = AnswerResult{Pending: true, TotalScore: int(player.Score)}

	default:
		return AnswerResult{}, core.ErrValidation
	}

	err = s.withTx(ctx, func(tx *Service) error {
		if _, err := tx.q.CreateAnswer(ctx, params); err != nil {
			return err
		}
		if params.PointsAwarded != 0 {
			if err := tx.q.AddPlayerScore(ctx, db.AddPlayerScoreParams{ID: player.ID, Score: params.PointsAwarded}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return AnswerResult{}, err
	}

	outcome := "incorrect"
	switch {
	case result.Pending:
		outcome = "pending"
	case result.Correct:
		outcome = "correct"
	}
	answersSubmitted.Add(ctx, 1, metric.WithAttributes(
		attribute.String("question_type", currentQuestion.Type),
		attribute.String("outcome", outcome),
	))
	return result, nil
}

// GradeAnswer manually grades a free-text answer: full points for the
// question if correct, none otherwise. It can be called again for the
// same answer to correct a grading mistake — the player's score is
// adjusted by the difference from whatever was previously awarded, not
// simply added to again.
func (s *Service) GradeAnswer(ctx context.Context, gameID, answerID uuid.UUID, correct bool) (GradedAnswer, error) {
	ctx, span := tracer.Start(ctx, "game.GradeAnswer")
	defer span.End()

	answer, err := s.q.GetAnswerByID(ctx, answerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return GradedAnswer{}, core.ErrNotFound
		}
		return GradedAnswer{}, err
	}
	if answer.GameID != gameID || !answer.AnswerText.Valid {
		return GradedAnswer{}, core.ErrNotFound
	}

	q, err := s.q.GetQuestionByID(ctx, answer.QuestionID)
	if err != nil {
		return GradedAnswer{}, err
	}
	newPoints := int32(0)
	if correct {
		newPoints = q.Points
	}
	delta := newPoints - answer.PointsAwarded

	var player db.Player
	err = s.withTx(ctx, func(tx *Service) error {
		if _, err := tx.q.GradeAnswer(ctx, db.GradeAnswerParams{
			ID:            answerID,
			IsCorrect:     pgtype.Bool{Bool: correct, Valid: true},
			PointsAwarded: newPoints,
		}); err != nil {
			return err
		}
		if delta != 0 {
			if err := tx.q.AddPlayerScore(ctx, db.AddPlayerScoreParams{ID: answer.PlayerID, Score: delta}); err != nil {
				return err
			}
		}
		p, err := tx.q.GetPlayerByID(ctx, answer.PlayerID)
		if err != nil {
			return err
		}
		player = p
		return nil
	})
	if err != nil {
		return GradedAnswer{}, err
	}

	outcome := "incorrect"
	if correct {
		outcome = "correct"
	}
	answersGraded.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))

	return GradedAnswer{
		ClientID:      player.ClientID,
		QuestionID:    answer.QuestionID,
		QuestionIndex: int(q.Position),
		Correct:       correct,
		PointsAwarded: int(newPoints),
		TotalScore:    int(player.Score),
	}, nil
}

// ListFreeTextAnswers returns every free-text answer submitted so far for
// one question in this game, oldest first, for the admin to grade.
func (s *Service) ListFreeTextAnswers(ctx context.Context, gameID, questionID uuid.UUID) ([]FreeTextAnswer, error) {
	ctx, span := tracer.Start(ctx, "game.ListFreeTextAnswers")
	defer span.End()

	rows, err := s.q.ListFreeTextAnswersForQuestion(ctx, db.ListFreeTextAnswersForQuestionParams{
		GameID: gameID, QuestionID: questionID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]FreeTextAnswer, 0, len(rows))
	for _, r := range rows {
		if !r.AnswerText.Valid {
			continue // a multiple_choice answer, if the wrong questionID was passed
		}
		out = append(out, FreeTextAnswer{
			ID:            r.ID,
			ClientID:      r.ClientID,
			Nickname:      r.Nickname,
			Text:          r.AnswerText.String,
			Graded:        r.IsCorrect.Valid,
			Correct:       r.IsCorrect.Valid && r.IsCorrect.Bool,
			PointsAwarded: int(r.PointsAwarded),
		})
	}
	return out, nil
}

// GetPlayerAnswerStatus looks up clientID's own answer (if any) to
// questionID in gameID. A player who hasn't answered, or who isn't found
// (shouldn't normally happen for a connected client), gets the zero value
// (Answered: false) rather than an error — the caller treats both the
// same way.
func (s *Service) GetPlayerAnswerStatus(ctx context.Context, gameID, clientID, questionID uuid.UUID) (PlayerAnswerStatus, error) {
	ctx, span := tracer.Start(ctx, "game.GetPlayerAnswerStatus")
	defer span.End()

	player, err := s.q.GetPlayer(ctx, db.GetPlayerParams{GameID: gameID, ClientID: clientID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return PlayerAnswerStatus{}, nil
		}
		return PlayerAnswerStatus{}, err
	}
	answer, err := s.q.GetAnswer(ctx, db.GetAnswerParams{QuestionID: questionID, PlayerID: player.ID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return PlayerAnswerStatus{}, nil
		}
		return PlayerAnswerStatus{}, err
	}

	status := PlayerAnswerStatus{Answered: true, PointsAwarded: int(answer.PointsAwarded)}
	if answer.SelectedOptionID.Valid {
		id := uuid.UUID(answer.SelectedOptionID.Bytes)
		status.SelectedOptionID = &id
	}
	if answer.AnswerText.Valid {
		status.Text = &answer.AnswerText.String
	}
	if answer.IsCorrect.Valid {
		status.Correct = answer.IsCorrect.Bool
	} else {
		status.Pending = true
	}
	return status, nil
}
