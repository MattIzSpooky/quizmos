package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
)

type GameSummary struct {
	db.Game
	QuizTitle      string
	QuizTimed      bool
	PlayerCount    int
	TotalQuestions int
}

type GameDetail struct {
	GameSummary
	Players []db.Player
}

type LeaderboardEntry struct {
	ClientID uuid.UUID
	Nickname string
	Score    int
	Rank     int
}

// TimeOrZero converts a possibly-null Postgres timestamp to a time.Time,
// for mapping db rows into API response types.
func TimeOrZero(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func (s *Service) summarize(ctx context.Context, game db.Game) (GameSummary, error) {
	quiz, err := s.q.GetQuiz(ctx, game.QuizID)
	if err != nil {
		return GameSummary{}, err
	}
	playerCount, err := s.q.CountPlayers(ctx, game.ID)
	if err != nil {
		return GameSummary{}, err
	}
	total, err := s.q.CountQuizQuestions(ctx, game.QuizID)
	if err != nil {
		return GameSummary{}, err
	}
	return GameSummary{
		Game:           game,
		QuizTitle:      quiz.Title,
		QuizTimed:      quiz.Timed,
		PlayerCount:    int(playerCount),
		TotalQuestions: int(total),
	}, nil
}

func (s *Service) CreateGame(ctx context.Context, createdBy string, quizID uuid.UUID) (GameSummary, error) {
	count, err := s.q.CountQuizQuestions(ctx, quizID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return GameSummary{}, ErrNotFound
		}
		return GameSummary{}, err
	}
	if _, err := s.q.GetQuiz(ctx, quizID); err != nil {
		if err == pgx.ErrNoRows {
			return GameSummary{}, ErrNotFound
		}
		return GameSummary{}, err
	}
	if count == 0 {
		return GameSummary{}, ErrValidation
	}

	code, err := s.generateUniqueGameCode(ctx)
	if err != nil {
		return GameSummary{}, err
	}
	game, err := s.q.CreateGame(ctx, db.CreateGameParams{QuizID: quizID, Code: code, CreatedBy: createdBy})
	if err != nil {
		return GameSummary{}, err
	}
	return s.summarize(ctx, game)
}

func (s *Service) ListGames(ctx context.Context, status *string) ([]GameSummary, error) {
	games, err := s.q.ListGames(ctx, textParam(status))
	if err != nil {
		return nil, err
	}
	out := make([]GameSummary, 0, len(games))
	for _, g := range games {
		summary, err := s.summarize(ctx, g)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *Service) GetGameDetail(ctx context.Context, id uuid.UUID) (GameDetail, error) {
	game, err := s.q.GetGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return GameDetail{}, ErrNotFound
		}
		return GameDetail{}, err
	}
	summary, err := s.summarize(ctx, game)
	if err != nil {
		return GameDetail{}, err
	}
	players, err := s.q.ListPlayersByGame(ctx, id)
	if err != nil {
		return GameDetail{}, err
	}
	return GameDetail{GameSummary: summary, Players: players}, nil
}

// QuestionAtIndex returns the question at the given 0-based position in the
// game's quiz, along with the quiz's total question count.
func (s *Service) QuestionAtIndex(ctx context.Context, quizID uuid.UUID, index int) (QuestionWithOptions, int, error) {
	questions, err := s.ListQuestions(ctx, quizID)
	if err != nil {
		return QuestionWithOptions{}, 0, err
	}
	if index < 0 || index >= len(questions) {
		return QuestionWithOptions{}, len(questions), ErrNotFound
	}
	return questions[index], len(questions), nil
}

func (s *Service) StartGame(ctx context.Context, id uuid.UUID) (GameSummary, error) {
	game, err := s.q.StartGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return GameSummary{}, ErrConflict
		}
		return GameSummary{}, err
	}
	return s.summarize(ctx, game)
}

// AdvanceResult describes the outcome of ending the current question and
// moving on, for the caller (the REST handler) to broadcast over the
// websocket.
type AdvanceResult struct {
	Game             GameSummary
	Ended            bool
	PrevIndex        int
	PrevQuestion     QuestionWithOptions
	AnswerCounts     map[uuid.UUID]int
	NextQuestion     *QuestionWithOptions
	FinalLeaderboard []LeaderboardEntry
}

func (s *Service) AdvanceGame(ctx context.Context, id uuid.UUID) (AdvanceResult, error) {
	game, err := s.q.GetGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return AdvanceResult{}, ErrNotFound
		}
		return AdvanceResult{}, err
	}
	if game.Status != "in_progress" || !game.CurrentQuestionIndex.Valid {
		return AdvanceResult{}, ErrConflict
	}
	prevIndex := int(game.CurrentQuestionIndex.Int32)

	prevQuestion, total, err := s.QuestionAtIndex(ctx, game.QuizID, prevIndex)
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
		summary, err := s.summarize(ctx, ended)
		if err != nil {
			return AdvanceResult{}, err
		}
		leaderboard, err := s.Leaderboard(ctx, id)
		if err != nil {
			return AdvanceResult{}, err
		}
		result.Game = summary
		result.Ended = true
		result.FinalLeaderboard = leaderboard
		return result, nil
	}

	updated, err := s.q.SetCurrentQuestionIndex(ctx, db.SetCurrentQuestionIndexParams{
		ID: id, CurrentQuestionIndex: pgtype.Int4{Int32: int32(nextIndex), Valid: true},
	})
	if err != nil {
		return AdvanceResult{}, err
	}
	summary, err := s.summarize(ctx, updated)
	if err != nil {
		return AdvanceResult{}, err
	}
	nextQuestion, _, err := s.QuestionAtIndex(ctx, game.QuizID, nextIndex)
	if err != nil {
		return AdvanceResult{}, err
	}
	result.Game = summary
	result.NextQuestion = &nextQuestion
	return result, nil
}

// ReviewResult describes the question to broadcast as a read-only recap
// (question.reviewed) — the game's actual state is untouched.
type ReviewResult struct {
	Game         GameSummary
	Question     QuestionWithOptions
	AnswerCounts map[uuid.UUID]int
}

// ReviewQuestion is a pure broadcast: it never mutates the game (not the
// current-question pointer, not the answers table), so "next question"
// always continues from wherever live play actually is regardless of
// what's been reviewed in the meantime. targetIndex must be at or before
// the game's current question — reviewing ahead isn't supported, since
// that question hasn't been asked (or scored) yet.
func (s *Service) ReviewQuestion(ctx context.Context, id uuid.UUID, targetIndex int) (ReviewResult, error) {
	game, err := s.q.GetGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ReviewResult{}, ErrNotFound
		}
		return ReviewResult{}, err
	}
	if game.Status != "in_progress" || !game.CurrentQuestionIndex.Valid {
		return ReviewResult{}, ErrConflict
	}
	if targetIndex < 0 || targetIndex > int(game.CurrentQuestionIndex.Int32) {
		return ReviewResult{}, ErrValidation
	}

	summary, err := s.summarize(ctx, game)
	if err != nil {
		return ReviewResult{}, err
	}
	question, _, err := s.QuestionAtIndex(ctx, game.QuizID, targetIndex)
	if err != nil {
		return ReviewResult{}, err
	}
	counts, err := s.answerCounts(ctx, question.ID)
	if err != nil {
		return ReviewResult{}, err
	}
	return ReviewResult{Game: summary, Question: question, AnswerCounts: counts}, nil
}

type EndGameResult struct {
	Game             GameSummary
	FinalLeaderboard []LeaderboardEntry
}

func (s *Service) EndGame(ctx context.Context, id uuid.UUID) (EndGameResult, error) {
	game, err := s.q.EndGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return EndGameResult{}, ErrConflict
		}
		return EndGameResult{}, err
	}
	summary, err := s.summarize(ctx, game)
	if err != nil {
		return EndGameResult{}, err
	}
	leaderboard, err := s.Leaderboard(ctx, id)
	if err != nil {
		return EndGameResult{}, err
	}
	return EndGameResult{Game: summary, FinalLeaderboard: leaderboard}, nil
}

func (s *Service) answerCounts(ctx context.Context, questionID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := s.q.CountAnswersByOption(ctx, questionID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		out[r.SelectedOptionID] = int(r.AnswerCount)
	}
	return out, nil
}

func (s *Service) Leaderboard(ctx context.Context, gameID uuid.UUID) ([]LeaderboardEntry, error) {
	rows, err := s.q.LeaderboardByGame(ctx, gameID)
	if err != nil {
		return nil, err
	}
	out := make([]LeaderboardEntry, len(rows))
	for i, r := range rows {
		out[i] = LeaderboardEntry{ClientID: r.ClientID, Nickname: r.Nickname, Score: int(r.Score), Rank: int(r.Rank)}
	}
	return out, nil
}

type PublicGame struct {
	Code        string
	QuizTitle   string
	Status      string
	PlayerCount int
}

// GetGameByCode returns the raw game row for a join code, without
// requiring or creating a player.
func (s *Service) GetGameByCode(ctx context.Context, code string) (db.Game, error) {
	game, err := s.q.GetGameByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.Game{}, ErrNotFound
		}
		return db.Game{}, err
	}
	return game, nil
}

func (s *Service) GetPublicGame(ctx context.Context, code string) (PublicGame, error) {
	game, err := s.q.GetGameByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PublicGame{}, ErrNotFound
		}
		return PublicGame{}, err
	}
	quiz, err := s.q.GetQuiz(ctx, game.QuizID)
	if err != nil {
		return PublicGame{}, err
	}
	playerCount, err := s.q.CountPlayers(ctx, game.ID)
	if err != nil {
		return PublicGame{}, err
	}
	return PublicGame{Code: game.Code, QuizTitle: quiz.Title, Status: game.Status, PlayerCount: int(playerCount)}, nil
}

type JoinResult struct {
	Game   db.Game
	Player db.Player
}

func (s *Service) JoinGame(ctx context.Context, code string, clientID uuid.UUID, nickname string) (JoinResult, error) {
	game, err := s.q.GetGameByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return JoinResult{}, ErrNotFound
		}
		return JoinResult{}, err
	}
	if game.Status == "ended" {
		return JoinResult{}, ErrConflict
	}
	player, err := s.q.UpsertPlayer(ctx, db.UpsertPlayerParams{GameID: game.ID, ClientID: clientID, Nickname: nickname})
	if err != nil {
		return JoinResult{}, err
	}
	return JoinResult{Game: game, Player: player}, nil
}

// KickPlayer removes a player from a game's lobby. It's lobby-only —
// removing someone mid-round would orphan any in-flight answer and skew
// scoring — and it's not a ban: nothing stops the same client_id from
// joining again afterward, same as anyone else.
func (s *Service) KickPlayer(ctx context.Context, gameID, clientID uuid.UUID) error {
	game, err := s.q.GetGame(ctx, gameID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	if game.Status != "lobby" {
		return ErrConflict
	}
	n, err := s.q.DeletePlayer(ctx, db.DeletePlayerParams{GameID: gameID, ClientID: clientID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPlayer looks up an existing player by (code, client_id) without
// creating one — used to authorize a websocket connection, which must
// never create player identity itself.
func (s *Service) GetPlayerByCode(ctx context.Context, code string, clientID uuid.UUID) (db.Game, db.Player, error) {
	game, err := s.q.GetGameByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.Game{}, db.Player{}, ErrNotFound
		}
		return db.Game{}, db.Player{}, err
	}
	player, err := s.q.GetPlayer(ctx, db.GetPlayerParams{GameID: game.ID, ClientID: clientID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.Game{}, db.Player{}, ErrNotFound
		}
		return db.Game{}, db.Player{}, err
	}
	return game, player, nil
}

type AnswerResult struct {
	Correct       bool
	PointsAwarded int
	TotalScore    int
}

// SubmitAnswer validates and records a player's answer to the game's
// current question. It is called from the websocket handler.
func (s *Service) SubmitAnswer(ctx context.Context, gameID uuid.UUID, clientID, questionID, optionID uuid.UUID) (AnswerResult, error) {
	game, err := s.q.GetGame(ctx, gameID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return AnswerResult{}, ErrNotFound
		}
		return AnswerResult{}, err
	}
	if game.Status != "in_progress" || !game.CurrentQuestionIndex.Valid {
		return AnswerResult{}, ErrConflict
	}

	currentQuestion, _, err := s.QuestionAtIndex(ctx, game.QuizID, int(game.CurrentQuestionIndex.Int32))
	if err != nil {
		return AnswerResult{}, err
	}
	if currentQuestion.ID != questionID {
		return AnswerResult{}, ErrConflict
	}

	player, err := s.q.GetPlayer(ctx, db.GetPlayerParams{GameID: gameID, ClientID: clientID})
	if err != nil {
		if err == pgx.ErrNoRows {
			return AnswerResult{}, ErrNotFound
		}
		return AnswerResult{}, err
	}

	if _, err := s.q.GetAnswer(ctx, db.GetAnswerParams{QuestionID: questionID, PlayerID: player.ID}); err == nil {
		return AnswerResult{}, ErrConflict
	} else if err != pgx.ErrNoRows {
		return AnswerResult{}, err
	}

	option, err := s.q.GetOption(ctx, optionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return AnswerResult{}, ErrValidation
		}
		return AnswerResult{}, err
	}
	if option.QuestionID != questionID {
		return AnswerResult{}, ErrValidation
	}

	points := 0
	if option.IsCorrect {
		points = int(currentQuestion.Points)
	}

	var result AnswerResult
	err = s.withTx(ctx, func(tx *Service) error {
		if _, err := tx.q.CreateAnswer(ctx, db.CreateAnswerParams{
			GameID:           gameID,
			QuestionID:       questionID,
			PlayerID:         player.ID,
			SelectedOptionID: optionID,
			IsCorrect:        option.IsCorrect,
			PointsAwarded:    int32(points),
		}); err != nil {
			return err
		}
		if points != 0 {
			if err := tx.q.AddPlayerScore(ctx, db.AddPlayerScoreParams{ID: player.ID, Score: int32(points)}); err != nil {
				return err
			}
		}
		result = AnswerResult{Correct: option.IsCorrect, PointsAwarded: points, TotalScore: int(player.Score) + points}
		return nil
	})
	return result, err
}
