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

	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
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
	// reviewing tracks "we're currently showing a read-only recap of
	// this question index" — set on question.reviewed, cleared on the
	// next real question.started. ReviewQuestion itself never touches
	// the database (see service.ReviewQuestion), so this in-memory flag
	// is the *only* record that a recap is in progress; without it, a
	// player who reconnects mid-recap would catch up to live play
	// instead of whatever's actually on everyone else's screen.
	reviewing *int
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
	reviewing := rm.reviewing
	rm.mu.Unlock()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go c.writeLoop(ctx)

	h.broadcastToRoom(rm, TypePresencePlayerJoined, PresencePlayerJoined{
		Player:      PlayerSummary{ClientID: clientID.String(), Nickname: player.Nickname},
		PlayerCount: int64(count),
	})
	h.sendCatchUp(ctx, game, c, reviewing)

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

// sendCatchUp gets a freshly-connected client to the same state as
// everyone else in the room. Without it, a player who reconnects (a
// dropped connection, a page refresh) while a question is already live
// would just sit on the lobby screen forever: question.started only
// ever broadcasts once, at the moment the question actually starts.
// reviewingIndex mirrors whatever the room's in-memory recap state was
// at the moment this client connected (see room.reviewing) — when set,
// it takes priority over live play, so someone reconnecting mid-recap
// sees the same read-only question everyone else on screen sees, not
// whatever question is actually live underneath it.
func (h *Hub) sendCatchUp(ctx context.Context, game db.Game, c *client, reviewingIndex *int) {
	if game.Status == "in_progress" && reviewingIndex != nil {
		if result, err := h.svc.ReviewQuestion(ctx, game.ID, *reviewingIndex); err == nil {
			h.sendTo(c, TypeQuestionReviewed, questionReviewedPayload(result.Question, result.Game.TotalQuestions, result.AnswerCounts))
			return
		}
		// Fall through to live catch-up below if the recap couldn't be
		// rebuilt (e.g. the index stopped being valid in some edge case) —
		// showing the live question is still better than showing nothing.
	}

	switch game.Status {
	case "in_progress":
		if !game.CurrentQuestionIndex.Valid {
			return
		}
		quiz, err := h.svc.GetQuiz(ctx, game.QuizID)
		if err != nil {
			return
		}
		question, total, err := h.svc.QuestionAtIndex(ctx, game.QuizID, int(game.CurrentQuestionIndex.Int32))
		if err != nil {
			return
		}
		options := make([]QuestionOption, len(question.Options))
		for i, o := range question.Options {
			options[i] = QuestionOption{ID: o.ID.String(), Text: o.Text}
		}
		h.sendTo(c, TypeQuestionStarted, QuestionStarted{
			QuestionIndex:    int64(question.Position),
			QuestionID:       question.ID.String(),
			Prompt:           question.Prompt,
			Options:          options,
			Timed:            quiz.Timed,
			TimeLimitSeconds: int64(question.TimeLimitSeconds),
			TotalQuestions:   int64(total),
		})
	case "ended":
		leaderboard, err := h.svc.Leaderboard(ctx, game.ID)
		if err != nil {
			return
		}
		entries := make([]LeaderboardEntry, len(leaderboard))
		for i, e := range leaderboard {
			entries[i] = LeaderboardEntry{
				ClientID: e.ClientID.String(),
				Nickname: e.Nickname,
				Score:    int64(e.Score),
				Rank:     int64(e.Rank),
			}
		}
		h.sendTo(c, TypeGameEnded, GameEnded{FinalLeaderboard: entries, EndedAt: game.EndedAt.Time})
	}
}

// questionReviewedPayload mirrors handlers.questionReviewedPayload — kept
// as its own small copy here (rather than exported and shared) since
// handlers already imports ws, and having ws import handlers back would
// be a cycle.
func questionReviewedPayload(q service.QuestionWithOptions, total int, counts map[uuid.UUID]int) QuestionReviewed {
	options := make([]QuestionOption, len(q.Options))
	var correctID string
	answerCounts := make([]AnswerCount, 0, len(q.Options))
	for i, o := range q.Options {
		options[i] = QuestionOption{ID: o.ID.String(), Text: o.Text}
		if o.IsCorrect {
			correctID = o.ID.String()
		}
		answerCounts = append(answerCounts, AnswerCount{OptionID: o.ID.String(), Count: int64(counts[o.ID])})
	}
	return QuestionReviewed{
		QuestionIndex:   int64(q.Position),
		QuestionID:      q.ID.String(),
		Prompt:          q.Prompt,
		Options:         options,
		CorrectOptionID: correctID,
		AnswerCounts:    answerCounts,
		TotalQuestions:  int64(total),
	}
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

// Kick disconnects a specific client from a game's room, if they're
// currently connected — a no-op otherwise (they may never have opened
// the play screen). It notifies them with player.kicked first, then
// closes the connection shortly after so the message has time to flush;
// the room's other members learn about it the normal way, via the
// presence.playerLeft that fires once the connection actually drops.
func (h *Hub) Kick(gameID, clientID uuid.UUID, reason string) {
	h.mu.RLock()
	rm, ok := h.rooms[gameID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	rm.mu.RLock()
	c, ok := rm.clients[clientID]
	rm.mu.RUnlock()
	if !ok {
		return
	}

	h.sendTo(c, TypePlayerKicked, PlayerKicked{Reason: reason})
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = c.conn.CloseNow()
	}()
}

// CloseRoom disconnects every client currently in gameID's room — call it
// once the game has ended, after broadcasting game.ended, so connections
// don't sit open indefinitely with nothing left to happen. The same grace
// period as Kick gives that broadcast time to actually reach clients
// before their sockets close.
func (h *Hub) CloseRoom(gameID uuid.UUID) {
	h.mu.RLock()
	rm, ok := h.rooms[gameID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	rm.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(rm.clients))
	for _, c := range rm.clients {
		conns = append(conns, c.conn)
	}
	rm.mu.RUnlock()

	go func() {
		time.Sleep(300 * time.Millisecond)
		for _, conn := range conns {
			_ = conn.CloseNow()
		}
	}()
}

// Broadcast sends payload to every client currently connected to gameID's
// room, creating the room (with no clients in it yet) if this is the
// first thing ever broadcast for this game. Exported for REST admin
// handlers to call after mutating game state.
func (h *Hub) Broadcast(gameID uuid.UUID, msgType string, payload any) {
	rm := h.roomFor(gameID)

	switch p := payload.(type) {
	case QuestionStarted:
		rm.mu.Lock()
		rm.reviewing = nil
		rm.mu.Unlock()
	case QuestionReviewed:
		idx := int(p.QuestionIndex)
		rm.mu.Lock()
		rm.reviewing = &idx
		rm.mu.Unlock()
	}

	h.broadcastToRoom(rm, msgType, payload)
}
