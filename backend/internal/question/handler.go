package question

import (
	"context"
	"errors"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/audit"
	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/core"
)

// Handler implements the question-related methods of
// api.StrictServerInterface — embedded into handlers.Handlers alongside
// the game and quiz handlers.
type Handler struct {
	svc      *Service
	keycloak *auth.Keycloak
}

func NewHandler(svc *Service, keycloak *auth.Keycloak) *Handler {
	return &Handler{svc: svc, keycloak: keycloak}
}

// questionDomain and quizDomain log audit lines ("question.action" and,
// for the one action that mutates a whole quiz's question order rather
// than any single question, "quiz.action") — see audit.Domain.
const (
	questionDomain audit.Domain = "question"
	quizDomain     audit.Domain = "quiz"
)

func notFound() api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse(apiError("not_found", "question or quiz not found"))
}

func badRequest(msg string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse(apiError("validation_error", msg))
}

func apiError(code, message string) api.Error {
	return api.Error{Code: code, Message: message}
}

func (h *Handler) CreateQuestion(ctx context.Context, req api.CreateQuestionRequestObject) (api.CreateQuestionResponseObject, error) {
	timeLimit := 30
	if req.Body.TimeLimitSeconds != nil {
		timeLimit = *req.Body.TimeLimitSeconds
	}
	points := 1000
	if req.Body.Points != nil {
		points = *req.Body.Points
	}

	var options []api.CreateQuestionOption
	if req.Body.Options != nil {
		options = *req.Body.Options
	}
	q, err := h.svc.Create(ctx, req.QuizId, string(req.Body.Type), req.Body.Prompt, timeLimit, points, optionsToService(options))
	if err != nil {
		switch {
		case errors.Is(err, core.ErrNotFound):
			return api.CreateQuestion404JSONResponse{NotFoundJSONResponse: notFound()}, nil
		case errors.Is(err, core.ErrValidation):
			return api.CreateQuestion400JSONResponse{BadRequestJSONResponse: badRequest("multiple_choice needs at least 2 options; free_text must not have any")}, nil
		}
		return nil, err
	}
	questionDomain.LogAdmin(ctx, "question.created", q.ID, "quiz.id", req.QuizId, "type", req.Body.Type)
	return api.CreateQuestion201JSONResponse(ToAPI(q)), nil
}

func (h *Handler) ListQuestions(ctx context.Context, req api.ListQuestionsRequestObject) (api.ListQuestionsResponseObject, error) {
	questions, err := h.svc.List(ctx, req.QuizId)
	if err != nil {
		return nil, err
	}
	out := make(api.ListQuestions200JSONResponse, len(questions))
	for i, q := range questions {
		out[i] = ToAPI(q)
	}
	return out, nil
}

func (h *Handler) GetQuestion(ctx context.Context, req api.GetQuestionRequestObject) (api.GetQuestionResponseObject, error) {
	q, err := h.svc.Get(ctx, req.QuizId, req.QuestionId)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.GetQuestion404JSONResponse{NotFoundJSONResponse: notFound()}, nil
		}
		return nil, err
	}
	return api.GetQuestion200JSONResponse(ToAPI(q)), nil
}

func (h *Handler) UpdateQuestion(ctx context.Context, req api.UpdateQuestionRequestObject) (api.UpdateQuestionResponseObject, error) {
	var options []OptionInput
	if req.Body.Options != nil {
		options = optionsToService(*req.Body.Options)
	}
	q, err := h.svc.Update(ctx, req.QuizId, req.QuestionId, req.Body.Prompt, req.Body.TimeLimitSeconds, req.Body.Points, options)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrNotFound):
			return api.UpdateQuestion404JSONResponse{NotFoundJSONResponse: notFound()}, nil
		case errors.Is(err, core.ErrValidation):
			return api.UpdateQuestion400JSONResponse{BadRequestJSONResponse: badRequest("multiple_choice needs at least 2 options; free_text must not have any")}, nil
		}
		return nil, err
	}
	questionDomain.LogAdmin(ctx, "question.updated", q.ID, "quiz.id", req.QuizId, "fields", updatedQuestionFields(req.Body))
	return api.UpdateQuestion200JSONResponse(ToAPI(q)), nil
}

// updatedQuestionFields lists which fields an UpdateQuestion request
// actually touched (as opposed to left unset), so "question.updated" says
// more than just that *something* changed.
func updatedQuestionFields(body *api.UpdateQuestionJSONRequestBody) []string {
	var fields []string
	if body.Prompt != nil {
		fields = append(fields, "prompt")
	}
	if body.TimeLimitSeconds != nil {
		fields = append(fields, "time_limit_seconds")
	}
	if body.Points != nil {
		fields = append(fields, "points")
	}
	if body.Options != nil {
		fields = append(fields, "options")
	}
	return fields
}

func (h *Handler) DeleteQuestion(ctx context.Context, req api.DeleteQuestionRequestObject) (api.DeleteQuestionResponseObject, error) {
	if err := h.svc.Delete(ctx, req.QuizId, req.QuestionId); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.DeleteQuestion404JSONResponse{NotFoundJSONResponse: notFound()}, nil
		}
		return nil, err
	}
	questionDomain.LogAdmin(ctx, "question.deleted", req.QuestionId, "quiz.id", req.QuizId)
	return api.DeleteQuestion204Response{}, nil
}

func (h *Handler) ReorderQuestions(ctx context.Context, req api.ReorderQuestionsRequestObject) (api.ReorderQuestionsResponseObject, error) {
	questions, err := h.svc.Reorder(ctx, req.QuizId, req.Body.QuestionIds)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrValidation):
			return api.ReorderQuestions400JSONResponse{BadRequestJSONResponse: badRequest("questionIds must exactly match the quiz's existing questions")}, nil
		}
		return nil, err
	}
	// This reorders the whole quiz's question list, not any single
	// question, so it's logged against the quiz rather than a question.id
	// that wouldn't uniquely describe what changed.
	quizDomain.LogAdmin(ctx, "quiz.questions_reordered", req.QuizId, "count", len(questions))
	out := make(api.ReorderQuestions200JSONResponse, len(questions))
	for i, q := range questions {
		out[i] = ToAPI(q)
	}
	return out, nil
}
