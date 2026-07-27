package telemetry

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/log"
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

// otlpJSONHandler renders each record as one JSON line — byte-identical
// to what the stdout handler writes, via the same slog.JSONHandler — and
// exports that whole line as a single opaque OTLP log body, rather than
// attaching each attribute as a separate OTel log attribute the way
// otelslog.Handler does.
//
// That distinction matters here: Loki's OTLP ingestion promotes log
// attributes it doesn't otherwise recognize to index labels, and
// per-request fields like path, status, and (worst of all) trace_id are
// each close to unique, so attaching them as attributes mints Loki a
// brand-new stream per request — a classic cardinality-explosion
// anti-pattern, not just a cosmetic issue. Keeping them as text inside
// the body sidesteps that: they're still fully queryable via `| json` in
// LogQL, just not indexed. Trace/span correlation still works natively
// here regardless, since that comes from ctx at Emit time, not from
// attributes.
type otlpJSONHandler struct {
	logger log.Logger
	json   slog.Handler
	buf    *bytes.Buffer
	mu     *sync.Mutex
}

func newOTLPJSONHandler(logger log.Logger) *otlpJSONHandler {
	buf := new(bytes.Buffer)
	return &otlpJSONHandler{logger: logger, json: slog.NewJSONHandler(buf, nil), buf: buf, mu: new(sync.Mutex)}
}

func (h *otlpJSONHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.json.Enabled(ctx, level)
}

func (h *otlpJSONHandler) Handle(ctx context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.buf.Reset()
	if err := h.json.Handle(ctx, record); err != nil {
		return err
	}
	body := strings.TrimRight(h.buf.String(), "\n")

	var rec log.Record
	rec.SetTimestamp(record.Time)
	rec.SetObservedTimestamp(time.Now())
	rec.SetSeverity(otelSeverity(record.Level))
	rec.SetSeverityText(record.Level.String())
	rec.SetBody(log.StringValue(body))
	h.logger.Emit(ctx, rec)
	return nil
}

func (h *otlpJSONHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &otlpJSONHandler{logger: h.logger, json: h.json.WithAttrs(attrs), buf: h.buf, mu: h.mu}
}

func (h *otlpJSONHandler) WithGroup(name string) slog.Handler {
	return &otlpJSONHandler{logger: h.logger, json: h.json.WithGroup(name), buf: h.buf, mu: h.mu}
}

// otelSeverity maps a slog level to its nearest OTel log severity. slog's
// four levels each become the "1" variant of the matching OTel severity
// band (there is no equivalent finer-grained source distinction to make
// use of the 2-4 variants).
func otelSeverity(level slog.Level) log.Severity {
	switch {
	case level >= slog.LevelError:
		return log.SeverityError
	case level >= slog.LevelWarn:
		return log.SeverityWarn
	case level >= slog.LevelInfo:
		return log.SeverityInfo
	default:
		return log.SeverityDebug
	}
}
