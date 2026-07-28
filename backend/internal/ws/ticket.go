package ws

import (
	"time"

	"github.com/google/uuid"
)

// adminTicketTTL is deliberately short: a ticket only has to survive the
// time between POST /admin/games/{gameId}/ws-ticket returning and the
// browser's very next websocket handshake, not a whole admin session.
const adminTicketTTL = 30 * time.Second

type ticketEntry struct {
	gameID  uuid.UUID
	expires time.Time
}

// MintAdminTicket issues a single-use, short-lived ticket for connecting to
// the admin websocket (see UpgradeAdmin). Browsers can't attach the
// Authorization header to a websocket handshake, so this stands in for the
// bearer token the REST handler calling this already verified, without
// ever putting the real token in a URL (and therefore into access logs or
// trace attributes).
func (h *Hub) MintAdminTicket(gameID uuid.UUID) string {
	ticket := uuid.NewString()
	now := time.Now()

	h.ticketsMu.Lock()
	defer h.ticketsMu.Unlock()
	// Opportunistic cleanup: tickets are single-use and expire in seconds,
	// so a background sweep goroutine would be overkill — piggybacking the
	// sweep on every mint keeps the map from growing with ones nobody ever
	// redeemed.
	for t, e := range h.tickets {
		if now.After(e.expires) {
			delete(h.tickets, t)
		}
	}
	h.tickets[ticket] = ticketEntry{gameID: gameID, expires: now.Add(adminTicketTTL)}
	return ticket
}

// redeemAdminTicket consumes a ticket if it exists, hasn't expired, and was
// minted for gameID — regardless of outcome, it's gone from the store
// afterward, since a ticket is single-use even when redemption fails (e.g.
// the caller passed the right ticket but the wrong game id).
func (h *Hub) redeemAdminTicket(ticket string, gameID uuid.UUID) bool {
	h.ticketsMu.Lock()
	defer h.ticketsMu.Unlock()
	e, ok := h.tickets[ticket]
	if !ok {
		return false
	}
	delete(h.tickets, ticket)
	return !time.Now().After(e.expires) && e.gameID == gameID
}
