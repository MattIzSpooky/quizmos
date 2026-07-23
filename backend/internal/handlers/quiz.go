package handlers

import (
	"context"
	"errors"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/service"
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
	quiz, err := h.svc.CreateQuiz(ctx, adminSubject(ctx), req.Body.Title, description, timed)
	if err != nil {
		return nil, err
	}
	return api.CreateQuiz201JSONResponse(quizToAPI(quiz)), nil
}

func (h *Handlers) ListQuizzes(ctx context.Context, _ api.ListQuizzesRequestObject) (api.ListQuizzesResponseObject, error) {
	quizzes, err := h.svc.ListQuizzes(ctx)
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
	quiz, questions, err := h.svc.GetQuizDetail(ctx, req.QuizId)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.GetQuiz404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "quiz not found"))}, nil
		}
		return nil, err
	}
	return api.GetQuiz200JSONResponse(quizDetailToAPI(quiz, questions)), nil
}

func (h *Handlers) UpdateQuiz(ctx context.Context, req api.UpdateQuizRequestObject) (api.UpdateQuizResponseObject, error) {
	quiz, err := h.svc.UpdateQuiz(ctx, req.QuizId, req.Body.Title, req.Body.Description, req.Body.Timed)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.UpdateQuiz404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "quiz not found"))}, nil
		}
		return nil, err
	}
	return api.UpdateQuiz200JSONResponse(quizToAPI(quiz)), nil
}

func (h *Handlers) DeleteQuiz(ctx context.Context, req api.DeleteQuizRequestObject) (api.DeleteQuizResponseObject, error) {
	if err := h.svc.DeleteQuiz(ctx, req.QuizId); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return api.DeleteQuiz404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse(apiError("not_found", "quiz not found"))}, nil
		case errors.Is(err, service.ErrConflict):
			return api.DeleteQuiz409JSONResponse{ConflictJSONResponse: api.ConflictJSONResponse(apiError("conflict", "a game has already been created from this quiz"))}, nil
		}
		return nil, err
	}
	return api.DeleteQuiz204Response{}, nil
}
