// Package ws implements the live-gameplay websocket channel described by
// api/asyncapi.yaml: one hub keyed by game ID, one room per game, one
// client per connected player. REST admin handlers call the Broadcast*
// methods after mutating game state; the room never mutates state itself
// except via Service.SubmitAnswer on answer.submit.
package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/mattizspooky/quizmos/backend/internal/middleware"
	"github.com/mattizspooky/quizmos/backend/internal/service"
)

type client struct {
	clientID uuid.UUID
	nickname string
	conn     *websocket.Conn
	send     chan Envelope
}

type room struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]*client
}

type Hub struct {
	svc *service.Service

	mu    sync.RWMutex
	rooms map[uuid.UUID]*room

	AllowedOrigins []string
}

func NewHub(svc *service.Service, allowedOrigins []string) *Hub {
	return &Hub{svc: svc, rooms: make(map[uuid.UUID]*room), AllowedOrigins: allowedOrigins}
}

func (h *Hub) roomFor(gameID uuid.UUID) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[gameID]
	if !ok {
		r = &room{clients: make(map[uuid.UUID]*client)}
		h.rooms[gameID] = r
	}
	return r
}

// Upgrade handles GET /ws/games/{code}?client_id={uuid}. The player must
// already exist (created via POST /api/games/join) — the websocket never
// creates player identity itself.
func (h *Hub) Upgrade(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	clientIDStr, _ := middleware.ClientIDFromContext(r.Context())
	clientID, err := uuid.Parse(clientIDStr)
	if err != nil {
		http.Error(w, "missing or invalid client_id", http.StatusBadRequest)
		return
	}

	game, player, err := h.svc.GetPlayerByCode(r.Context(), code, clientID)
	if err != nil {
		http.Error(w, "no such player in this game; join via POST /api/games/join first", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.AllowedOrigins})
	if err != nil {
		return
	}

	c := &client{clientID: clientID, nickname: player.Nickname, conn: conn, send: make(chan Envelope, 16)}
	rm := h.roomFor(game.ID)

	rm.mu.Lock()
	rm.clients[clientID] = c
	count := len(rm.clients)
	rm.mu.Unlock()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go c.writeLoop(ctx)

	h.broadcastToRoom(rm, TypePresencePlayerJoined, PresencePlayerJoined{
		Player:      PlayerSummary{ClientID: clientID.String(), Nickname: player.Nickname},
		PlayerCount: int64(count),
	})

	h.readLoop(ctx, game.ID, c)

	rm.mu.Lock()
	delete(rm.clients, clientID)
	remaining := len(rm.clients)
	rm.mu.Unlock()
	close(c.send)

	h.broadcastToRoom(rm, TypePresencePlayerLeft, PresencePlayerLeft{
		ClientID:    clientID.String(),
		PlayerCount: int64(remaining),
	})
}

func (c *client) writeLoop(ctx context.Context) {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := wsjson.Write(writeCtx, c.conn, env)
			cancel()
			if err != nil {
				return
			}
		case <-pingTicker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (h *Hub) readLoop(ctx context.Context, gameID uuid.UUID, c *client) {
	defer c.conn.CloseNow()
	for {
		var env Envelope
		if err := wsjson.Read(ctx, c.conn, &env); err != nil {
			return
		}
		h.handleMessage(ctx, gameID, c, env)
	}
}

func (h *Hub) handleMessage(ctx context.Context, gameID uuid.UUID, c *client, env Envelope) {
	switch env.Type {
	case TypeAnswerSubmit:
		var submit AnswerSubmit
		if err := json.Unmarshal(env.Payload, &submit); err != nil {
			h.sendError(c, "bad_request", "malformed answer.submit payload")
			return
		}
		h.handleAnswerSubmit(ctx, gameID, c, submit)
	default:
		h.sendError(c, "unknown_message_type", "unrecognized message type: "+env.Type)
	}
}

func (h *Hub) handleAnswerSubmit(ctx context.Context, gameID uuid.UUID, c *client, submit AnswerSubmit) {
	questionID, err1 := uuid.Parse(submit.QuestionID)
	optionID, err2 := uuid.Parse(submit.OptionID)
	if err1 != nil || err2 != nil {
		h.sendError(c, "bad_request", "questionId/optionId must be UUIDs")
		return
	}

	result, err := h.svc.SubmitAnswer(ctx, gameID, c.clientID, questionID, optionID)
	if err != nil {
		h.sendError(c, "answer_rejected", err.Error())
		return
	}

	h.sendTo(c, TypeAnswerResult, AnswerResult{
		QuestionID:    submit.QuestionID,
		Correct:       result.Correct,
		PointsAwarded: int64(result.PointsAwarded),
		TotalScore:    int64(result.TotalScore),
	})
}

func (h *Hub) sendError(c *client, code, message string) {
	h.sendTo(c, TypeError, ErrorPayload{Code: code, Message: message})
}

func (h *Hub) sendTo(c *client, msgType string, payload any) {
	env, err := encode(msgType, payload)
	if err != nil {
		log.Printf("ws: encode %s: %v", msgType, err)
		return
	}
	select {
	case c.send <- env:
	default:
		log.Printf("ws: dropping %s for client %s: send buffer full", msgType, c.clientID)
	}
}

func (h *Hub) broadcastToRoom(rm *room, msgType string, payload any) {
	env, err := encode(msgType, payload)
	if err != nil {
		log.Printf("ws: encode %s: %v", msgType, err)
		return
	}
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	for _, c := range rm.clients {
		select {
		case c.send <- env:
		default:
			log.Printf("ws: dropping %s for client %s: send buffer full", msgType, c.clientID)
		}
	}
}

// ConnectedClientIDs returns the client IDs (as strings) currently
// connected to gameID's room, for the admin game-detail view.
func (h *Hub) ConnectedClientIDs(gameID uuid.UUID) map[string]bool {
	h.mu.RLock()
	rm, ok := h.rooms[gameID]
	h.mu.RUnlock()
	out := make(map[string]bool)
	if !ok {
		return out
	}
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	for id := range rm.clients {
		out[id.String()] = true
	}
	return out
}

// Broadcast sends payload to every client currently connected to gameID's
// room. It is a no-op if the room doesn't exist yet (e.g. no one has
// connected). Exported for REST admin handlers to call after mutating game
// state.
func (h *Hub) Broadcast(gameID uuid.UUID, msgType string, payload any) {
	h.mu.RLock()
	rm, ok := h.rooms[gameID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	h.broadcastToRoom(rm, msgType, payload)
}
