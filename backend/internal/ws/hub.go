// Package ws implements the live-gameplay websocket channel described by
// api/asyncapi.yaml: one hub keyed by game ID, one room per game, one
// client per connected player. REST admin handlers call the Broadcast*
// methods after mutating game state; the room never mutates state itself
// except via game.Service.SubmitAnswer on answer.submit.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/mattizspooky/quizmos/backend/internal/audit"
	db "github.com/mattizspooky/quizmos/backend/internal/db/sqlc"
	"github.com/mattizspooky/quizmos/backend/internal/game"
	"github.com/mattizspooky/quizmos/backend/internal/middleware"
	"github.com/mattizspooky/quizmos/backend/internal/question"
	"github.com/mattizspooky/quizmos/backend/internal/quiz"
)

// gameDomain logs "game.action" audit lines for player-initiated
// mutations over the websocket (answer.submit) — the same shape and
// helper internal/handlers uses for admin-initiated ones, so both are
// queryable together in Loki regardless of which side triggered them.
const gameDomain audit.Domain = "game"

// tracer and meter are safe to use before telemetry.Setup runs: otel.Tracer
// and otel.Meter both return lazily-delegating wrappers that resolve the
// real provider at each Start/measurement call, not at the time these vars
// are initialized.
var tracer = otel.Tracer("quizmos/ws")
var meter = otel.Meter("quizmos/ws")

var wsConnectionsActive = func() metric.Int64UpDownCounter {
	c, err := meter.Int64UpDownCounter("quizmos.ws.connections.active", metric.WithDescription("currently connected websocket clients, across all games"))
	if err != nil {
		panic(fmt.Sprintf("ws: create counter quizmos.ws.connections.active: %v", err))
	}
	return c
}()

type client struct {
	clientID uuid.UUID
	nickname string
	conn     *websocket.Conn
	send     chan Envelope

	// connID correlates every message on this connection without keeping
	// one trace open for the connection's whole (potentially very long)
	// lifetime — see handleMessage, which starts a fresh trace per message
	// and stamps it with this value. It's the handshake request's own
	// trace ID (assigned by otelhttp, see httpserver.New), reused rather
	// than minting a second identifier for the same connection.
	connID string
}

type room struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]*client
	// admins holds the admin live-control page's own websocket
	// connections for this game, keyed by connID (admins have no
	// client_id — a browser tab is just whichever connection it is).
	// They receive every message clients do (see broadcastToRoom) plus
	// admin-only ones (see broadcastToAdmins), but never count toward
	// playerCount/ConnectedClientIDs and can't submit answers.
	admins map[string]*client
	// reviewing tracks "we're currently showing a read-only recap of
	// this question index" — set on question.reviewed, cleared on the
	// next real question.started. ReviewQuestion itself never touches
	// the database (see game.Service.ReviewQuestion), so this in-memory flag
	// is the *only* record that a recap is in progress; without it, a
	// player who reconnects mid-recap would catch up to live play
	// instead of whatever's actually on everyone else's screen.
	reviewing *int
}

type Hub struct {
	games   *game.Service
	quizzes *quiz.Service

	mu    sync.RWMutex
	rooms map[uuid.UUID]*room

	// ticketsMu guards tickets, the single-use admin-websocket tickets
	// minted by MintAdminTicket and consumed by redeemAdminTicket — see
	// ticket.go.
	ticketsMu sync.Mutex
	tickets   map[string]ticketEntry

	AllowedOrigins []string
}

func NewHub(games *game.Service, quizzes *quiz.Service, allowedOrigins []string) *Hub {
	return &Hub{
		games:          games,
		quizzes:        quizzes,
		rooms:          make(map[uuid.UUID]*room),
		tickets:        make(map[string]ticketEntry),
		AllowedOrigins: allowedOrigins,
	}
}

func (h *Hub) roomFor(gameID uuid.UUID) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[gameID]
	if !ok {
		r = &room{clients: make(map[uuid.UUID]*client), admins: make(map[string]*client)}
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

	g, player, err := h.games.GetPlayerByCode(r.Context(), code, clientID)
	if err != nil {
		http.Error(w, "no such player in this game; join via POST /api/games/join first", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.AllowedOrigins})
	if err != nil {
		return
	}

	connID := trace.SpanContextFromContext(r.Context()).TraceID().String()
	c := &client{clientID: clientID, nickname: player.Nickname, conn: conn, send: make(chan Envelope, 16), connID: connID}
	rm := h.roomFor(g.ID)

	rm.mu.Lock()
	rm.clients[clientID] = c
	count := len(rm.clients)
	reviewing := rm.reviewing
	rm.mu.Unlock()

	slog.InfoContext(r.Context(), "ws.connect",
		"ws.connection_id", connID, "game.id", g.ID, "client.id", clientID, "nickname", player.Nickname)
	wsConnectionsActive.Add(r.Context(), 1)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go c.writeLoop(ctx)

	h.broadcastToRoom(rm, TypePresencePlayerJoined, PresencePlayerJoined{
		Player:      PlayerSummary{ClientID: clientID.String(), Nickname: player.Nickname},
		PlayerCount: int64(count),
	})
	h.sendCatchUp(ctx, g, c, reviewing)

	h.readLoop(ctx, g.ID, c)

	rm.mu.Lock()
	delete(rm.clients, clientID)
	remaining := len(rm.clients)
	rm.mu.Unlock()
	close(c.send)

	slog.InfoContext(ctx, "ws.disconnect",
		"ws.connection_id", connID, "game.id", g.ID, "client.id", clientID)
	wsConnectionsActive.Add(ctx, -1)

	h.broadcastToRoom(rm, TypePresencePlayerLeft, PresencePlayerLeft{
		ClientID:    clientID.String(),
		PlayerCount: int64(remaining),
	})
}

// UpgradeAdmin handles GET /ws/admin/games/{gameId}?ticket={ticket}. The
// ticket must have been minted moments earlier by POST
// /admin/games/{gameId}/ws-ticket (see handlers.CreateGameWsTicket, which
// only succeeds for an authenticated admin) — browsers can't attach the
// Authorization header to a websocket handshake, so this ticket is how the
// connection proves it belongs to that admin without the real bearer token
// ever appearing in a URL. Unlike the player Upgrade above, there's no
// catch-up snapshot: the admin page does one REST fetch itself around
// connecting, and this socket only ever carries deltas from that point on.
func (h *Hub) UpgradeAdmin(w http.ResponseWriter, r *http.Request) {
	gameID, err := uuid.Parse(chi.URLParam(r, "gameId"))
	if err != nil {
		http.Error(w, "invalid game id", http.StatusBadRequest)
		return
	}

	ticket := r.URL.Query().Get("ticket")
	if ticket == "" || !h.redeemAdminTicket(ticket, gameID) {
		http.Error(w, "missing or invalid ticket", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.AllowedOrigins})
	if err != nil {
		return
	}

	connID := trace.SpanContextFromContext(r.Context()).TraceID().String()
	c := &client{conn: conn, send: make(chan Envelope, 16), connID: connID}
	rm := h.roomFor(gameID)

	rm.mu.Lock()
	rm.admins[connID] = c
	rm.mu.Unlock()

	slog.InfoContext(r.Context(), "ws.admin_connect", "ws.connection_id", connID, "game.id", gameID)
	wsConnectionsActive.Add(r.Context(), 1)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go c.writeLoop(ctx)
	h.adminReadLoop(ctx, c)

	rm.mu.Lock()
	delete(rm.admins, connID)
	rm.mu.Unlock()
	close(c.send)

	slog.InfoContext(ctx, "ws.admin_disconnect", "ws.connection_id", connID, "game.id", gameID)
	wsConnectionsActive.Add(ctx, -1)
}

// adminReadLoop just drains inbound frames until the connection closes —
// admin connections are receive-only, there's nothing for them to submit,
// unlike readLoop's players which can send answer.submit.
func (h *Hub) adminReadLoop(ctx context.Context, c *client) {
	defer c.conn.CloseNow()
	for {
		var env Envelope
		if err := wsjson.Read(ctx, c.conn, &env); err != nil {
			return
		}
	}
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
func (h *Hub) sendCatchUp(ctx context.Context, g db.Game, c *client, reviewingIndex *int) {
	if g.Status == "in_progress" && reviewingIndex != nil {
		if result, err := h.games.ReviewQuestion(ctx, g.ID, *reviewingIndex); err == nil {
			h.sendTo(c, TypeQuestionReviewed, questionReviewedPayload(result.Question, result.Game.TotalQuestions, result.AnswerCounts))
			return
		}
		// Fall through to live catch-up below if the recap couldn't be
		// rebuilt (e.g. the index stopped being valid in some edge case) —
		// showing the live question is still better than showing nothing.
	}

	switch g.Status {
	case "in_progress":
		if !g.CurrentQuestionIndex.Valid {
			return
		}
		qz, err := h.quizzes.Get(ctx, g.QuizID)
		if err != nil {
			return
		}
		q, total, err := h.games.QuestionAtIndex(ctx, g.QuizID, int(g.CurrentQuestionIndex.Int32))
		if err != nil {
			return
		}
		options := make([]QuestionOption, len(q.Options))
		for i, o := range q.Options {
			options[i] = QuestionOption{ID: o.ID.String(), Text: o.Text}
		}
		mediaURL, mediaType := mediaFields(q)
		payload := QuestionStarted{
			QuestionIndex:    int64(q.Position),
			QuestionID:       q.ID.String(),
			Type:             Type(q.Type),
			Prompt:           q.Prompt,
			Options:          options,
			Timed:            qz.Timed,
			TimeLimitSeconds: int64(q.TimeLimitSeconds),
			TotalQuestions:   int64(total),
			MediaURL:         mediaURL,
			MediaType:        mediaType,
		}
		// A reconnecting client (dropped connection, page refresh) might
		// already have answered this exact question — without folding
		// that in, they'd see a blank, re-answerable question and a
		// resubmission would be silently rejected as a duplicate.
		if status, err := h.games.GetPlayerAnswerStatus(ctx, g.ID, c.clientID, q.ID); err == nil {
			applyYourAnswer(&payload, status)
		}
		h.sendTo(c, TypeQuestionStarted, payload)
	case "ended":
		leaderboard, err := h.games.Leaderboard(ctx, g.ID)
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
				Color:    Color(e.Color),
			}
		}
		h.sendTo(c, TypeGameEnded, GameEnded{FinalLeaderboard: entries, EndedAt: g.EndedAt.Time})
	}
}

// mediaFields mirrors handlers.mediaFields — kept as its own small copy
// here (rather than exported and shared) since handlers already imports
// ws, and having ws import handlers back would be a cycle.
func mediaFields(q question.WithOptions) (*string, *MediaType) {
	if q.MediaURL == "" {
		return nil, nil
	}
	url := q.MediaURL
	mediaType := MediaType(q.MediaType.String)
	return &url, &mediaType
}

// questionReviewedPayload mirrors handlers.questionReviewedPayload — kept
// as its own small copy here (rather than exported and shared) since
// handlers already imports ws, and having ws import handlers back would
// be a cycle.
func questionReviewedPayload(q question.WithOptions, total int, counts map[uuid.UUID]int) QuestionReviewed {
	options := make([]QuestionOption, len(q.Options))
	var correctID *string
	answerCounts := make([]AnswerCount, 0, len(q.Options))
	for i, o := range q.Options {
		options[i] = QuestionOption{ID: o.ID.String(), Text: o.Text}
		if o.IsCorrect {
			id := o.ID.String()
			correctID = &id
		}
		answerCounts = append(answerCounts, AnswerCount{OptionID: o.ID.String(), Count: int64(counts[o.ID])})
	}
	mediaURL, mediaType := mediaFields(q)
	return QuestionReviewed{
		QuestionIndex:   int64(q.Position),
		QuestionID:      q.ID.String(),
		Prompt:          q.Prompt,
		Options:         options,
		CorrectOptionID: correctID,
		AnswerCounts:    answerCounts,
		TotalQuestions:  int64(total),
		MediaURL:        mediaURL,
		MediaType:       mediaType,
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

// handleMessage gives every inbound message its own short-lived trace,
// rather than nesting it under the connection's own (potentially
// hours-long) handshake span: tracing backends generally assume a trace
// starts and finishes in seconds, and head-based sampling has to decide
// before knowing how long a connection will stay open. c.connID (shared by
// every message on this connection, and by the ws.connect/ws.disconnect
// log lines above) is how these otherwise-unrelated traces get correlated
// back into one session.
func (h *Hub) handleMessage(ctx context.Context, gameID uuid.UUID, c *client, env Envelope) {
	rootCtx := trace.ContextWithSpanContext(ctx, trace.SpanContext{})
	msgCtx, span := tracer.Start(rootCtx, "ws."+env.Type, trace.WithAttributes(
		attribute.String("ws.connection_id", c.connID),
		attribute.String("ws.message_type", env.Type),
		attribute.String("game.id", gameID.String()),
		attribute.String("client.id", c.clientID.String()),
	))
	defer span.End()

	slog.InfoContext(msgCtx, "ws.message",
		"ws.connection_id", c.connID, "ws.message_type", env.Type, "game.id", gameID, "client.id", c.clientID)

	switch env.Type {
	case TypeAnswerSubmit:
		var submit AnswerSubmit
		if err := json.Unmarshal(env.Payload, &submit); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "malformed answer.submit payload")
			h.sendError(c, "bad_request", "malformed answer.submit payload")
			return
		}
		h.handleAnswerSubmit(msgCtx, gameID, c, submit)
	default:
		span.SetStatus(codes.Error, "unrecognized message type")
		h.sendError(c, "unknown_message_type", "unrecognized message type: "+env.Type)
	}
}

func (h *Hub) handleAnswerSubmit(ctx context.Context, gameID uuid.UUID, c *client, submit AnswerSubmit) {
	questionID, err := uuid.Parse(submit.QuestionID)
	if err != nil {
		trace.SpanFromContext(ctx).SetStatus(codes.Error, "questionId must be a UUID")
		h.sendError(c, "bad_request", "questionId must be a UUID")
		return
	}

	var optionID *uuid.UUID
	if submit.OptionID != nil {
		id, err := uuid.Parse(*submit.OptionID)
		if err != nil {
			trace.SpanFromContext(ctx).SetStatus(codes.Error, "optionId must be a UUID")
			h.sendError(c, "bad_request", "optionId must be a UUID")
			return
		}
		optionID = &id
	}

	result, err := h.games.SubmitAnswer(ctx, gameID, c.clientID, questionID, optionID, submit.Text)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err)
		span.SetStatus(codes.Error, "answer rejected")
		h.sendError(c, "answer_rejected", err.Error())
		return
	}

	gameDomain.Log(ctx, "game.answer_submitted", gameID, audit.Player, c.clientID.String(),
		"nickname", c.nickname, "question.id", questionID, "correct", result.Correct, "points_awarded", result.PointsAwarded, "pending", result.Pending)

	h.sendTo(c, TypeAnswerResult, AnswerResult{
		QuestionID:    submit.QuestionID,
		Correct:       result.Correct,
		PointsAwarded: int64(result.PointsAwarded),
		TotalScore:    int64(result.TotalScore),
		Pending:       result.Pending,
	})

	if submit.Text != nil {
		h.broadcastFreeTextAnswerUpdate(ctx, gameID, questionID, c.clientID)
	}
}

// broadcastFreeTextAnswerUpdate tells connected admins about a free-text
// submission the moment it happens, replacing the admin page's old poll of
// ListFreeTextAnswers. It re-reads the row it just wrote (same pattern
// handlers.GradeAnswer uses to report a grading outcome) rather than
// threading an answer ID back out of game.Service.SubmitAnswer, which
// today only reports the score, not the row.
func (h *Hub) broadcastFreeTextAnswerUpdate(ctx context.Context, gameID, questionID, clientID uuid.UUID) {
	answers, err := h.games.ListFreeTextAnswers(ctx, gameID, questionID)
	if err != nil {
		slog.WarnContext(ctx, "ws.free_text_answer_lookup_failed", "error", err)
		return
	}
	for _, a := range answers {
		if a.ClientID != clientID {
			continue
		}
		payload := FreeTextAnswerUpdated{
			QuestionID: questionID.String(),
			ID:         a.ID.String(),
			ClientID:   a.ClientID.String(),
			Nickname:   a.Nickname,
			Text:       a.Text,
			Graded:     a.Graded,
		}
		if a.Graded {
			correct := a.Correct
			points := int64(a.PointsAwarded)
			payload.Correct = &correct
			payload.PointsAwarded = &points
		}
		h.BroadcastToAdmins(gameID, TypeFreeTextAnswerUpdated, payload)
		return
	}
}

func (h *Hub) sendError(c *client, code, message string) {
	h.sendTo(c, TypeError, ErrorPayload{Code: code, Message: message})
}

func (h *Hub) sendTo(c *client, msgType string, payload any) {
	env, err := encode(msgType, payload)
	if err != nil {
		slog.Error("ws.encode_failed", "ws.message_type", msgType, "ws.connection_id", c.connID, "error", err)
		return
	}
	select {
	case c.send <- env:
	default:
		slog.Warn("ws.send_dropped", "ws.message_type", msgType, "ws.connection_id", c.connID, "client.id", c.clientID)
	}
}

func (h *Hub) broadcastToRoom(rm *room, msgType string, payload any) {
	env, err := encode(msgType, payload)
	if err != nil {
		slog.Error("ws.encode_failed", "ws.message_type", msgType, "error", err)
		return
	}
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	for _, c := range rm.clients {
		select {
		case c.send <- env:
		default:
			slog.Warn("ws.send_dropped", "ws.message_type", msgType, "ws.connection_id", c.connID, "client.id", c.clientID)
		}
	}
	// Every player-facing broadcast reaches connected admins too, so the
	// admin live-control page can replace its game-state polling with
	// this same feed — see broadcastToAdmins for the reverse (admin-only
	// messages that must never reach players).
	sendEnvelopeTo(rm.admins, msgType, env)
}

// broadcastToAdmins sends payload only to gameID's connected admin
// connections (see room.admins) — for events, like a free-text answer
// submission, that must not reach players.
func (h *Hub) broadcastToAdmins(rm *room, msgType string, payload any) {
	env, err := encode(msgType, payload)
	if err != nil {
		slog.Error("ws.encode_failed", "ws.message_type", msgType, "error", err)
		return
	}
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	sendEnvelopeTo(rm.admins, msgType, env)
}

// sendEnvelopeTo delivers env to every client in recipients — recipients
// is always rm.admins today, but takes a plain map (not *room) so it can
// be called while rm.mu is already held for reading, by either
// broadcastToRoom or broadcastToAdmins.
func sendEnvelopeTo(recipients map[string]*client, msgType string, env Envelope) {
	for _, c := range recipients {
		select {
		case c.send <- env:
		default:
			slog.Warn("ws.send_dropped", "ws.message_type", msgType, "ws.connection_id", c.connID)
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

// SendToClient unicasts payload to one specific client in gameID's room, if
// they're currently connected — a no-op otherwise (e.g. they submitted a
// free-text answer and then closed the tab before the admin got to
// grading it). Exported for REST admin handlers, mirroring Broadcast.
func (h *Hub) SendToClient(gameID, clientID uuid.UUID, msgType string, payload any) {
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
	h.sendTo(c, msgType, payload)
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
	conns := make([]*websocket.Conn, 0, len(rm.clients)+len(rm.admins))
	for _, c := range rm.clients {
		conns = append(conns, c.conn)
	}
	for _, c := range rm.admins {
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

// BroadcastToAdmins sends payload only to gameID's connected admin
// connections (not to players), creating the room if this is the first
// thing ever broadcast for this game. Exported for REST admin handlers to
// call after mutating state that only the admin view cares about (e.g. a
// free-text answer being submitted or graded).
func (h *Hub) BroadcastToAdmins(gameID uuid.UUID, msgType string, payload any) {
	h.broadcastToAdmins(h.roomFor(gameID), msgType, payload)
}

// BroadcastQuestionStarted sends question.started to every client
// currently connected to gameID's room, personalizing each recipient's
// copy with their own existing answer to the question, if they have one
// (see PlayerAnswerStatus). Plain Broadcast is fine for a genuinely new
// question — nobody could have answered it yet — but resuming live play
// after a review redelivers question.started for a question that isn't
// actually fresh to everyone in the room, and without this they'd see a
// blank, re-answerable question client-side even though resubmitting
// would be rejected as a duplicate answer.
func (h *Hub) BroadcastQuestionStarted(ctx context.Context, gameID uuid.UUID, base QuestionStarted) {
	rm := h.roomFor(gameID)
	rm.mu.Lock()
	rm.reviewing = nil
	rm.mu.Unlock()

	rm.mu.RLock()
	clients := make([]*client, 0, len(rm.clients))
	for _, c := range rm.clients {
		clients = append(clients, c)
	}
	rm.mu.RUnlock()

	questionID, err := uuid.Parse(base.QuestionID)
	if err != nil {
		h.broadcastToRoom(rm, TypeQuestionStarted, base)
		return
	}

	// One query for every connected client's answer status, rather than a
	// GetPlayerAnswerStatus round trip per client — a room of N players
	// would otherwise be N*2 sequential queries just to resume live play.
	statuses, err := h.games.AnswerStatusesForQuestion(ctx, gameID, questionID)
	if err != nil {
		statuses = nil // fall back to sending the un-personalized payload rather than failing the whole broadcast
	}

	for _, c := range clients {
		payload := base
		if status, ok := statuses[c.clientID]; ok {
			applyYourAnswer(&payload, status)
		}
		h.sendTo(c, TypeQuestionStarted, payload)
	}

	// Admins have no "your answer" of their own to personalize — they get
	// the plain base payload, same as broadcastToRoom would send.
	h.broadcastToAdmins(rm, TypeQuestionStarted, base)
}

// applyYourAnswer folds a player's existing answer (if any) into their
// copy of a question.started payload.
func applyYourAnswer(payload *QuestionStarted, status game.PlayerAnswerStatus) {
	if !status.Answered {
		return
	}
	ya := &YourAnswer{Pending: status.Pending}
	if status.SelectedOptionID != nil {
		id := status.SelectedOptionID.String()
		ya.OptionID = &id
	}
	if status.Text != nil {
		ya.Text = status.Text
	}
	if !status.Pending {
		correct := status.Correct
		points := int64(status.PointsAwarded)
		ya.Correct = &correct
		ya.PointsAwarded = &points
	}
	payload.YourAnswer = ya
}
