package game

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/mattizspooky/quizmos/backend/internal/core"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/question"
)

// summaryFromRow builds a Summary from the game_summaries view shape
// (games joined with its quiz's title/timed, plus live player/question
// counts — see migration 000007). StartGame/SetCurrentQuestionIndex/EndGame
// each select this same column set via a CTE so a mutation's result can be
// converted here too, without a separate round trip to re-fetch the
// summary afterward — hence the explicit conversion at each call site
// instead of a single parameter type: db.StartGameRow, db.
// SetCurrentQuestionIndexRow, and db.EndGameRow are structurally identical
// to db.GameSummary (same fields, same order), so converting one to
// another is a plain Go type conversion.
func summaryFromRow(g db.GameSummary) Summary {
	return Summary{
		Game: db.Game{
			ID: g.ID, QuizID: g.QuizID, Code: g.Code, Status: g.Status,
			CurrentQuestionIndex: g.CurrentQuestionIndex, CreatedBy: g.CreatedBy,
			CreatedAt: g.CreatedAt, StartedAt: g.StartedAt, EndedAt: g.EndedAt,
		},
		QuizTitle:      g.QuizTitle,
		QuizTimed:      g.QuizTimed,
		PlayerCount:    int(g.PlayerCount),
		TotalQuestions: int(g.TotalQuestions),
	}
}

func (s *Service) Create(ctx context.Context, createdBy string, quizID uuid.UUID) (Summary, error) {
	ctx, span := tracer.Start(ctx, "game.Create")
	defer span.End()

	quiz, err := s.q.GetQuizSummary(ctx, quizID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Summary{}, core.ErrNotFound
		}
		return Summary{}, err
	}
	if quiz.QuestionCount == 0 {
		return Summary{}, core.ErrValidation
	}

	code, err := s.generateUniqueGameCode(ctx)
	if err != nil {
		return Summary{}, err
	}
	g, err := s.q.CreateGame(ctx, db.CreateGameParams{QuizID: quizID, Code: code, CreatedBy: createdBy})
	if err != nil {
		return Summary{}, err
	}
	// A freshly created game has no players yet, and the quiz info was
	// just read above — no need for a further round trip to summarize it.
	summary := Summary{
		Game:           g,
		QuizTitle:      quiz.Title,
		QuizTimed:      quiz.Timed,
		PlayerCount:    0,
		TotalQuestions: int(quiz.QuestionCount),
	}
	gamesCreated.Add(ctx, 1)
	return summary, nil
}

func (s *Service) List(ctx context.Context, status *string) ([]Summary, error) {
	ctx, span := tracer.Start(ctx, "game.List")
	defer span.End()

	rows, err := s.q.ListGames(ctx, core.TextParam(status))
	if err != nil {
		return nil, err
	}
	out := make([]Summary, len(rows))
	for i, r := range rows {
		out[i] = summaryFromRow(r)
	}
	return out, nil
}

func (s *Service) GetDetail(ctx context.Context, id uuid.UUID) (Detail, error) {
	ctx, span := tracer.Start(ctx, "game.GetDetail")
	defer span.End()

	g, err := s.q.GetGameSummary(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Detail{}, core.ErrNotFound
		}
		return Detail{}, err
	}
	players, err := s.q.ListPlayersByGame(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	return Detail{Summary: summaryFromRow(g), Players: players}, nil
}

// QuestionAtIndex returns the question at the given 0-based position in the
// game's quiz, along with the quiz's total question count.
func (s *Service) QuestionAtIndex(ctx context.Context, quizID uuid.UUID, index int) (question.WithOptions, int, error) {
	ctx, span := tracer.Start(ctx, "game.QuestionAtIndex")
	defer span.End()

	questions, err := s.questions.List(ctx, quizID)
	if err != nil {
		return question.WithOptions{}, 0, err
	}
	if index < 0 || index >= len(questions) {
		return question.WithOptions{}, len(questions), core.ErrNotFound
	}
	return questions[index], len(questions), nil
}

func (s *Service) Start(ctx context.Context, id uuid.UUID) (Summary, error) {
	ctx, span := tracer.Start(ctx, "game.Start")
	defer span.End()

	g, err := s.q.StartGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Summary{}, core.ErrConflict
		}
		return Summary{}, err
	}
	gamesStarted.Add(ctx, 1)
	return summaryFromRow(db.GameSummary(g)), nil
}

func (s *Service) Advance(ctx context.Context, id uuid.UUID) (AdvanceResult, error) {
	ctx, span := tracer.Start(ctx, "game.Advance")
	defer span.End()

	g, err := s.q.GetGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return AdvanceResult{}, core.ErrNotFound
		}
		return AdvanceResult{}, err
	}
	if g.Status != "in_progress" || !g.CurrentQuestionIndex.Valid {
		return AdvanceResult{}, core.ErrConflict
	}
	prevIndex := int(g.CurrentQuestionIndex.Int32)

	prevQuestion, total, err := s.QuestionAtIndex(ctx, g.QuizID, prevIndex)
	if err != nil {
		return AdvanceResult{}, err
	}
	counts, err := s.answerCounts(ctx, prevQuestion.ID)
	if err != nil {
		return AdvanceResult{}, err
	}

	result := AdvanceResult{PrevIndex: prevIndex, PrevQuestion: prevQuestion, AnswerCounts: counts}

	nextIndex := prevIndex + 1
	if nextIndex >= total {
		ended, err := s.q.EndGame(ctx, id)
		if err != nil {
			return AdvanceResult{}, err
		}
		leaderboard, err := s.Leaderboard(ctx, id)
		if err != nil {
			return AdvanceResult{}, err
		}
		result.Game = summaryFromRow(db.GameSummary(ended))
		result.Ended = true
		result.FinalLeaderboard = leaderboard
		gamesEnded.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "completed")))
		return result, nil
	}

	updated, err := s.q.SetCurrentQuestionIndex(ctx, db.SetCurrentQuestionIndexParams{
		ID: id, CurrentQuestionIndex: pgtype.Int4{Int32: int32(nextIndex), Valid: true},
	})
	if err != nil {
		return AdvanceResult{}, err
	}
	nextQuestion, _, err := s.QuestionAtIndex(ctx, g.QuizID, nextIndex)
	if err != nil {
		return AdvanceResult{}, err
	}
	result.Game = summaryFromRow(db.GameSummary(updated))
	result.NextQuestion = &nextQuestion
	return result, nil
}

// ReviewQuestion is a pure broadcast: it never mutates the game (not the
// current-question pointer, not the answers table), so "next question"
// always continues from wherever live play actually is regardless of
// what's been reviewed in the meantime. targetIndex must be at or before
// the game's current question — reviewing ahead isn't supported, since
// that question hasn't been asked (or scored) yet.
func (s *Service) ReviewQuestion(ctx context.Context, id uuid.UUID, targetIndex int) (ReviewResult, error) {
	ctx, span := tracer.Start(ctx, "game.ReviewQuestion")
	defer span.End()

	g, err := s.q.GetGameSummary(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ReviewResult{}, core.ErrNotFound
		}
		return ReviewResult{}, err
	}
	if g.Status != "in_progress" || !g.CurrentQuestionIndex.Valid {
		return ReviewResult{}, core.ErrConflict
	}
	currentIndex := int(g.CurrentQuestionIndex.Int32)
	if targetIndex < 0 || targetIndex > currentIndex {
		return ReviewResult{}, core.ErrValidation
	}

	q, _, err := s.QuestionAtIndex(ctx, g.QuizID, targetIndex)
	if err != nil {
		return ReviewResult{}, err
	}
	counts, err := s.answerCounts(ctx, q.ID)
	if err != nil {
		return ReviewResult{}, err
	}
	return ReviewResult{
		Game:         summaryFromRow(g),
		Question:     q,
		AnswerCounts: counts,
		IsLive:       targetIndex == currentIndex,
	}, nil
}

// ResetQuestionAnswers deletes every answer recorded for one question in
// this game and reverses whatever points it awarded, so players can answer
// it again. It's a recovery tool for admin mistakes (e.g. the question was
// shown before everyone was ready), not something used every game — unlike
// ReviewQuestion, this does mutate state. Only questions at or before the
// game's current one can be reset, matching ReviewQuestion's rule.
func (s *Service) ResetQuestionAnswers(ctx context.Context, id uuid.UUID, targetIndex int) (ResetAnswersResult, error) {
	ctx, span := tracer.Start(ctx, "game.ResetQuestionAnswers")
	defer span.End()

	g, err := s.q.GetGameSummary(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ResetAnswersResult{}, core.ErrNotFound
		}
		return ResetAnswersResult{}, err
	}
	if g.Status != "in_progress" || !g.CurrentQuestionIndex.Valid {
		return ResetAnswersResult{}, core.ErrConflict
	}
	currentIndex := int(g.CurrentQuestionIndex.Int32)
	if targetIndex < 0 || targetIndex > currentIndex {
		return ResetAnswersResult{}, core.ErrValidation
	}

	q, _, err := s.QuestionAtIndex(ctx, g.QuizID, targetIndex)
	if err != nil {
		return ResetAnswersResult{}, err
	}

	answers, err := s.q.GetAnswersForQuestion(ctx, db.GetAnswersForQuestionParams{GameID: id, QuestionID: q.ID})
	if err != nil {
		return ResetAnswersResult{}, err
	}

	err = s.withTx(ctx, func(tx *Service) error {
		if err := tx.q.DeleteAnswersForQuestion(ctx, db.DeleteAnswersForQuestionParams{GameID: id, QuestionID: q.ID}); err != nil {
			return err
		}
		for _, a := range answers {
			if a.PointsAwarded == 0 {
				continue
			}
			if err := tx.q.AddPlayerScore(ctx, db.AddPlayerScoreParams{ID: a.PlayerID, Score: -a.PointsAwarded}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ResetAnswersResult{}, err
	}

	// g was read before the reset, but resetting answers doesn't change
	// the game row itself (status/current_question_index) or the player
	// count, so it's still valid to return here.
	leaderboard, err := s.Leaderboard(ctx, id)
	if err != nil {
		return ResetAnswersResult{}, err
	}
	return ResetAnswersResult{Game: summaryFromRow(g), Question: q, Leaderboard: leaderboard}, nil
}

func (s *Service) End(ctx context.Context, id uuid.UUID) (EndGameResult, error) {
	ctx, span := tracer.Start(ctx, "game.End")
	defer span.End()

	g, err := s.q.EndGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return EndGameResult{}, core.ErrConflict
		}
		return EndGameResult{}, err
	}
	leaderboard, err := s.Leaderboard(ctx, id)
	if err != nil {
		return EndGameResult{}, err
	}
	gamesEnded.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "force_ended")))
	return EndGameResult{Game: summaryFromRow(db.GameSummary(g)), FinalLeaderboard: leaderboard}, nil
}

func (s *Service) answerCounts(ctx context.Context, questionID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := s.q.CountAnswersByOption(ctx, questionID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		// free_text answers group into one NULL row — irrelevant here,
		// this histogram is a multiple_choice-only concept.
		if !r.SelectedOptionID.Valid {
			continue
		}
		out[uuid.UUID(r.SelectedOptionID.Bytes)] = int(r.AnswerCount)
	}
	return out, nil
}

func (s *Service) Leaderboard(ctx context.Context, gameID uuid.UUID) ([]LeaderboardEntry, error) {
	ctx, span := tracer.Start(ctx, "game.Leaderboard")
	defer span.End()

	rows, err := s.q.LeaderboardByGame(ctx, gameID)
	if err != nil {
		return nil, err
	}
	out := make([]LeaderboardEntry, len(rows))
	for i, r := range rows {
		out[i] = LeaderboardEntry{ClientID: r.ClientID, Nickname: r.Nickname, Score: int(r.Score), Rank: int(r.Rank), Color: r.Color}
	}
	return out, nil
}

func (s *Service) GetPublic(ctx context.Context, code string) (PublicGame, error) {
	ctx, span := tracer.Start(ctx, "game.GetPublic")
	defer span.End()

	g, err := s.q.GetGameSummaryByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PublicGame{}, core.ErrNotFound
		}
		return PublicGame{}, err
	}
	return PublicGame{ID: g.ID, Code: g.Code, QuizTitle: g.QuizTitle, Status: g.Status, PlayerCount: int(g.PlayerCount)}, nil
}
