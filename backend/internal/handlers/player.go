package handlers

import (
	"context"
	"errors"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/service"
)

func (h *Handlers) JoinGame(ctx context.Context, req api.JoinGameRequestObject) (api.JoinGameResponseObject, error) {
	var color string
	if req.Body.Color != nil {
		color = string(*req.Body.Color)
	}
	result, err := h.svc.JoinGame(ctx, req.Body.Code, req.Params.XClientId, req.Body.Nickname, color)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return api.JoinGame404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "no game with that code"))}, nil
		case errors.Is(err, service.ErrConflict):
			return api.JoinGame409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(apiError("conflict", "this game is no longer open to new players"))}, nil
		}
		return nil, err
	}
	logGameAction(ctx, "game.player_joined", result.Game.ID, actorPlayer, req.Params.XClientId.String(),
		"nickname", result.Player.Nickname)
	return api.JoinGame200JSONResponse{
		GameId:   result.Game.ID,
		Code:     result.Game.Code,
		Status:   api.GameStatus(result.Game.Status),
		Nickname: result.Player.Nickname,
		Color:    api.PlayerColor(result.Player.Color),
	}, nil
}

func (h *Handlers) KickPlayer(ctx context.Context, req api.KickPlayerRequestObject) (api.KickPlayerResponseObject, error) {
	if err := h.svc.KickPlayer(ctx, req.GameId, req.ClientId); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return api.KickPlayer404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		case errors.Is(err, service.ErrConflict):
			return api.KickPlayer409JSONResponse{ConflictJSONResponse: conflictGame("players can only be kicked while the game is in the lobby")}, nil
		}
		return nil, err
	}
	logGameAction(ctx, "game.player_kicked", req.GameId, actorAdmin, adminActor(ctx),
		"target.client_id", req.ClientId)
	h.hub.Kick(req.GameId, req.ClientId, "Removed by the host")
	return api.KickPlayer204Response{}, nil
}

func (h *Handlers) GetPublicGame(ctx context.Context, req api.GetPublicGameRequestObject) (api.GetPublicGameResponseObject, error) {
	game, err := h.svc.GetPublicGame(ctx, req.Code)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.GetPublicGame404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "no game with that code"))}, nil
		}
		return nil, err
	}
	return api.GetPublicGame200JSONResponse{
		Code:        game.Code,
		QuizTitle:   game.QuizTitle,
		Status:      api.GameStatus(game.Status),
		PlayerCount: game.PlayerCount,
	}, nil
}

func (h *Handlers) GetPublicLeaderboard(ctx context.Context, req api.GetPublicLeaderboardRequestObject) (api.GetPublicLeaderboardResponseObject, error) {
	game, err := h.svc.GetPublicGame(ctx, req.Code)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.GetPublicLeaderboard404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "no game with that code"))}, nil
		}
		return nil, err
	}
	gameRow, err := h.svc.GetGameByCode(ctx, game.Code)
	if err != nil {
		return nil, err
	}
	leaderboard, err := h.svc.Leaderboard(ctx, gameRow.ID)
	if err != nil {
		return nil, err
	}
	return api.GetPublicLeaderboard200JSONResponse(leaderboardToAPI(leaderboard)), nil
}
