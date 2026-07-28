package ws

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestHub() *Hub {
	return NewHub(nil, nil, nil)
}

func TestAdminTicket_ValidTicketRedeemsOnce(t *testing.T) {
	h := newTestHub()
	gameID := uuid.New()

	ticket := h.MintAdminTicket(gameID)

	if !h.redeemAdminTicket(ticket, gameID) {
		t.Fatal("expected first redeem to succeed")
	}
	if h.redeemAdminTicket(ticket, gameID) {
		t.Error("expected second redeem of the same ticket to fail")
	}
}

func TestAdminTicket_WrongGameIDFails(t *testing.T) {
	h := newTestHub()
	ticket := h.MintAdminTicket(uuid.New())

	if h.redeemAdminTicket(ticket, uuid.New()) {
		t.Error("expected redeem for a different game id to fail")
	}
}

func TestAdminTicket_UnknownTicketFails(t *testing.T) {
	h := newTestHub()

	if h.redeemAdminTicket(uuid.NewString(), uuid.New()) {
		t.Error("expected redeem of a never-minted ticket to fail")
	}
}

func TestAdminTicket_ExpiredTicketFails(t *testing.T) {
	h := newTestHub()
	gameID := uuid.New()
	ticket := uuid.NewString()

	h.ticketsMu.Lock()
	h.tickets[ticket] = ticketEntry{gameID: gameID, expires: time.Now().Add(-time.Second)}
	h.ticketsMu.Unlock()

	if h.redeemAdminTicket(ticket, gameID) {
		t.Error("expected redeem of an expired ticket to fail")
	}
}

func TestAdminTicket_MintPrunesExpiredEntries(t *testing.T) {
	h := newTestHub()
	stale := uuid.NewString()

	h.ticketsMu.Lock()
	h.tickets[stale] = ticketEntry{gameID: uuid.New(), expires: time.Now().Add(-time.Minute)}
	h.ticketsMu.Unlock()

	h.MintAdminTicket(uuid.New())

	h.ticketsMu.Lock()
	_, stillThere := h.tickets[stale]
	h.ticketsMu.Unlock()
	if stillThere {
		t.Error("expected the stale ticket to be pruned by the next mint")
	}
}
