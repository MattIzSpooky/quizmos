package e2e

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"

	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// registerQuestionNavigationSteps covers question_navigation.feature:
// re-showing an earlier question read-only, and resuming live play by
// reviewing the still-current one.
func registerQuestionNavigationSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the admin goes back to the previous question$`, theAdminGoesBackToThePreviousQuestion)
	sc.Step(`^going back should fail with status (\d+)$`, goingBackShouldFailWithStatus)
	sc.Step(`^the admin reviews question (\d+)$`, theAdminReviewsQuestionN)
	sc.Step(`^reviewing question (\d+) should fail with status (\d+)$`, reviewingQuestionNShouldFailWithStatus)
}

func theAdminGoesBackToThePreviousQuestion(ctx context.Context) error {
	w := worldFromContext(ctx)
	current, err := currentQuestionIndex(ctx, w)
	if err != nil {
		return err
	}
	return reviewQuestionAtIndex(ctx, w, current-1, http.StatusOK)
}

func goingBackShouldFailWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	current, err := currentQuestionIndex(ctx, w)
	if err != nil {
		return err
	}
	return reviewQuestionAtIndex(ctx, w, current-1, want)
}

// theAdminReviewsQuestionN uses 1-based question numbers to match how
// they're referred to in feature files ("question 1", "question 2", ...).
func theAdminReviewsQuestionN(ctx context.Context, n int) error {
	w := worldFromContext(ctx)
	return reviewQuestionAtIndex(ctx, w, n-1, http.StatusOK)
}

func reviewingQuestionNShouldFailWithStatus(ctx context.Context, n, want int) error {
	w := worldFromContext(ctx)
	return reviewQuestionAtIndex(ctx, w, n-1, want)
}

func reviewQuestionAtIndex(ctx context.Context, w *World, index, wantStatus int) error {
	// Reviewing the still-current question resumes live play by
	// redelivering question.started (see backend ReviewQuestion's IsLive
	// case) — on top of whatever question.started backlog already
	// exists (the question actually starting, any earlier review-then-
	// resume). Fast-forward every player past that backlog so a later
	// assertion catches the fresh resend, not stale history.
	for _, p := range w.players {
		p.catchUp(ws.TypeQuestionStarted)
	}
	path := fmt.Sprintf("/admin/games/%s/review-question", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, map[string]any{"questionIndex": index})
	if err != nil {
		return err
	}
	if resp.Status != wantStatus {
		return fmt.Errorf("expected status %d reviewing question index %d, got %d: %v", wantStatus, index, resp.Status, resp.Body)
	}
	return nil
}
