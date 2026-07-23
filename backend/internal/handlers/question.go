package handlers

import (
	"context"
	"errors"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/service"
)

func notFoundQuestion() api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse(apiError("not_found", "question or quiz not found"))
}

func badRequestQuestion(msg string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse(apiError("validation_error", msg))
}

func (h *Handlers) CreateQuestion(ctx context.Context, req api.CreateQuestionRequestObject) (api.CreateQuestionResponseObject, error) {
	timeLimit := 30
	if req.Body.TimeLimitSeconds != nil {
		timeLimit = *req.Body.TimeLimitSeconds
	}
	points := 1000
	if req.Body.Points != nil {
		points = *req.Body.Points
	}

	question, err := h.svc.CreateQuestion(ctx, req.QuizId, string(req.Body.Type), req.Body.Prompt, timeLimit, points, createQuestionOptionsToService(req.Body.Options))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return api.CreateQuestion404JSONResponse{NotFoundJSONResponse: notFoundQuestion()}, nil
		case errors.Is(err, service.ErrValidation):
			return api.CreateQuestion400JSONResponse{BadRequestJSONResponse: badRequestQuestion("a question needs at least 2 options")}, nil
		}
		return nil, err
	}
	return api.CreateQuestion201JSONResponse(questionToAPI(question)), nil
}

func (h *Handlers) ListQuestions(ctx context.Context, req api.ListQuestionsRequestObject) (api.ListQuestionsResponseObject, error) {
	questions, err := h.svc.ListQuestions(ctx, req.QuizId)
	if err != nil {
		return nil, err
	}
	out := make(api.ListQuestions200JSONResponse, len(questions))
	for i, q := range questions {
		out[i] = questionToAPI(q)
	}
	return out, nil
}

func (h *Handlers) GetQuestion(ctx context.Context, req api.GetQuestionRequestObject) (api.GetQuestionResponseObject, error) {
	question, err := h.svc.GetQuestion(ctx, req.QuizId, req.QuestionId)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.GetQuestion404JSONResponse{NotFoundJSONResponse: notFoundQuestion()}, nil
		}
		return nil, err
	}
	return api.GetQuestion200JSONResponse(questionToAPI(question)), nil
}

func (h *Handlers) UpdateQuestion(ctx context.Context, req api.UpdateQuestionRequestObject) (api.UpdateQuestionResponseObject, error) {
	var options []service.QuestionOptionInput
	if req.Body.Options != nil {
		options = createQuestionOptionsToService(*req.Body.Options)
	}
	question, err := h.svc.UpdateQuestion(ctx, req.QuizId, req.QuestionId, req.Body.Prompt, req.Body.TimeLimitSeconds, req.Body.Points, options)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return api.UpdateQuestion404JSONResponse{NotFoundJSONResponse: notFoundQuestion()}, nil
		case errors.Is(err, service.ErrValidation):
			return api.UpdateQuestion400JSONResponse{BadRequestJSONResponse: badRequestQuestion("a question needs at least 2 options")}, nil
		}
		return nil, err
	}
	return api.UpdateQuestion200JSONResponse(questionToAPI(question)), nil
}

func (h *Handlers) DeleteQuestion(ctx context.Context, req api.DeleteQuestionRequestObject) (api.DeleteQuestionResponseObject, error) {
	if err := h.svc.DeleteQuestion(ctx, req.QuizId, req.QuestionId); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.DeleteQuestion404JSONResponse{NotFoundJSONResponse: notFoundQuestion()}, nil
		}
		return nil, err
	}
	return api.DeleteQuestion204Response{}, nil
}

func (h *Handlers) ReorderQuestions(ctx context.Context, req api.ReorderQuestionsRequestObject) (api.ReorderQuestionsResponseObject, error) {
	questions, err := h.svc.ReorderQuestions(ctx, req.QuizId, req.Body.QuestionIds)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrValidation):
			return api.ReorderQuestions400JSONResponse{BadRequestJSONResponse: badRequestQuestion("questionIds must exactly match the quiz's existing questions")}, nil
		}
		return nil, err
	}
	out := make(api.ReorderQuestions200JSONResponse, len(questions))
	for i, q := range questions {
		out[i] = questionToAPI(q)
	}
	return out, nil
}
