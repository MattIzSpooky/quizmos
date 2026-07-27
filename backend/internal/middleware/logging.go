package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

// RequestLogger logs one structured line per request — telemetry.Setup's
// traceEnrichedHandler tags it (and every other log line) with the trace
// ID otelhttp assigned it (see httpserver.New, which wraps the whole
// router in otelhttp.NewHandler ahead of this middleware), so a log line
// and its trace are always one lookup apart without this middleware
// needing to add it itself. It also echoes that trace ID back as
// X-Trace-Id, cheap enough to be worth it: a user hitting a bug can hand
// that value to whoever's looking at Tempo/Grafana.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

		traceID := trace.SpanContextFromContext(r.Context()).TraceID()
		if traceID.IsValid() {
			w.Header().Set("X-Trace-Id", traceID.String())
		}

		next.ServeHTTP(ww, r)

		slog.InfoContext(r.Context(), "http.request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}
