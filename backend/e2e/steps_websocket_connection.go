package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// registerWebsocketConnectionSteps covers websocket_auth.feature (the
// player channel should refuse anyone who hasn't actually joined) plus
// the connect/disconnect lifecycle steps other gameplay features reuse.
func registerWebsocketConnectionSteps(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]*)" connects to the game websocket$`, connectsToTheGameWebsocket)
	sc.Step(`^"([^"]*)" reconnects to the game websocket$`, connectsToTheGameWebsocket)
	sc.Step(`^"([^"]*)" disconnects$`, disconnects)
	sc.Step(`^"([^"]*)"'s websocket connection should be closed$`, websocketConnectionShouldBeClosed)
	sc.Step(`^"([^"]*)" tries to connect to the game websocket without joining$`, triesToConnectWithoutJoining)
	sc.Step(`^someone tries to connect to the game websocket with client id "([^"]*)"$`, someoneTriesToConnectWithClientID)
	sc.Step(`^the websocket connection should be rejected with status (\d+)$`, theWebsocketConnectionShouldBeRejectedWithStatus)
}

func connectsToTheGameWebsocket(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	// A reconnect's catch-up can resend question.started (see hub
	// sendCatchUp) on top of whatever backlog already exists from this
	// question actually starting — fast-forward past that backlog so a
	// later assertion catches the fresh catch-up send, not stale history.
	p.catchUp(ws.TypeQuestionStarted)
	return w.connectPlayerSocket(ctx, p)
}

func disconnects(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if p.conn == nil {
		return fmt.Errorf("player %q isn't connected", nickname)
	}
	return p.conn.CloseNow()
}

// websocketConnectionShouldBeClosed waits for p.closed (see world.go) to
// close — proof the connection was actually torn down server-side (e.g.
// by Hub.CloseRoom after its game's quiz was deleted), not just that some
// message arrived beforehand.
func websocketConnectionShouldBeClosed(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	p, ok := w.players[nickname]
	if !ok {
		return fmt.Errorf("player %q hasn't joined the game yet", nickname)
	}
	if p.closed == nil {
		return fmt.Errorf("player %q was never connected", nickname)
	}
	select {
	case <-p.closed:
		return nil
	case <-time.After(defaultWaitTimeout):
		return fmt.Errorf("expected %q's websocket connection to be closed, but it's still open", nickname)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// triesToConnectWithoutJoining attempts the websocket handshake with a
// freshly-minted client_id that was never used to join the game via POST
// /games/join — Hub.Upgrade should reject it (no matching player row)
// before ever completing the upgrade.
func triesToConnectWithoutJoining(ctx context.Context, nickname string) error {
	w := worldFromContext(ctx)
	clientID := w.newClientID()
	w.players[nickname] = newPlayer(nickname, clientID)
	return attemptWebsocketDial(ctx, w, w.gameCode, clientID)
}

// someoneTriesToConnectWithClientID attempts the handshake with a raw,
// possibly-malformed client_id string — Hub.Upgrade should reject
// anything that doesn't parse as a UUID before looking up a player at
// all.
func someoneTriesToConnectWithClientID(ctx context.Context, clientID string) error {
	w := worldFromContext(ctx)
	return attemptWebsocketDial(ctx, w, w.gameCode, clientID)
}

func attemptWebsocketDial(ctx context.Context, w *World, gameCode, clientID string) error {
	conn, status, err := dialGameWebsocket(ctx, w, gameCode, clientID)
	if err == nil {
		conn.CloseNow()
		return fmt.Errorf("expected the websocket handshake to be rejected, but it succeeded")
	}
	w.lastWSStatus = status
	return nil
}

func theWebsocketConnectionShouldBeRejectedWithStatus(ctx context.Context, want int) error {
	w := worldFromContext(ctx)
	if w.lastWSStatus != want {
		return fmt.Errorf("expected websocket rejection status %d, got %d", want, w.lastWSStatus)
	}
	return nil
}
