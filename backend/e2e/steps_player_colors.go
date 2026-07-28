package e2e

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// registerPlayerColorSteps covers player_colors.feature: choosing a
// cosmetic color on join, falling back to a default for an unrecognized
// one, and that color showing up wherever a player is displayed.
func registerPlayerColorSteps(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]*)" joins the game with color "([^"]*)"$`, joinsTheGameWithColor)
	sc.Step(`^"([^"]*)" should be shown to the admin with color "([^"]*)"$`, shouldBeShownToTheAdminWithColor)
	sc.Step(`^"([^"]*)" joins the game again with color "([^"]*)"$`, rejoinsTheGameWithColor)
	sc.Step(`^the leaderboard should show "([^"]*)" with color "([^"]*)"$`, theLeaderboardShouldShowWithColor)
}

// joinsTheGameWithColor covers both a recognized color and an
// unrecognized one (the latter should fall back to the default rather
// than fail the join — color is cosmetic, never worth rejecting a
// request over).
func joinsTheGameWithColor(ctx context.Context, nickname, color string) error {
	w := worldFromContext(ctx)
	clientID := w.newClientID()
	resp, err := w.publicRequest(ctx, http.MethodPost, "/games/join", clientID, map[string]any{
		"code":     w.gameCode,
		"nickname": nickname,
		"color":    color,
	})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 joining as %q, got %d: %v", nickname, resp.Status, resp.Body)
	}
	w.players[nickname] = newPlayer(nickname, clientID)
	return nil
}

func shouldBeShownToTheAdminWithColor(ctx context.Context, nickname, wantColor string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	for _, raw := range resp.Body["players"].([]any) {
		player := raw.(map[string]any)
		if player["clientId"] == p.clientID {
			if got := player["color"]; got != wantColor {
				return fmt.Errorf("expected color %q for %q, got %v", wantColor, nickname, got)
			}
			return nil
		}
	}
	return fmt.Errorf("no admin roster entry for %q", nickname)
}

// rejoinsTheGameWithColor is rejoinsTheGame plus a color, to check that
// UpsertPlayer's ON CONFLICT clause updates color the same way it
// already updates nickname.
func rejoinsTheGameWithColor(ctx context.Context, nickname, color string) error {
	w := worldFromContext(ctx)
	existing, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q was never registered in this scenario", nickname)
	}
	resp, err := w.publicRequest(ctx, http.MethodPost, "/games/join", existing.clientID, map[string]any{
		"code":     w.gameCode,
		"nickname": nickname,
		"color":    color,
	})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("expected 200 rejoining as %q, got %d: %v", nickname, resp.Status, resp.Body)
	}
	return nil
}

func theLeaderboardShouldShowWithColor(ctx context.Context, nickname, wantColor string) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/leaderboard", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	entries, _ := resp.Body["entries"].([]any)
	for _, raw := range entries {
		entry := raw.(map[string]any)
		if entry["nickname"] == nickname {
			if got := entry["color"]; got != wantColor {
				return fmt.Errorf("expected color %q for %q, got %v", wantColor, nickname, got)
			}
			return nil
		}
	}
	return fmt.Errorf("no leaderboard entry for %q", nickname)
}
