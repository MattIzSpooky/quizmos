package e2e

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// registerResetAnswersSteps covers reset_answers.feature: wiping every
// player's answer to an already-asked question (and reversing its
// points) so it can be answered again.
func registerResetAnswersSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the admin resets the answers for question (\d+)$`, theAdminResetsAnswersForQuestionN)
	sc.Step(`^resetting answers for question (\d+) should fail with status (\d+)$`, resettingAnswersForQuestionNShouldFailWithStatus)
}

// theAdminResetsAnswersForQuestionN uses 1-based question numbers to match
// how they're referred to in feature files ("question 1", "question 2", ...).
func theAdminResetsAnswersForQuestionN(ctx context.Context, n int) error {
	w := worldFromContext(ctx)
	return resetAnswersAtIndex(ctx, w, n-1, http.StatusOK)
}

func resettingAnswersForQuestionNShouldFailWithStatus(ctx context.Context, n, want int) error {
	w := worldFromContext(ctx)
	return resetAnswersAtIndex(ctx, w, n-1, want)
}

func resetAnswersAtIndex(ctx context.Context, w *World, index, wantStatus int) error {
	path := fmt.Sprintf("/admin/games/%s/reset-answers", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, map[string]any{"questionIndex": index})
	if err != nil {
		return err
	}
	if resp.Status != wantStatus {
		return fmt.Errorf("expected status %d resetting answers for question index %d, got %d: %v", wantStatus, index, resp.Status, resp.Body)
	}
	return nil
}
