package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/service"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

func notFoundGame() api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse(apiError("not_found", "game not found"))
}

func conflictGame(msg string) api.ConflictJSONResponse {
	return api.ConflictJSONResponse(apiError("conflict", msg))
}

func (h *Handlers) CreateGame(ctx context.Context, req api.CreateGameRequestObject) (api.CreateGameResponseObject, error) {
	game, err := h.svc.CreateGame(ctx, adminSubject(ctx), req.Body.QuizId)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return api.CreateGame404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		case errors.Is(err, service.ErrValidation):
			return api.CreateGame400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse(apiError("validation_error", "quiz has no questions"))}, nil
		}
		return nil, err
	}
	return api.CreateGame201JSONResponse(gameToAPI(game)), nil
}

func (h *Handlers) ListGames(ctx context.Context, req api.ListGamesRequestObject) (api.ListGamesResponseObject, error) {
	var status *string
	if req.Params.Status != nil {
		s := string(*req.Params.Status)
		status = &s
	}
	games, err := h.svc.ListGames(ctx, status)
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
	detail, err := h.svc.GetGameDetail(ctx, req.GameId)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.GetGame404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		}
		return nil, err
	}
	connected := h.hub.ConnectedClientIDs(req.GameId)
	return api.GetGame200JSONResponse(gameDetailToAPI(detail, connected)), nil
}

func (h *Handlers) StartGame(ctx context.Context, req api.StartGameRequestObject) (api.StartGameResponseObject, error) {
	game, err := h.svc.StartGame(ctx, req.GameId)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrConflict):
			return api.StartGame409JSONResponse{ConflictJSONResponse: conflictGame("game is not in the lobby")}, nil
		}
		return nil, err
	}

	firstQuestion, total, err := h.svc.QuestionAtIndex(ctx, game.QuizID, 0)
	if err != nil {
		return nil, err
	}

	h.hub.Broadcast(req.GameId, ws.TypeGameStarted, ws.GameStarted{StartedAt: time.Now()})
	h.hub.Broadcast(req.GameId, ws.TypeQuestionStarted, questionStartedPayload(firstQuestion, 0, total))

	return api.StartGame200JSONResponse(gameToAPI(game)), nil
}

func (h *Handlers) AdvanceGame(ctx context.Context, req api.AdvanceGameRequestObject) (api.AdvanceGameResponseObject, error) {
	result, err := h.svc.AdvanceGame(ctx, req.GameId)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return api.AdvanceGame404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		case errors.Is(err, service.ErrConflict):
			return api.AdvanceGame409JSONResponse{ConflictJSONResponse: conflictGame("game is not in progress")}, nil
		}
		return nil, err
	}

	h.hub.Broadcast(req.GameId, ws.TypeQuestionEnded, questionEndedPayload(result.PrevQuestion, result.PrevIndex, result.AnswerCounts))

	if result.Ended {
		h.hub.Broadcast(req.GameId, ws.TypeGameEnded, ws.GameEnded{
			FinalLeaderboard: leaderboardEntriesPayload(result.FinalLeaderboard),
			EndedAt:          time.Now(),
		})
	} else {
		leaderboard, err := h.svc.Leaderboard(ctx, req.GameId)
		if err != nil {
			return nil, err
		}
		h.hub.Broadcast(req.GameId, ws.TypeLeaderboardUpdated, ws.LeaderboardUpdated{
			QuestionIndex: int64(result.PrevIndex),
			Entries:       leaderboardEntriesPayload(leaderboard),
		})
		total := result.Game.TotalQuestions
		h.hub.Broadcast(req.GameId, ws.TypeQuestionStarted, questionStartedPayload(*result.NextQuestion, result.PrevIndex+1, total))
	}

	return api.AdvanceGame200JSONResponse(gameToAPI(result.Game)), nil
}

func (h *Handlers) EndGame(ctx context.Context, req api.EndGameRequestObject) (api.EndGameResponseObject, error) {
	result, err := h.svc.EndGame(ctx, req.GameId)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			return api.EndGame404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		}
		return nil, err
	}
	h.hub.Broadcast(req.GameId, ws.TypeGameEnded, ws.GameEnded{
		FinalLeaderboard: leaderboardEntriesPayload(result.FinalLeaderboard),
		EndedAt:          time.Now(),
	})
	return api.EndGame200JSONResponse(gameToAPI(result.Game)), nil
}

func (h *Handlers) GetAdminLeaderboard(ctx context.Context, req api.GetAdminLeaderboardRequestObject) (api.GetAdminLeaderboardResponseObject, error) {
	leaderboard, err := h.svc.Leaderboard(ctx, req.GameId)
	if err != nil {
		return nil, err
	}
	return api.GetAdminLeaderboard200JSONResponse(leaderboardToAPI(leaderboard)), nil
}

func questionStartedPayload(q service.QuestionWithOptions, index, total int) ws.QuestionStarted {
	options := make([]ws.QuestionOption, len(q.Options))
	for i, o := range q.Options {
		options[i] = ws.QuestionOption{ID: o.ID.String(), Text: o.Text}
	}
	return ws.QuestionStarted{
		QuestionIndex:    int64(index),
		QuestionID:       q.ID.String(),
		Prompt:           q.Prompt,
		Options:          options,
		TimeLimitSeconds: int64(q.TimeLimitSeconds),
		TotalQuestions:   int64(total),
	}
}

func questionEndedPayload(q service.QuestionWithOptions, index int, counts map[uuid.UUID]int) ws.QuestionEnded {
	var correctID string
	answerCounts := make([]ws.AnswerCount, 0, len(q.Options))
	for _, o := range q.Options {
		if o.IsCorrect {
			correctID = o.ID.String()
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

func leaderboardEntriesPayload(entries []service.LeaderboardEntry) []ws.LeaderboardEntry {
	out := make([]ws.LeaderboardEntry, len(entries))
	for i, e := range entries {
		out[i] = ws.LeaderboardEntry{ClientID: e.ClientID.String(), Nickname: e.Nickname, Score: int64(e.Score), Rank: int64(e.Rank)}
	}
	return out
}
