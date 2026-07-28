package e2e

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// registerGameControlSteps covers the admin actions (start / advance /
// end) that drive a game through its lifecycle — shared across
// game_lifecycle.feature and nearly every other gameplay feature's
// Background.
func registerGameControlSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the admin starts the game$`, theAdminStartsTheGame)
	sc.Step(`^starting the game should fail with status (\d+)$`, startingTheGameShouldFailWithStatus)
	sc.Step(`^the admin advances to the next question$`, theAdminAdvancesToTheNextQuestion)
	sc.Step(`^the admin ends the game$`, theAdminEndsTheGame)
}

func theAdminStartsTheGame(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/start", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 starting game, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}

func startingTheGameShouldFailWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/start", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d starting game, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

func theAdminAdvancesToTheNextQuestion(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/next-question", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 advancing game, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}

// currentQuestionIndex fetches the game's live current-question index
// (0-based) from the admin API, so steps can compute "one before that"
// without the World tracking it separately.
func currentQuestionIndex(ctx context.Context, w *World) (int, error) {
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	idx, ok := resp.Body["currentQuestionIndex"].(float64)
	if !ok {
		return 0, fmt.Errorf("game has no current question index (status: %v)", resp.Body["status"])
	}
	return int(idx), nil
}

func theAdminEndsTheGame(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/end", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 ending game, got %d: %v", resp.Status, resp.Body)
	}
	return nil
}
