package question

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/mattizspooky/quizmos/backend/internal/api"
	"github.com/mattizspooky/quizmos/backend/internal/core"
)

// isForbidden reports whether err (from Keycloak.RequireAdminToken) is a
// "wrong role" failure (403) rather than a missing/invalid token (401).
func isForbidden(err error) bool {
	var withStatus interface{ HTTPStatus() int }
	return errors.As(err, &withStatus) && withStatus.HTTPStatus() == http.StatusForbidden
}

// authHeaderValue reads an optional Authorization header param as a
// plain string, treating a completely absent header the same as an
// empty one — both fail RequireAdminToken's "Bearer " prefix check the
// same way, so a missing header still reaches the handler's own auth
// check (and its proper 401 JSON response) instead of failing earlier
// in generated parameter-binding code with a raw 400.
func authHeaderValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// UploadQuestionMedia and DeleteQuestionMedia check the Authorization
// header themselves, rather than relying on the OpenAPI request
// validator's usual security-scheme enforcement — see the Authorization
// parameter's description in api/openapi.yaml for why: the validator
// would otherwise consume the multipart body before the handler gets a
// chance to stream it to storage.
func (h *Handler) UploadQuestionMedia(ctx context.Context, req api.UploadQuestionMediaRequestObject) (api.UploadQuestionMediaResponseObject, error) {
	if _, err := h.keycloak.RequireAdminToken(authHeaderValue(req.Params.Authorization)); err != nil {
		if isForbidden(err) {
			return api.UploadQuestionMedia403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(apiError("forbidden", err.Error()))}, nil
		}
		return api.UploadQuestionMedia401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(apiError("unauthorized", err.Error()))}, nil
	}

	part, err := nextFilePart(req.Body)
	if err != nil {
		return api.UploadQuestionMedia400JSONResponse{BadRequestJSONResponse: badRequest(`expected a multipart file field named "file"`)}, nil
	}
	defer part.Close()

	contentType := part.Header.Get("Content-Type")
	_, limit, ok := MediaLimitBytes(contentType)
	if !ok {
		return api.UploadQuestionMedia400JSONResponse{BadRequestJSONResponse: badRequest("unsupported media type: " + contentType)}, nil
	}
	// Multipart parts don't carry a size header, so the only way to
	// enforce the cap is to read up to one byte past it: if that many
	// bytes actually come through, the file is too big.
	data, err := io.ReadAll(io.LimitReader(part, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return api.UploadQuestionMedia400JSONResponse{BadRequestJSONResponse: badRequest("file too large")}, nil
	}

	q, err := h.svc.UploadMedia(ctx, req.QuizId, req.QuestionId, contentType, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		switch {
		case errors.Is(err, core.ErrNotFound):
			return api.UploadQuestionMedia404JSONResponse{NotFoundJSONResponse: notFound()}, nil
		case errors.Is(err, core.ErrValidation):
			return api.UploadQuestionMedia400JSONResponse{BadRequestJSONResponse: badRequest("unsupported media type: " + contentType)}, nil
		}
		return nil, err
	}
	return api.UploadQuestionMedia200JSONResponse(ToAPI(q)), nil
}

func (h *Handler) DeleteQuestionMedia(ctx context.Context, req api.DeleteQuestionMediaRequestObject) (api.DeleteQuestionMediaResponseObject, error) {
	if _, err := h.keycloak.RequireAdminToken(authHeaderValue(req.Params.Authorization)); err != nil {
		if isForbidden(err) {
			return api.DeleteQuestionMedia403JSONResponse{ForbiddenJSONResponse: api.ForbiddenJSONResponse(apiError("forbidden", err.Error()))}, nil
		}
		return api.DeleteQuestionMedia401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(apiError("unauthorized", err.Error()))}, nil
	}

	q, err := h.svc.DeleteMedia(ctx, req.QuizId, req.QuestionId)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return api.DeleteQuestionMedia404JSONResponse{NotFoundJSONResponse: notFound()}, nil
		}
		return nil, err
	}
	return api.DeleteQuestionMedia200JSONResponse(ToAPI(q)), nil
}

// nextFilePart returns the first part named "file" in mr, skipping any
// other fields a client might have sent alongside it.
func nextFilePart(mr *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := mr.NextPart()
		if err != nil {
			return nil, err
		}
		if part.FormName() == "file" {
			return part, nil
		}
		part.Close()
	}
}
