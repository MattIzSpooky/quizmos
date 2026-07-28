package e2e

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

// registerAdminGameSocketSteps covers admin_game_socket.feature: the
// admin live-control page's own websocket, gated by a short-lived,
// single-use ticket (since browsers can't attach the Authorization header
// to a websocket handshake) rather than the player channel's client_id.
func registerAdminGameSocketSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I try to mint an admin websocket ticket with no bearer token$`, iTryToMintAnAdminWsTicketWithNoBearerToken)
	sc.Step(`^I try to mint an admin websocket ticket with an invalid bearer token$`, iTryToMintAnAdminWsTicketWithAnInvalidBearerToken)
	sc.Step(`^I try to mint an admin websocket ticket as a user without the admin role$`, iTryToMintAnAdminWsTicketAsAUserWithoutTheAdminRole)
	sc.Step(`^I try to mint an admin websocket ticket for an unknown game$`, iTryToMintAnAdminWsTicketForAnUnknownGame)
	sc.Step(`^the admin connects to the game control websocket$`, theAdminConnectsToTheGameControlWebsocket)
	sc.Step(`^the admin mints a websocket ticket$`, theAdminMintsAWebsocketTicket)
	sc.Step(`^the admin connects to the game control websocket using that ticket$`, theAdminConnectsUsingThatTicket)
	sc.Step(`^someone tries to connect to the admin game control websocket without a ticket$`, someoneTriesToConnectToAdminSocketWithoutATicket)
	sc.Step(`^someone tries to reuse that same ticket to connect$`, someoneTriesToReuseThatSameTicket)
	sc.Step(`^the admin should receive an? "([^"]*)" message$`, theAdminShouldReceiveAMessage)
	sc.Step(`^the admin's websocket connection should be closed$`, theAdminsWebsocketConnectionShouldBeClosed)
	sc.Step(`^a second admin tab connects to the game control websocket$`, aSecondAdminTabConnects)
	sc.Step(`^the second admin tab should receive an? "([^"]*)" message$`, theSecondAdminTabShouldReceiveAMessage)
}

func iTryToMintAnAdminWsTicketWithNoBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", w.gameID)
	resp, err := w.request(ctx, http.MethodPost, path, "", "", nil)
	w.lastResponse = resp
	return err
}

func iTryToMintAnAdminWsTicketWithAnInvalidBearerToken(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", w.gameID)
	resp, err := w.request(ctx, http.MethodPost, path, invalidBearerToken, "", nil)
	w.lastResponse = resp
	return err
}

func iTryToMintAnAdminWsTicketAsAUserWithoutTheAdminRole(ctx context.Context) error {
	w := worldFromContext(ctx)
	token, err := w.env.noRoleToken(ctx)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", w.gameID)
	resp, err := w.request(ctx, http.MethodPost, path, token, "", nil)
	w.lastResponse = resp
	return err
}

// iTryToMintAnAdminWsTicketForAnUnknownGame exercises CreateGameWsTicket's
// own not-found check — unlike the auth-rejection steps above, this uses
// a valid admin token against a game id that simply doesn't exist.
func iTryToMintAnAdminWsTicketForAnUnknownGame(ctx context.Context) error {
	w := worldFromContext(ctx)
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", uuid.NewString())
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	w.lastResponse = resp
	return err
}

func theAdminConnectsToTheGameControlWebsocket(ctx context.Context) error {
	return worldFromContext(ctx).connectAdminSocket(ctx)
}

func theAdminMintsAWebsocketTicket(ctx context.Context) error {
	w := worldFromContext(ctx)
	ticket, err := w.mintAdminWsTicket(ctx, w.gameID)
	if err != nil {
		return err
	}
	w.lastWSTicket = ticket
	return nil
}

func theAdminConnectsUsingThatTicket(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.lastWSTicket == "" {
		return fmt.Errorf("no admin websocket ticket has been minted yet")
	}
	return w.connectAdminSocketWithTicket(ctx, w.lastWSTicket)
}

func someoneTriesToConnectToAdminSocketWithoutATicket(ctx context.Context) error {
	w := worldFromContext(ctx)
	conn, status, err := dialAdminWebsocket(ctx, w, w.gameID, "")
	if err == nil {
		conn.CloseNow()
		return fmt.Errorf("expected the admin websocket handshake to be rejected, but it succeeded")
	}
	w.lastWSStatus = status
	return nil
}

// someoneTriesToReuseThatSameTicket dials the admin websocket again with
// whatever ticket last got a connection established (see
// theAdminConnectsUsingThatTicket) — tickets are single-use, so this
// second dial should be rejected even though the ticket itself was valid.
func someoneTriesToReuseThatSameTicket(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.lastWSTicket == "" {
		return fmt.Errorf("no admin websocket ticket has been minted yet")
	}
	conn, status, err := dialAdminWebsocket(ctx, w, w.gameID, w.lastWSTicket)
	if err == nil {
		conn.CloseNow()
		return fmt.Errorf("expected reusing the ticket to be rejected, but it succeeded")
	}
	w.lastWSStatus = status
	return nil
}

func theAdminShouldReceiveAMessage(ctx context.Context, msgType string) error {
	w := worldFromContext(ctx)
	if w.adminConn == nil {
		return fmt.Errorf("the admin hasn't connected to the game control websocket yet")
	}
	_, err := w.adminConn.waitFor(ctx, msgType, defaultWaitTimeout)
	return err
}

// theAdminsWebsocketConnectionShouldBeClosed mirrors
// websocketConnectionShouldBeClosed (players) for the admin's own
// connection — e.g. once the game ends, ws.Hub.CloseRoom should tear it
// down along with every player connection in the room.
func theAdminsWebsocketConnectionShouldBeClosed(ctx context.Context) error {
	w := worldFromContext(ctx)
	if w.adminConn == nil || w.adminConn.closed == nil {
		return fmt.Errorf("the admin was never connected to the game control websocket")
	}
	select {
	case <-w.adminConn.closed:
		return nil
	case <-time.After(defaultWaitTimeout):
		return fmt.Errorf("expected the admin's websocket connection to be closed, but it's still open")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func aSecondAdminTabConnects(ctx context.Context) error {
	return worldFromContext(ctx).connectSecondAdminSocket(ctx)
}

func theSecondAdminTabShouldReceiveAMessage(ctx context.Context, msgType string) error {
	w := worldFromContext(ctx)
	if w.secondAdminConn == nil {
		return fmt.Errorf("the second admin tab hasn't connected to the game control websocket yet")
	}
	_, err := w.secondAdminConn.waitFor(ctx, msgType, defaultWaitTimeout)
	return err
}
