package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/core"
	"github.com/mattizspooky/quizmos/backend/internal/game"
	"github.com/mattizspooky/quizmos/backend/internal/question"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

func notFoundGame() api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse(apiError("not_found", "game not found"))
}

func conflictGame(msg string) api.ConflictJSONResponse {
	return api.ConflictJSONResponse(apiError("conflict", msg))
}

func (h *Handlers) CreateGame(ctx context.Context, req api.CreateGameRequestObject) (api.CreateGameResponseObject, error) {
	g, err := h.games.Create(ctx, auth.Subject(ctx), req.Body.QuizId)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrNotFound):
			return api.CreateGame404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		case errors.Is(err, core.ErrValidation):
			return api.CreateGame400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(apiError("validation_error", "quiz has no questions"))}, nil
		}
		return nil, err
	}
	logGameAction(ctx, "game.created", g.ID, actorAdmin, auth.Actor(ctx), "quiz.id", req.Body.QuizId)
	return api.CreateGame201JSONResponse(gameToAPI(g)), nil
}

func (h *Handlers) ListGames(ctx context.Context, req api.ListGamesRequestObject) (api.ListGamesResponseObject, error) {
	var status *string
	if req.Params.Status != nil {
		s := string(*req.Params.Status)
		status = &s
	}
	games, err := h.games.List(ctx, status)
	if err != nil {
		return nil, err
	}
	out := make(api.ListGames200JSONResponse, len(games))
	for i, g := range games {
		out[i] = gameToAPI(g)
	}
	return out, nil
}

func (h *Handlers) GetGame(ctx context.Context, req api.GetGameRequestObject) (api.GetGameResponseObject, error) {
	detail, err := h.games.GetDetail(ctx, req.GameId)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.GetGame404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		}
		return nil, err
	}
	connected := h.hub.ConnectedClientIDs(req.GameId)
	return api.GetGame200JSONResponse(gameDetailToAPI(detail, connected)), nil
}

func (h *Handlers) StartGame(ctx context.Context, req api.StartGameRequestObject) (api.StartGameResponseObject, error) {
	g, err := h.games.Start(ctx, req.GameId)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrConflict):
			return api.StartGame409JSONResponse{ConflictJSONResponse: conflictGame("game is not in the lobby")}, nil
		}
		return nil, err
	}

	firstQuestion, total, err := h.games.QuestionAtIndex(ctx, g.QuizID, 0)
	if err != nil {
		return nil, err
	}

	logGameAction(ctx, "game.started", req.GameId, actorAdmin, auth.Actor(ctx))
	h.hub.Broadcast(req.GameId, ws.TypeGameStarted, ws.GameStarted{StartedAt: time.Now()})
	h.hub.Broadcast(req.GameId, ws.TypeQuestionStarted, questionStartedPayload(firstQuestion, 0, total, g.QuizTimed))

	return api.StartGame200JSONResponse(gameToAPI(g)), nil
}

func (h *Handlers) AdvanceGame(ctx context.Context, req api.AdvanceGameRequestObject) (api.AdvanceGameResponseObject, error) {
	result, err := h.games.Advance(ctx, req.GameId)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrNotFound):
			return api.AdvanceGame404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		case errors.Is(err, core.ErrConflict):
			return api.AdvanceGame409JSONResponse{ConflictJSONResponse: conflictGame("game is not in progress")}, nil
		}
		return nil, err
	}

	logGameAction(ctx, "game.advanced", req.GameId, actorAdmin, auth.Actor(ctx),
		"prev_question_index", result.PrevIndex, "ended", result.Ended)
	h.hub.Broadcast(req.GameId, ws.TypeQuestionEnded, questionEndedPayload(result.PrevQuestion, result.PrevIndex, result.AnswerCounts))

	if result.Ended {
		h.hub.Broadcast(req.GameId, ws.TypeGameEnded, ws.GameEnded{
			FinalLeaderboard: leaderboardEntriesPayload(result.FinalLeaderboard),
			EndedAt:          time.Now(),
		})
		h.hub.CloseRoom(req.GameId)
	} else {
		leaderboard, err := h.games.Leaderboard(ctx, req.GameId)
		if err != nil {
			return nil, err
		}
		h.hub.Broadcast(req.GameId, ws.TypeLeaderboardUpdated, ws.LeaderboardUpdated{
			QuestionIndex: int64(result.PrevIndex),
			Entries:       leaderboardEntriesPayload(leaderboard),
		})
		total := result.Game.TotalQuestions
		h.hub.Broadcast(req.GameId, ws.TypeQuestionStarted, questionStartedPayload(*result.NextQuestion, result.PrevIndex+1, total, result.Game.QuizTimed))
	}

	return api.AdvanceGame200JSONResponse(gameToAPI(result.Game)), nil
}

func (h *Handlers) ReviewQuestion(ctx context.Context, req api.ReviewQuestionRequestObject) (api.ReviewQuestionResponseObject, error) {
	result, err := h.games.ReviewQuestion(ctx, req.GameId, req.Body.QuestionIndex)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrNotFound):
			return api.ReviewQuestion404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		case errors.Is(err, core.ErrConflict):
			return api.ReviewQuestion409JSONResponse{ConflictJSONResponse: conflictGame("game is not in progress")}, nil
		case errors.Is(err, core.ErrValidation):
			return api.ReviewQuestion400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(apiError("validation_error", "questionIndex must be at or before the current question"))}, nil
		}
		return nil, err
	}

	logGameAction(ctx, "game.question_reviewed", req.GameId, actorAdmin, auth.Actor(ctx),
		"question_index", req.Body.QuestionIndex, "is_live", result.IsLive)

	if result.IsLive {
		// Switching back to the actual current question resumes live
		// play — it may still be open for answers, and question.reviewed
		// always reveals the correct answer, which would leak it early.
		// BroadcastQuestionStarted (not a plain Broadcast) personalizes
		// each recipient's copy with their own answer, if they already
		// submitted one before the admin reviewed away and back.
		h.hub.BroadcastQuestionStarted(ctx, req.GameId, questionStartedPayload(
			result.Question, int(result.Question.Position), result.Game.TotalQuestions, result.Game.QuizTimed,
		))
	} else {
		h.hub.Broadcast(req.GameId, ws.TypeQuestionReviewed, questionReviewedPayload(result.Question, result.Game.TotalQuestions, result.AnswerCounts))
	}

	return api.ReviewQuestion200JSONResponse(gameToAPI(result.Game)), nil
}

func (h *Handlers) ResetQuestionAnswers(ctx context.Context, req api.ResetQuestionAnswersRequestObject) (api.ResetQuestionAnswersResponseObject, error) {
	result, err := h.games.ResetQuestionAnswers(ctx, req.GameId, req.Body.QuestionIndex)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrNotFound):
			return api.ResetQuestionAnswers404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		case errors.Is(err, core.ErrConflict):
			return api.ResetQuestionAnswers409JSONResponse{ConflictJSONResponse: conflictGame("game is not in progress")}, nil
		case errors.Is(err, core.ErrValidation):
			return api.ResetQuestionAnswers400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(apiError("validation_error", "questionIndex must be at or before the current question"))}, nil
		}
		return nil, err
	}

	logGameAction(ctx, "game.answers_reset", req.GameId, actorAdmin, auth.Actor(ctx),
		"question_index", req.Body.QuestionIndex)
	h.hub.Broadcast(req.GameId, ws.TypeQuestionAnswersReset, ws.QuestionAnswersReset{
		QuestionIndex: int64(result.Question.Position),
		QuestionID:    result.Question.ID.String(),
	})
	h.hub.Broadcast(req.GameId, ws.TypeLeaderboardUpdated, ws.LeaderboardUpdated{
		QuestionIndex: int64(result.Question.Position),
		Entries:       leaderboardEntriesPayload(result.Leaderboard),
	})

	return api.ResetQuestionAnswers200JSONResponse(gameToAPI(result.Game)), nil
}

func (h *Handlers) EndGame(ctx context.Context, req api.EndGameRequestObject) (api.EndGameResponseObject, error) {
	result, err := h.games.End(ctx, req.GameId)
	if err != nil {
		if errors.Is(err, core.ErrConflict) {
			return api.EndGame404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		}
		return nil, err
	}
	logGameAction(ctx, "game.ended", req.GameId, actorAdmin, auth.Actor(ctx))
	h.hub.Broadcast(req.GameId, ws.TypeGameEnded, ws.GameEnded{
		FinalLeaderboard: leaderboardEntriesPayload(result.FinalLeaderboard),
		EndedAt:          time.Now(),
	})
	h.hub.CloseRoom(req.GameId)
	return api.EndGame200JSONResponse(gameToAPI(result.Game)), nil
}

func (h *Handlers) GetAdminLeaderboard(ctx context.Context, req api.GetAdminLeaderboardRequestObject) (api.GetAdminLeaderboardResponseObject, error) {
	leaderboard, err := h.games.Leaderboard(ctx, req.GameId)
	if err != nil {
		return nil, err
	}
	return api.GetAdminLeaderboard200JSONResponse(leaderboardToAPI(leaderboard)), nil
}

// mediaFields returns q's attached media as pointers suitable for an
// optional websocket payload field — both nil when there's none.
func mediaFields(q question.WithOptions) (*string, *ws.MediaType) {
	if q.MediaURL == "" {
		return nil, nil
	}
	url := q.MediaURL
	mediaType := ws.MediaType(q.MediaType.String)
	return &url, &mediaType
}

func questionStartedPayload(q question.WithOptions, index, total int, timed bool) ws.QuestionStarted {
	options := make([]ws.QuestionOption, len(q.Options))
	for i, o := range q.Options {
		options[i] = ws.QuestionOption{ID: o.ID.String(), Text: o.Text}
	}
	mediaURL, mediaType := mediaFields(q)
	return ws.QuestionStarted{
		QuestionIndex:    int64(index),
		QuestionID:       q.ID.String(),
		Type:             ws.Type(q.Type),
		Prompt:           q.Prompt,
		Options:          options,
		Timed:            timed,
		TimeLimitSeconds: int64(q.TimeLimitSeconds),
		TotalQuestions:   int64(total),
		MediaURL:         mediaURL,
		MediaType:        mediaType,
	}
}

// questionReviewedPayload renders a read-only recap of a question that's
// already been through question.ended — the correct answer is included
// up front, unlike questionStartedPayload. correctOptionId/answerCounts
// only apply to multiple_choice; free_text has neither.
func questionReviewedPayload(q question.WithOptions, total int, counts map[uuid.UUID]int) ws.QuestionReviewed {
	options := make([]ws.QuestionOption, len(q.Options))
	var correctID *string
	answerCounts := make([]ws.AnswerCount, 0, len(q.Options))
	for i, o := range q.Options {
		options[i] = ws.QuestionOption{ID: o.ID.String(), Text: o.Text}
		if o.IsCorrect {
			id := o.ID.String()
			correctID = &id
		}
		answerCounts = append(answerCounts, ws.AnswerCount{OptionID: o.ID.String(), Count: int64(counts[o.ID])})
	}
	mediaURL, mediaType := mediaFields(q)
	return ws.QuestionReviewed{
		QuestionIndex:   int64(q.Position),
		QuestionID:      q.ID.String(),
		Prompt:          q.Prompt,
		Options:         options,
		CorrectOptionID: correctID,
		AnswerCounts:    answerCounts,
		TotalQuestions:  int64(total),
		MediaURL:        mediaURL,
		MediaType:       mediaType,
	}
}

func leaderboardEntriesPayload(entries []game.LeaderboardEntry) []ws.LeaderboardEntry {
	out := make([]ws.LeaderboardEntry, len(entries))
	for i, e := range entries {
		out[i] = ws.LeaderboardEntry{ClientID: e.ClientID.String(), Nickname: e.Nickname, Score: int64(e.Score), Rank: int64(e.Rank), Color: ws.Color(e.Color)}
	}
	return out
}

func questionEndedPayload(q question.WithOptions, index int, counts map[uuid.UUID]int) ws.QuestionEnded {
	var correctID *string
	answerCounts := make([]ws.AnswerCount, 0, len(q.Options))
	for _, o := range q.Options {
		if o.IsCorrect {
			id := o.ID.String()
			correctID = &id
		}
		answerCounts = append(answerCounts, ws.AnswerCount{OptionID: o.ID.String(), Count: int64(counts[o.ID])})
	}
	return ws.QuestionEnded{
		QuestionIndex:   int64(index),
		QuestionID:      q.ID.String(),
		CorrectOptionID: correctID,
		AnswerCounts:    answerCounts,
	}
}
