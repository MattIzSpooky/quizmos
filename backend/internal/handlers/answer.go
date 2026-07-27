package handlers

import (
	"context"
	"errors"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/core"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// notFoundAnswer covers GradeAnswer's 404 case, which can mean either an
// unknown game or an unknown/non-free-text answer.
func notFoundAnswer() api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse(apiError("not_found", "question or answer not found"))
}

func (h *Handlers) ListFreeTextAnswers(ctx context.Context, req api.ListFreeTextAnswersRequestObject) (api.ListFreeTextAnswersResponseObject, error) {
	answers, err := h.games.ListFreeTextAnswers(ctx, req.GameId, req.QuestionId)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.ListFreeTextAnswers404JSONResponse{NotFoundJSONResponse: notFoundGame()}, nil
		}
		return nil, err
	}
	out := make(api.ListFreeTextAnswers200JSONResponse, len(answers))
	for i, a := range answers {
		out[i] = freeTextAnswerToAPI(a)
	}
	return out, nil
}

func (h *Handlers) GradeAnswer(ctx context.Context, req api.GradeAnswerRequestObject) (api.GradeAnswerResponseObject, error) {
	graded, err := h.games.GradeAnswer(ctx, req.GameId, req.AnswerId, req.Body.Correct)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.GradeAnswer404JSONResponse{NotFoundJSONResponse: notFoundAnswer()}, nil
		}
		return nil, err
	}
	logGameAction(ctx, "game.answer_graded", req.GameId, actorAdmin, auth.Actor(ctx),
		"target.client_id", graded.ClientID, "question.id", graded.QuestionID, "correct", graded.Correct, "points_awarded", graded.PointsAwarded)

	// Tell the grader's own player their final verdict — they only ever
	// saw a "pending" answer.result when they first submitted.
	h.hub.SendToClient(req.GameId, graded.ClientID, ws.TypeAnswerResult, ws.AnswerResult{
		QuestionID:    graded.QuestionID.String(),
		Correct:       graded.Correct,
		PointsAwarded: int64(graded.PointsAwarded),
		TotalScore:    int64(graded.TotalScore),
		Pending:       false,
	})

	leaderboard, err := h.games.Leaderboard(ctx, req.GameId)
	if err != nil {
		return nil, err
	}
	h.hub.Broadcast(req.GameId, ws.TypeLeaderboardUpdated, ws.LeaderboardUpdated{
		QuestionIndex: int64(graded.QuestionIndex),
		Entries:       leaderboardEntriesPayload(leaderboard),
	})

	answers, err := h.games.ListFreeTextAnswers(ctx, req.GameId, graded.QuestionID)
	if err != nil {
		return nil, err
	}
	for _, a := range answers {
		if a.ClientID == graded.ClientID {
			return api.GradeAnswer200JSONResponse(freeTextAnswerToAPI(a)), nil
		}
	}
	return nil, errors.New("graded answer vanished before it could be re-read")
}
