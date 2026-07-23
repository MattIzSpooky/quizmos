package e2e

import (
	"context"
	"encoding/json"
	"fmt"
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

	lastResponse apiResponse
	lastErr      error
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

	go func() {
		for {
			var env wsEnvelope
			readCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			err := wsjson.Read(readCtx, conn, &env)
			cancel()
			if err != nil {
				return
			}
			p.recordMessage(env)
		}
	}()
	return nil
}

func (w *World) closeAllSockets() {
	for _, p := range w.players {
		if p.conn != nil {
			_ = p.conn.CloseNow()
		}
	}
}
