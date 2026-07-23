package handlers

import (
	"context"

	"github.com/mattizspooky/quizmos/backend/internal/auth"
	"github.com/mattizspooky/quizmos/backend/internal/service"
	"github.com/mattizspooky/quizmos/backend/internal/ws"
)

// Handlers implements api.StrictServerInterface. It holds the domain
// service (persistence + business rules) and the websocket hub (to
// broadcast the effects of admin mutations to connected players).
type Handlers struct {
	svc      *service.Service
	hub      *ws.Hub
	keycloak *auth.Keycloak
}

func New(svc *service.Service, hub *ws.Hub, keycloak *auth.Keycloak) *Handlers {
	return &Handlers{svc: svc, hub: hub, keycloak: keycloak}
}

// adminSubject returns the Keycloak subject of the authenticated caller,
// used only for the created_by audit column.
func adminSubject(ctx context.Context) string {
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		return claims.Subject
	}
	return ""
}
