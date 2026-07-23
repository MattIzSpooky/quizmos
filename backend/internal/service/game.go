package service

import (
	"context"
	"strings"
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
	Color    string
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

// ReviewResult describes the question to broadcast — the game's actual
// state is untouched either way (see ReviewQuestion).
type ReviewResult struct {
	Game         GameSummary
	Question     QuestionWithOptions
	AnswerCounts map[uuid.UUID]int
	// IsLive is true when targetIndex is the game's actual current
	// question, not an earlier, already-ended one. The caller should
	// broadcast question.started (resume live play), not
	// question.reviewed — that question may still be open for answers,
	// and question.reviewed always reveals the correct answer, which
	// would leak it early.
	IsLive bool
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
	currentIndex := int(game.CurrentQuestionIndex.Int32)
	if targetIndex < 0 || targetIndex > currentIndex {
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
	return ReviewResult{
		Game:         summary,
		Question:     question,
		AnswerCounts: counts,
		IsLive:       targetIndex == currentIndex,
	}, nil
}

// ResetAnswersResult describes the outcome of wiping a question's answers,
// for the caller to broadcast over the websocket.
type ResetAnswersResult struct {
	Game        GameSummary
	Question    QuestionWithOptions
	Leaderboard []LeaderboardEntry
}

// ResetQuestionAnswers deletes every answer recorded for one question in
// this game and reverses whatever points it awarded, so players can answer
// it again. It's a recovery tool for admin mistakes (e.g. the question was
// shown before everyone was ready), not something used every game — unlike
// ReviewQuestion, this does mutate state. Only questions at or before the
// game's current one can be reset, matching ReviewQuestion's rule.
func (s *Service) ResetQuestionAnswers(ctx context.Context, id uuid.UUID, targetIndex int) (ResetAnswersResult, error) {
	game, err := s.q.GetGame(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ResetAnswersResult{}, ErrNotFound
		}
		return ResetAnswersResult{}, err
	}
	if game.Status != "in_progress" || !game.CurrentQuestionIndex.Valid {
		return ResetAnswersResult{}, ErrConflict
	}
	currentIndex := int(game.CurrentQuestionIndex.Int32)
	if targetIndex < 0 || targetIndex > currentIndex {
		return ResetAnswersResult{}, ErrValidation
	}

	question, _, err := s.QuestionAtIndex(ctx, game.QuizID, targetIndex)
	if err != nil {
		return ResetAnswersResult{}, err
	}

	answers, err := s.q.GetAnswersForQuestion(ctx, db.GetAnswersForQuestionParams{GameID: id, QuestionID: question.ID})
	if err != nil {
		return ResetAnswersResult{}, err
	}

	err = s.withTx(ctx, func(tx *Service) error {
		if err := tx.q.DeleteAnswersForQuestion(ctx, db.DeleteAnswersForQuestionParams{GameID: id, QuestionID: question.ID}); err != nil {
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

	summary, err := s.summarize(ctx, game)
	if err != nil {
		return ResetAnswersResult{}, err
	}
	leaderboard, err := s.Leaderboard(ctx, id)
	if err != nil {
		return ResetAnswersResult{}, err
	}
	return ResetAnswersResult{Game: summary, Question: question, Leaderboard: leaderboard}, nil
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

// JoinGame only admits new players while the game is in the lobby. Once
// it's in_progress, a late join would drop someone into a quiz they never
// saw the earlier questions of with a clean scoreboard entry anyway, so
// there's no useful "late join" here — and once ended there's nothing to
// join at all. Reconnecting mid-game (e.g. a dropped connection) doesn't
// go through this path: it's a websocket concern against a player row
// that already exists, not a fresh join.
func (s *Service) JoinGame(ctx context.Context, code string, clientID uuid.UUID, nickname, color string) (JoinResult, error) {
	game, err := s.q.GetGameByCode(ctx, code)
	if err != nil {
		if err == pgx.ErrNoRows {
			return JoinResult{}, ErrNotFound
		}
		return JoinResult{}, err
	}
	if game.Status != "lobby" {
		return JoinResult{}, ErrConflict
	}
	player, err := s.q.UpsertPlayer(ctx, db.UpsertPlayerParams{
		GameID: game.ID, ClientID: clientID, Nickname: nickname, Color: NormalizePlayerColor(color),
	})
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
	// Pending is true for a free-text answer awaiting the admin's manual
	// grade — Correct/PointsAwarded are meaningless (zero) until then.
	Pending bool
}

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

	params := db.CreateAnswerParams{GameID: gameID, QuestionID: questionID, PlayerID: player.ID}
	var result AnswerResult

	switch currentQuestion.Type {
	case QuestionTypeMultipleChoice:
		if optionID == nil {
			return AnswerResult{}, ErrValidation
		}
		option, err := s.q.GetOption(ctx, *optionID)
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
		params.SelectedOptionID = pgtype.UUID{Bytes: *optionID, Valid: true}
		params.IsCorrect = pgtype.Bool{Bool: option.IsCorrect, Valid: true}
		params.PointsAwarded = int32(points)
		result = AnswerResult{Correct: option.IsCorrect, PointsAwarded: points, TotalScore: int(player.Score) + points}

	case QuestionTypeFreeText:
		if text == nil {
			return AnswerResult{}, ErrValidation
		}
		trimmed := strings.TrimSpace(*text)
		if trimmed == "" || len([]rune(trimmed)) > MaxFreeTextAnswerLength {
			return AnswerResult{}, ErrValidation
		}
		// SelectedOptionID/IsCorrect are left at their zero value (SQL
		// NULL) — grading happens later, by hand, via GradeAnswer.
		params.AnswerText = pgtype.Text{String: trimmed, Valid: true}
		result = AnswerResult{Pending: true, TotalScore: int(player.Score)}

	default:
		return AnswerResult{}, ErrValidation
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
	return result, err
}

// GradedAnswer describes the outcome of manually grading a free-text
// answer, for the caller (the REST handler) to notify the affected player
// and broadcast the updated leaderboard over the websocket.
type GradedAnswer struct {
	ClientID      uuid.UUID
	QuestionID    uuid.UUID
	QuestionIndex int
	Correct       bool
	PointsAwarded int
	TotalScore    int
}

// GradeAnswer manually grades a free-text answer: full points for the
// question if correct, none otherwise. It can be called again for the
// same answer to correct a grading mistake — the player's score is
// adjusted by the difference from whatever was previously awarded, not
// simply added to again.
func (s *Service) GradeAnswer(ctx context.Context, gameID, answerID uuid.UUID, correct bool) (GradedAnswer, error) {
	answer, err := s.q.GetAnswerByID(ctx, answerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return GradedAnswer{}, ErrNotFound
		}
		return GradedAnswer{}, err
	}
	if answer.GameID != gameID || !answer.AnswerText.Valid {
		return GradedAnswer{}, ErrNotFound
	}

	question, err := s.q.GetQuestionByID(ctx, answer.QuestionID)
	if err != nil {
		return GradedAnswer{}, err
	}
	newPoints := int32(0)
	if correct {
		newPoints = question.Points
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

	return GradedAnswer{
		ClientID:      player.ClientID,
		QuestionID:    answer.QuestionID,
		QuestionIndex: int(question.Position),
		Correct:       correct,
		PointsAwarded: int(newPoints),
		TotalScore:    int(player.Score),
	}, nil
}

// FreeTextAnswer is one player's submission to a free_text question, for
// the admin's grading view.
type FreeTextAnswer struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	Nickname      string
	Text          string
	Graded        bool
	Correct       bool
	PointsAwarded int
}

// ListFreeTextAnswers returns every free-text answer submitted so far for
// one question in this game, oldest first, for the admin to grade.
func (s *Service) ListFreeTextAnswers(ctx context.Context, gameID, questionID uuid.UUID) ([]FreeTextAnswer, error) {
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

// PlayerAnswerStatus is one player's own answer to one question, if they
// have one — used to fold "you already answered this" into a
// question.started message. Without it, a client that receives
// question.started for a question it already answered (resuming live
// play after a review, or reconnecting mid-question) would show a
// blank, re-answerable question, even though resubmitting would be
// rejected as a duplicate.
type PlayerAnswerStatus struct {
	Answered         bool
	SelectedOptionID *uuid.UUID
	Text             *string
	Pending          bool
	Correct          bool
	PointsAwarded    int
}

// GetPlayerAnswerStatus looks up clientID's own answer (if any) to
// questionID in gameID. A player who hasn't answered, or who isn't found
// (shouldn't normally happen for a connected client), gets the zero value
// (Answered: false) rather than an error — the caller treats both the
// same way.
func (s *Service) GetPlayerAnswerStatus(ctx context.Context, gameID, clientID, questionID uuid.UUID) (PlayerAnswerStatus, error) {
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
