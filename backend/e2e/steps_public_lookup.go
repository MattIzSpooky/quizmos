package e2e

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// registerPublicLookupSteps covers public_lookup.feature: GET
// /games/{code} and /games/{code}/leaderboard are unauthenticated — what
// a player's join screen calls to show a quiz name before they commit to
// joining, or to poll standings without a websocket.
func registerPublicLookupSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the public game lookup should show quiz "([^"]*)" and status "([^"]*)"$`, thePublicGameLookupShouldShow)
	sc.Step(`^the public game lookup for code "([^"]*)" should fail with status (\d+)$`, thePublicGameLookupForCodeShouldFail)
	sc.Step(`^the public leaderboard should show "([^"]*)" with score (\d+)$`, thePublicLeaderboardShouldShow)
}

func thePublicGameLookupShouldShow(ctx context.Context, wantTitle, wantStatus string) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/games/%s", w.gameCode)
	resp, err := w.publicRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 looking up game %q, got %d: %v", w.gameCode, resp.Status, resp.Body)
	}
	if got, _ := resp.Body["quizTitle"].(string); got != wantTitle {
		return fmt.Errorf("expected quizTitle %q, got %q", wantTitle, got)
	}
	if got, _ := resp.Body["status"].(string); got != wantStatus {
		return fmt.Errorf("expected status %q, got %q", wantStatus, got)
	}
	return nil
}

func thePublicGameLookupForCodeShouldFail(ctx context.Context, code string, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/games/%s", code)
	resp, err := w.publicRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d looking up code %q, got %d: %v", want, code, resp.Status, resp.Body)
	}
	return nil
}

func thePublicLeaderboardShouldShow(ctx context.Context, nickname string, wantScore int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/games/%s/leaderboard", w.gameCode)
	resp, err := w.publicRequest(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 fetching public leaderboard, got %d: %v", resp.Status, resp.Body)
	}
	entries, _ := resp.Body["entries"].([]any)
	for _, raw := range entries {
		entry := raw.(map[string]any)
		if entry["nickname"] == nickname {
			got := int(entry["score"].(float64))
			if got != wantScore {
				return fmt.Errorf("expected %q to have score %d, got %d", nickname, wantScore, got)
			}
			return nil
		}
	}
	return fmt.Errorf("no public leaderboard entry for %q", nickname)
}
