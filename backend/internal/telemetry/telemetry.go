// Package telemetry wires up OpenTelemetry for the backend: a
// TracerProvider and LoggerProvider that both export over OTLP/gRPC (see
// the "lgtm" service in docker-compose.yml), plus the stdlib log/slog
// default logger, bridged so every slog.*Context call lands both on stdout
// and in the log provider, correlated with whatever span is active on the
// passed context.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/mattizspooky/quizmos/backend/internal/config"
)

const serviceName = "quizmos-backend"

// Setup configures the global TracerProvider, LoggerProvider and
// propagator, and points slog.Default() at a handler that writes JSON to
// stdout and forwards to the LoggerProvider. It returns a shutdown func
// that flushes and closes both providers — call it via defer, alongside
// the other infra teardown in cmd/server/main.go.
//
// Both OTLP exporters dial the collector in the background: an unreachable
// or slow-to-start collector (cfg.OTelExporterEndpoint) must never delay
// the backend accepting real traffic.
func Setup(ctx context.Context, cfg config.Config) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(cfg.OTelServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTelExporterEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	logExporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(cfg.OTelExporterEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: log exporter: %w", err)
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	stdout := NewStdoutHandler(cfg.LogFormat)
	otelHandler := traceEnrichedHandler{newOTLPJSONHandler(loggerProvider.Logger(serviceName))}
	slog.SetDefault(slog.New(fanoutHandler{stdout, otelHandler}))

	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTelExporterEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: metric exporter: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)
	// otelhttp (wrapping the router in httpserver.New) picks up this same
	// global MeterProvider automatically, so HTTP server request
	// count/duration/size metrics start flowing with no extra wiring.

	shutdown := func(ctx context.Context) error {
		return errors.Join(
			tracerProvider.Shutdown(ctx),
			loggerProvider.Shutdown(ctx),
			meterProvider.Shutdown(ctx),
		)
	}
	return shutdown, nil
}
