// Package telemetry sets up distributed tracing (design §9): every service
// sends its spans over OTLP to the OpenTelemetry Collector, which forwards
// them to Jaeger.
//
// GoFrame already creates the spans that matter once a real tracer provider
// is installed: one per HTTP request, gRPC call (client and server), SQL
// statement and Redis command, and it propagates W3C trace context over HTTP
// and gRPC. This package installs that provider, removes secrets from spans
// before they leave the process (see redact.go), and gives background work
// its own traces (Quiet, Root).
package telemetry

import (
	"context"
	"crypto/rand"
	"os"
	"runtime/debug"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

// Instrumentation is the tracer name for spans this repository creates
// itself (events, background work).
const Instrumentation = "github.com/hughtaiyaki/goframeshop"

// Init installs the global tracer provider for service, configured from
// otel.endpoint (host:port of the collector's OTLP gRPC receiver, or env
// OTEL_ENDPOINT), otel.sampleRatio (default 1) and otel.environment
// (default "local"). With no endpoint, tracing stays off and nothing is
// exported. The returned function flushes pending spans; call it on exit.
func Init(ctx context.Context, service string) (func(context.Context) error, error) {
	// Always propagate trace context, even with tracing off, so a service
	// in the middle does not break the chain for the others.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	endpoint := g.Cfg().MustGet(ctx, "otel.endpoint").String()
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_ENDPOINT")
	}
	if endpoint == "" {
		g.Log().Info(ctx, "tracing is off: otel.endpoint is not set")
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(service),
		semconv.ServiceVersion(version()),
		attribute.String("deployment.environment.name", g.Cfg().MustGet(ctx, "otel.environment", "local").String()),
	))
	if err != nil {
		return nil, err
	}
	ratio := g.Cfg().MustGet(ctx, "otel.sampleRatio", 1.0).Float64()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithBatcher(&redactingExporter{next: exporter}, sdktrace.WithBatchTimeout(time.Second)),
	)
	otel.SetTracerProvider(provider)
	g.Log().Infof(ctx, "tracing on: service %s exports to %s (sample ratio %.2f)", service, endpoint, ratio)
	return provider.Shutdown, nil
}

// version is the module version or VCS revision the binary was built from.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 12 {
			return s.Value[:12]
		}
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return "dev"
}

// Quiet returns ctx carrying a valid trace that is NOT sampled. Background
// loops run on it, so their polling (the relay looking for events every
// 500ms, the recovery scan) records nothing: with the parent-based sampler,
// children of an unsampled parent are not recorded either. Logs still show
// a trace id. Work worth tracing starts its own trace with Root.
func Quiet(ctx context.Context) context.Context {
	var tid trace.TraceID
	var sid trace.SpanID
	_, _ = rand.Read(tid[:])
	_, _ = rand.Read(sid[:])
	return trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: 0,
	}))
}

// Root starts a new trace for one unit of background work (one order the
// recovery worker drives, one notification sent), so each gets its own trace
// id instead of all of them sharing the process's startup trace.
func Root(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(Instrumentation).Start(ctx, name,
		trace.WithNewRoot(), trace.WithSpanKind(trace.SpanKindInternal), trace.WithAttributes(attrs...))
}

// Start is what every main calls first: structured logging, Init, then a
// Quiet context for the process's own background work. The returned function flushes spans that
// are still buffered; defer it.
func Start(ctx context.Context, service string) (context.Context, func()) {
	// Logs first, so every line the process writes is structured.
	InstallLogging(ctx, service)
	shutdown, err := Init(ctx, service)
	if err != nil {
		g.Log().Fatalf(ctx, "tracing setup failed: %v", err)
	}
	return Quiet(ctx), func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			g.Log().Warningf(ctx, "flush traces on exit: %v", err)
		}
	}
}
