package handlers

import (
	"context"
	"errors"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/core"
)

func (h *Handlers) CreateQuiz(ctx context.Context, req api.CreateQuizRequestObject) (api.CreateQuizResponseObject, error) {
	description := ""
	if req.Body.Description != nil {
		description = *req.Body.Description
	}
	timed := true
	if req.Body.Timed != nil {
		timed = *req.Body.Timed
	}
	q, err := h.quizzes.Create(ctx, auth.Subject(ctx), req.Body.Title, description, timed)
	if err != nil {
		return nil, err
	}
	logQuizAction(ctx, "quiz.created", q.ID, "title", req.Body.Title)
	return api.CreateQuiz201JSONResponse(quizToAPI(q)), nil
}

func (h *Handlers) ListQuizzes(ctx context.Context, _ api.ListQuizzesRequestObject) (api.ListQuizzesResponseObject, error) {
	quizzes, err := h.quizzes.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListQuizzes200JSONResponse, len(quizzes))
	for i, q := range quizzes {
		out[i] = quizToAPI(q)
	}
	return out, nil
}

func (h *Handlers) GetQuiz(ctx context.Context, req api.GetQuizRequestObject) (api.GetQuizResponseObject, error) {
	q, questions, err := h.quizzes.GetDetail(ctx, req.QuizId)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.GetQuiz404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "quiz not found"))}, nil
		}
		return nil, err
	}
	return api.GetQuiz200JSONResponse(quizDetailToAPI(q, questions)), nil
}

func (h *Handlers) UpdateQuiz(ctx context.Context, req api.UpdateQuizRequestObject) (api.UpdateQuizResponseObject, error) {
	q, err := h.quizzes.Update(ctx, req.QuizId, req.Body.Title, req.Body.Description, req.Body.Timed)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.UpdateQuiz404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "quiz not found"))}, nil
		}
		return nil, err
	}
	logQuizAction(ctx, "quiz.updated", q.ID)
	return api.UpdateQuiz200JSONResponse(quizToAPI(q)), nil
}

func (h *Handlers) DeleteQuiz(ctx context.Context, req api.DeleteQuizRequestObject) (api.DeleteQuizResponseObject, error) {
	gameIDs, err := h.quizzes.Delete(ctx, req.QuizId)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.DeleteQuiz404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "quiz not found"))}, nil
		}
		return nil, err
	}
	logQuizAction(ctx, "quiz.deleted", req.QuizId, "games_closed", len(gameIDs))
	// Any game played from this quiz is gone too now — disconnect anyone
	// still connected to its room rather than leaving them hanging on a
	// game that no longer exists.
	for _, gameID := range gameIDs {
		h.hub.CloseRoom(gameID)
	}
	return api.DeleteQuiz204Response{}, nil
}
