// Package middleware holds simple chi middleware that isn't driven by the
// OpenAPI security scheme (which only covers the /api subtree).
package middleware

import "net/http"

type contextKey string

const clientIDContextKey contextKey = "quizmos.client_id"

// ClientID reads the caller-supplied client identifier so downstream
// handlers (currently just the websocket upgrade) can look it up without
// re-parsing query params. Absence is not fatal here; handlers decide.
func ClientID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.Header.Get("X-Client-Id")
		if clientID == "" {
			clientID = r.URL.Query().Get("client_id")
		}
		if clientID != "" {
			ctx := setClientID(r.Context(), clientID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
