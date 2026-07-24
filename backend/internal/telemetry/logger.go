package telemetry

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// traceEnrichedHandler adds trace_id/span_id attributes (read from the
// record's context) before delegating to the wrapped handler. The OTLP
// export path (otelslog.Handler) already correlates records with their span
// natively, so this only wraps handlers — like the stdout JSON one below —
// that don't, keeping stdout logs correlatable with a trace even without
// opening Grafana.
type traceEnrichedHandler struct {
	slog.Handler
}

func (h traceEnrichedHandler) Handle(ctx context.Context, record slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		record = record.Clone()
		record.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, record)
}

func (h traceEnrichedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceEnrichedHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceEnrichedHandler) WithGroup(name string) slog.Handler {
	return traceEnrichedHandler{h.Handler.WithGroup(name)}
}

// fanoutHandler dispatches every record to each of its handlers in turn —
// used to write logs to stdout (for local/`docker logs` viewing) and export
// them via OTLP (for Loki) from a single slog.Logger.
type fanoutHandler []slog.Handler

func (f fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, h := range f {
		if !h.Enabled(ctx, record.Level) {
			continue
		}
		if err := h.Handle(ctx, record.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (f fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make(fanoutHandler, len(f))
	for i, h := range f {
		next[i] = h.WithAttrs(attrs)
	}
	return next
}

func (f fanoutHandler) WithGroup(name string) slog.Handler {
	next := make(fanoutHandler, len(f))
	for i, h := range f {
		next[i] = h.WithGroup(name)
	}
	return next
}
