package e2e

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

// registerAdminAuthSteps covers admin_auth.feature: every admin endpoint
// should reject anonymous or under-privileged requests. These exercise
// the actual wired-together request pipeline (chi router ->
// openapi3filter -> Keycloak.AuthenticationFunc, or, for the two media
// routes, the handler's own manual check — see internal/handlers/media.go)
// rather than the auth package in isolation, which is already covered by
// unit tests. Every other admin step in this suite always uses a valid
// w.adminToken, so without these, a broken security wiring (e.g. a route
// accidentally left off the security scheme, or the media Skipper
// misconfigured) would go unnoticed.
func registerAdminAuthSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I am authenticated as an admin$`, iAmAuthenticatedAsAnAdmin)
	sc.Step(`^I request the admin quizzes list with no bearer token$`, iRequestTheAdminQuizzesListWithNoBearerToken)
	sc.Step(`^I request the admin quizzes list with an invalid bearer token$`, iRequestTheAdminQuizzesListWithAnInvalidBearerToken)
	sc.Step(`^I request the admin quizzes list as a user without the admin role$`, iRequestTheAdminQuizzesListAsAUserWithoutTheAdminRole)
	sc.Step(`^I try to upload media with no bearer token$`, iTryToUploadMediaWithNoBearerToken)
	sc.Step(`^I try to upload media with an invalid bearer token$`, iTryToUploadMediaWithAnInvalidBearerToken)
	sc.Step(`^I try to upload media as a user without the admin role$`, iTryToUploadMediaAsAUserWithoutTheAdminRole)
	sc.Step(`^I try to delete media with no bearer token$`, iTryToDeleteMediaWithNoBearerToken)
	sc.Step(`^I try to delete media with an invalid bearer token$`, iTryToDeleteMediaWithAnInvalidBearerToken)
	sc.Step(`^I try to delete media as a user without the admin role$`, iTryToDeleteMediaAsAUserWithoutTheAdminRole)
}

func iAmAuthenticatedAsAnAdmin(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.adminToken(ctx)
	if err != nil {
		return err
	}
	w.adminToken = token
	return nil
}

const invalidBearerToken = "not-a-real-token"

func iRequestTheAdminQuizzesListWithNoBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.request(ctx, http.MethodGet, "/admin/quizzes", "", "", nil)
	w.lastResponse = resp
	return err
}

func iRequestTheAdminQuizzesListWithAnInvalidBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.request(ctx, http.MethodGet, "/admin/quizzes", invalidBearerToken, "", nil)
	w.lastResponse = resp
	return err
}

func iRequestTheAdminQuizzesListAsAUserWithoutTheAdminRole(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.noRoleToken(ctx)
	if err != nil {
		return err
	}
	resp, err := w.request(ctx, http.MethodGet, "/admin/quizzes", token, "", nil)
	w.lastResponse = resp
	return err
}

// fakeMediaPath builds a syntactically valid question-media URL under
// the scenario's quiz without needing a real question — the auth check
// in both media handlers runs before any question lookup, so a
// nonexistent question id doesn't affect what's being tested here.
func fakeMediaPath(w *World) string {
	return fmt.Sprintf("/admin/quizzes/%s/questions/%s/media", w.quizID, uuid.NewString())
}

func iTryToUploadMediaWithNoBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.uploadMediaAs(ctx, fakeMediaPath(w), "", "test.png", "image/png", []byte("x"))
	w.lastResponse = resp
	return err
}

func iTryToUploadMediaWithAnInvalidBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.uploadMediaAs(ctx, fakeMediaPath(w), invalidBearerToken, "test.png", "image/png", []byte("x"))
	w.lastResponse = resp
	return err
}

func iTryToUploadMediaAsAUserWithoutTheAdminRole(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.noRoleToken(ctx)
	if err != nil {
		return err
	}
	resp, err := w.uploadMediaAs(ctx, fakeMediaPath(w), token, "test.png", "image/png", []byte("x"))
	w.lastResponse = resp
	return err
}

// DeleteQuestionMedia checks Authorization itself too (see
// internal/question/handler_media.go), the same as UploadQuestionMedia
// above, but it's a separate code path with its own RequireAdminToken
// call — worth covering on its own rather than assuming upload's coverage
// extends to it.

func iTryToDeleteMediaWithNoBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.request(ctx, http.MethodDelete, fakeMediaPath(w), "", "", nil)
	w.lastResponse = resp
	return err
}

func iTryToDeleteMediaWithAnInvalidBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.request(ctx, http.MethodDelete, fakeMediaPath(w), invalidBearerToken, "", nil)
	w.lastResponse = resp
	return err
}

func iTryToDeleteMediaAsAUserWithoutTheAdminRole(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.noRoleToken(ctx)
	if err != nil {
		return err
	}
	resp, err := w.request(ctx, http.MethodDelete, fakeMediaPath(w), token, "", nil)
	w.lastResponse = resp
	return err
}
