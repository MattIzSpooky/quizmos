package e2e

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

// registerGameLifecycleSteps covers game_lifecycle.feature: creating a
// game, players joining/kicking/rejoining, and the admin's view of who's
// connected.
func registerGameLifecycleSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I create a game for the quiz$`, iCreateAGameForTheQuiz)
	sc.Step(`^I try to create a game for an unknown quiz$`, iTryToCreateAGameForAnUnknownQuiz)
	sc.Step(`^"([^"]*)" joins the game$`, joinsTheGame)
	sc.Step(`^"([^"]*)" rejoins the game$`, rejoinsTheGame)
	sc.Step(`^"([^"]*)" tries to join game code "([^"]*)"$`, triesToJoinGameCode)
	sc.Step(`^the request should succeed$`, theRequestShouldSucceed)
	sc.Step(`^the request should fail with status (\d+)$`, theRequestShouldFailWithStatus)
	sc.Step(`^the game should have (\d+) players?$`, theGameShouldHaveNPlayers)
	sc.Step(`^the admin kicks "([^"]*)"$`, theAdminKicks)
	sc.Step(`^kicking "([^"]*)" should fail with status (\d+)$`, kickingShouldFailWithStatus)
	sc.Step(`^kicking a player who never joined should fail with status (\d+)$`, kickingANonexistentPlayerShouldFail)
	sc.Step(`^the admin should see "([^"]*)" as (connected|not connected)$`, theAdminShouldSeeAsConnected)
}

// iCreateAGameForTheQuiz deliberately does not assert on the response
// status — some scenarios expect this to fail (e.g. a quiz with no
// questions). Use "the request should succeed/fail with status N" to
// assert, same as the join steps.
func iCreateAGameForTheQuiz(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodPost, "/admin/games", map[string]any{"quizId": w.quizID})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status == http.StatusCreated {
		w.gameID = resp.Body["id"].(string)
		w.gameCode = resp.Body["code"].(string)
	}
	return nil
}

func iTryToCreateAGameForAnUnknownQuiz(ctx context.Context) error {
	w := worldFromContext(ctx)
	resp, err := w.adminRequest(ctx, http.MethodPost, "/admin/games", map[string]any{"quizId": uuid.NewString()})
	w.lastResponse = resp
	return err
}

func joinsTheGame(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	return doJoin(ctx, w, nickname, w.gameCode)
}

func triesToJoinGameCode(ctx context.Context, nickname, code string) error {
	w := worldFromContext(ctx)
	return doJoin(ctx, w, nickname, code)
}

func doJoin(ctx context.Context, w *World, nickname, code string) error {
	return doJoinAs(ctx, w, nickname, code, w.newClientID())
}

// rejoinsTheGame reuses the same client_id as an earlier join — unlike
// joinsTheGame, which always mints a fresh one — to verify that a kicked
// player can really come back as themselves, not just as a new stranger.
func rejoinsTheGame(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	existing, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q was never registered in this scenario", nickname)
	}
	return doJoinAs(ctx, w, nickname, w.gameCode, existing.clientID)
}

func doJoinAs(ctx context.Context, w *World, nickname, code, clientID string) error {
	resp, err := w.publicRequest(ctx, http.MethodPost, "/games/join", clientID, map[string]any{
		"code":     code,
		"nickname": nickname,
	})
	w.lastResponse = resp
	if err != nil {
		return err
	}
	w.players[nickname] = newPlayer(nickname, clientID)
	return nil
}

func theRequestShouldSucceed(ctx context.Context) error {
	status := worldFromContext(ctx).lastResponse.Status
	if status < 200 || status >= 300 {
		return fmt.Errorf("expected a 2xx response, got %d", status)
	}
	return nil
}

func theRequestShouldFailWithStatus(ctx context.Context, want int) error {
	got := worldFromContext(ctx).lastResponse.Status
	if got != want {
		return fmt.Errorf("expected status %d, got %d", want, got)
	}
	return nil
}

func theGameShouldHaveNPlayers(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	got := int(resp.Body["playerCount"].(float64))
	if got != want {
		return fmt.Errorf("expected %d players, got %d", want, got)
	}
	return nil
}

func theAdminKicks(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	return kickPlayer(ctx, w, nickname, http.StatusNoContent)
}

func kickingShouldFailWithStatus(ctx context.Context, nickname string, want int) error {
	w := worldFromContext(ctx)
	return kickPlayer(ctx, w, nickname, want)
}

func kickPlayer(ctx context.Context, w *World, nickname string, wantStatus int) error {
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	path := fmt.Sprintf("/admin/games/%s/players/%s", w.gameID, p.clientID)
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != wantStatus {
		return fmt.Errorf("expected status %d kicking %q, got %d: %v", wantStatus, nickname, resp.Status, resp.Body)
	}
	return nil
}

// kickingANonexistentPlayerShouldFail targets a client_id that never
// joined this game at all (as opposed to kickingShouldFailWithStatus,
// which targets a real player under the wrong game state).
func kickingANonexistentPlayerShouldFail(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/players/%s", w.gameID, uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodDelete, path, nil)
	w.lastResponse = resp
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("expected status %d kicking a nonexistent player, got %d: %v", want, resp.Status, resp.Body)
	}
	return nil
}

// theAdminShouldSeeAsConnected checks GetGame's per-player "connected"
// field (backed by ws.Hub.ConnectedClientIDs) — separate from whether the
// player has joined at all (playerCount), this is specifically whether
// their websocket is currently open.
func theAdminShouldSeeAsConnected(ctx context.Context, nickname, wantWord string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	want := wantWord == "connected"
	path := fmt.Sprintf("/admin/games/%s", w.gameID)
	resp, err := w.adminRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	for _, raw := range resp.Body["players"].([]any) {
		player := raw.(map[string]any)
		if player["clientId"] == p.clientID {
			got, _ := player["connected"].(bool)
			if got != want {
				return fmt.Errorf("expected connected=%v for %q, got %v", want, nickname, got)
			}
			return nil
		}
	}
	return fmt.Errorf("no admin roster entry for %q", nickname)
}
