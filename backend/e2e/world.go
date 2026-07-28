package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

// wsEnvelope mirrors the hand-written {type, payload} wire format from
// internal/ws/envelope.go — duplicated here deliberately, the same way a
// real API consumer would define its own copy rather than importing the
// server's internal package.
type wsEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// player is one scenario's simulated client: an anonymous client_id plus,
// once connected, a live websocket and a running log of every message
// it has received (so steps can assert on messages that already arrived
// as well as wait for ones still in flight).
type player struct {
	nickname string
	clientID string

	conn *websocket.Conn
	// closed is a fresh channel per connect, closed by the read loop when
	// it returns (i.e. the connection has actually ended, whether the
	// client or the server closed it) — lets a step assert a connection
	// was really torn down server-side (e.g. after deleting its game's
	// quiz) rather than just that some message arrived.
	closed chan struct{}

	mu       sync.Mutex
	messages []wsEnvelope
	consumed map[string]int // message type -> how many occurrences waitFor has already returned

	// currentQuestion tracks the last question.started payload this
	// player received, so an "answers" step can resolve an option by
	// its human-readable text without the scenario tracking UUIDs.
	currentQuestion *questionStartedPayload
}

func newPlayer(nickname, clientID string) *player {
	return &player{nickname: nickname, clientID: clientID, consumed: make(map[string]int)}
}

type questionStartedPayload struct {
	QuestionID string `json:"questionId"`
	Options    []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"options"`
}

func (p *player) recordMessage(env wsEnvelope) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = append(p.messages, env)
	if env.Type == "question.started" {
		var qs questionStartedPayload
		if err := json.Unmarshal(env.Payload, &qs); err == nil {
			p.currentQuestion = &qs
		}
	}
}

// catchUp marks every occurrence of msgType received so far as consumed,
// without returning any of them, so a later waitFor(msgType) only matches
// occurrences that arrive after this call. Use it right before triggering
// an action expected to (re)send a message of a type the player has
// already received unconsumed backlog of — e.g. resuming live play
// redelivers question.started, but so did the question actually
// starting and any earlier review; without fast-forwarding past that
// backlog, waitFor would return a stale occurrence instead of the fresh
// one the action just caused.
func (p *player) catchUp(msgType string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, m := range p.messages {
		if m.Type == msgType {
			count++
		}
	}
	p.consumed[msgType] = count
}

// waitFor blocks until the next not-yet-consumed occurrence of msgType
// arrives (i.e. repeated calls advance through the log in order, rather
// than all matching the first occurrence ever seen) — important once a
// player receives more than one message of the same type, e.g. a
// question.started per question.
func (p *player) waitFor(ctx context.Context, msgType string, timeout time.Duration) (wsEnvelope, error) {
	deadline := time.Now().Add(timeout)
	for {
		p.mu.Lock()
		seen := 0
		for _, m := range p.messages {
			if m.Type != msgType {
				continue
			}
			seen++
			if seen > p.consumed[msgType] {
				p.consumed[msgType] = seen
				p.mu.Unlock()
				return m, nil
			}
		}
		p.mu.Unlock()

		if time.Now().After(deadline) {
			return wsEnvelope{}, fmt.Errorf("timed out waiting for %q message for player %q", msgType, p.nickname)
		}
		select {
		case <-ctx.Done():
			return wsEnvelope{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// waitForCurrentQuestion blocks until this player's read loop has recorded
// at least one question.started message (populating currentQuestion), but
// — unlike waitFor — doesn't consume a tracked "occurrence" of the
// message type. Steps that submit an answer only need currentQuestion to
// be populated to resolve an option or grab the live questionId; they
// don't care which question.started message that was, and consuming one
// via waitFor would throw off a later step's count of how many
// question.started messages have actually been asserted on for this
// player (e.g. after resuming live play redelivers one).
func (p *player) waitForCurrentQuestion(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		p.mu.Lock()
		ready := p.currentQuestion != nil
		p.mu.Unlock()
		if ready {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for a question.started message for player %q", p.nickname)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// optionIDFor resolves an option's UUID from the text shown in the last
// question.started message this player received.
func (p *player) optionIDFor(text string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.currentQuestion == nil {
		return "", fmt.Errorf("player %q hasn't received a question.started message yet", p.nickname)
	}
	for _, o := range p.currentQuestion.Options {
		if o.Text == text {
			return o.ID, nil
		}
	}
	return "", fmt.Errorf("no option with text %q in the current question", text)
}

// questionOption is a (question, option) pair discovered while creating
// questions, so "the correct answer for ... is ..." style steps can
// resolve names to UUIDs against the admin-side record too.
type questionRecord struct {
	id      string
	options map[string]string // option text -> option id
}

// World holds all state for one scenario. A fresh World is created per
// scenario (see main_test.go); the environment (containers, DB, server)
// is shared and reset via truncateAll.
type World struct {
	env *environment

	adminToken string

	quizID    string
	questions map[string]questionRecord // question prompt -> record

	gameID   string
	gameCode string

	players map[string]*player // nickname -> player

	// adminConn is the admin live-control page's own websocket connection,
	// if the scenario has connected one — reuses the player type purely
	// as a message log/waitFor helper (see connectAdminSocket); an admin
	// connection has no client_id or nickname of its own, but nothing in
	// player's log/wait machinery actually depends on either.
	adminConn *player
	// secondAdminConn is a second, independent admin connection to the
	// same game — for scenarios proving that a second open admin tab
	// stays in sync (e.g. free-text grading), not just the one that
	// happened to trigger the mutation.
	secondAdminConn *player
	// lastWSTicket is the most recently minted admin websocket ticket, for
	// scenarios that mint one and then use (or reuse) it explicitly rather
	// than connecting immediately (see mintAdminWsTicket).
	lastWSTicket string

	// lastMediaURL is the public URL of the most recently uploaded
	// question media, kept around so a later step can confirm it's been
	// deleted from storage (e.g. after the owning quiz is deleted) —
	// by then the question itself is gone, so there's no API left to
	// look the URL back up through.
	lastMediaURL string

	lastResponse apiResponse
	lastErr      error

	// lastWSStatus is the HTTP status from the most recent rejected
	// websocket handshake attempt (see dialGameWebsocket) — there's no
	// apiResponse-shaped body to reuse lastResponse for, since the
	// rejection happens before any upgrade, let alone JSON.
	lastWSStatus int
}

func newWorld(env *environment) *World {
	return &World{
		env:       env,
		questions: make(map[string]questionRecord),
		players:   make(map[string]*player),
	}
}

func (w *World) newClientID() string {
	return uuid.NewString()
}

func (w *World) connectPlayerSocket(ctx context.Context, p *player) error {
	url := fmt.Sprintf("%s/games/%s?client_id=%s", w.env.wsBaseURL, w.gameCode, p.clientID)
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}
	p.conn = conn
	startRecording(p)
	return nil
}

// startRecording starts p's read loop against its already-established
// p.conn, closing p.closed once the loop returns (the connection ended,
// whichever side closed it) — shared by every kind of websocket
// connection a scenario opens (a real player or an admin's game-control
// connection), since the log/wait machinery in *player is identical
// either way.
func startRecording(p *player) {
	p.closed = make(chan struct{})
	go func() {
		defer close(p.closed)
		for {
			var env wsEnvelope
			readCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			err := wsjson.Read(readCtx, p.conn, &env)
			cancel()
			if err != nil {
				return
			}
			p.recordMessage(env)
		}
	}()
}

// dialGameWebsocket attempts the websocket handshake for gameCode/clientID
// without registering a player — for asserting Hub.Upgrade's rejection
// paths (never joined, malformed client_id), where a successful dial
// would be the test failure. status is the HTTP status the server
// responded with when the handshake didn't succeed (0 if no response was
// received at all, e.g. a connection-level failure).
func dialGameWebsocket(ctx context.Context, w *World, gameCode, clientID string) (conn *websocket.Conn, status int, err error) {
	url := fmt.Sprintf("%s/games/%s?client_id=%s", w.env.wsBaseURL, gameCode, clientID)
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil {
		status = resp.StatusCode
	}
	return conn, status, err
}

func (w *World) closeAllSockets() {
	for _, p := range w.players {
		if p.conn != nil {
			_ = p.conn.CloseNow()
		}
	}
	if w.adminConn != nil && w.adminConn.conn != nil {
		_ = w.adminConn.conn.CloseNow()
	}
	if w.secondAdminConn != nil && w.secondAdminConn.conn != nil {
		_ = w.secondAdminConn.conn.CloseNow()
	}
}

// mintAdminWsTicket calls POST /admin/games/{gameId}/ws-ticket with the
// scenario's admin token — the same request the real admin frontend makes
// right before opening the game-control websocket.
func (w *World) mintAdminWsTicket(ctx context.Context, gameID string) (string, error) {
	path := fmt.Sprintf("/admin/games/%s/ws-ticket", gameID)
	resp, err := w.adminRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	if resp.Status != http.StatusOK {
		return "", fmt.Errorf("expected 200 minting an admin ws ticket, got %d: %v", resp.Status, resp.Body)
	}
	ticket, _ := resp.Body["ticket"].(string)
	if ticket == "" {
		return "", fmt.Errorf("ws-ticket response missing a ticket: %v", resp.Body)
	}
	return ticket, nil
}

// connectAdminSocket mints a fresh ticket and connects the admin
// game-control websocket with it, recording the result on w.adminConn.
func (w *World) connectAdminSocket(ctx context.Context) error {
	ticket, err := w.mintAdminWsTicket(ctx, w.gameID)
	if err != nil {
		return err
	}
	return w.connectAdminSocketWithTicket(ctx, ticket)
}

func (w *World) connectAdminSocketWithTicket(ctx context.Context, ticket string) error {
	conn, _, err := dialAdminWebsocket(ctx, w, w.gameID, ticket)
	if err != nil {
		return fmt.Errorf("dial admin websocket: %w", err)
	}
	w.adminConn = newConnectedAdminPlayer(conn)
	return nil
}

// connectSecondAdminSocket mints its own fresh ticket, independent of
// w.adminConn's, and connects a second admin game-control websocket —
// for scenarios proving a second open admin tab stays in sync too, not
// just the one that happened to trigger a mutation.
func (w *World) connectSecondAdminSocket(ctx context.Context) error {
	ticket, err := w.mintAdminWsTicket(ctx, w.gameID)
	if err != nil {
		return err
	}
	conn, _, err := dialAdminWebsocket(ctx, w, w.gameID, ticket)
	if err != nil {
		return fmt.Errorf("dial second admin websocket: %w", err)
	}
	w.secondAdminConn = newConnectedAdminPlayer(conn)
	return nil
}

// newConnectedAdminPlayer wraps an already-established admin websocket
// connection in a *player (used purely as a message log/waitFor helper —
// see the World.adminConn field doc) and starts recording its messages.
func newConnectedAdminPlayer(conn *websocket.Conn) *player {
	p := newPlayer("admin", "")
	p.conn = conn
	startRecording(p)
	return p
}

// dialAdminWebsocket attempts the admin game-control websocket handshake
// for gameID/ticket without registering a connection — for asserting
// UpgradeAdmin's rejection paths (missing/invalid/already-used ticket),
// where a successful dial would be the test failure. status is the HTTP
// status the server responded with when the handshake didn't succeed (0
// if no response was received at all).
func dialAdminWebsocket(ctx context.Context, w *World, gameID, ticket string) (conn *websocket.Conn, status int, err error) {
	url := fmt.Sprintf("%s/admin/games/%s?ticket=%s", w.env.wsBaseURL, gameID, ticket)
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil {
		status = resp.StatusCode
	}
	return conn, status, err
}
