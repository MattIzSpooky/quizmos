package middleware

import "context"

func setClientID(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, clientIDContextKey, clientID)
}

// ClientIDFromContext returns the caller-supplied client identifier, if any.
func ClientIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(clientIDContextKey).(string)
	return v, ok
}
