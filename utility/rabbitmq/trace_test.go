package rabbitmq

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func recorder(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	sink := tracetest.NewInMemoryExporter()
	old, oldProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(sink)))
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTracerProvider(old); otel.SetTextMapPropagator(oldProp) })
	return sink
}

// publish returns the headers a message published inside a span gets.
func published(delayed bool) (amqp.Table, trace.SpanContext) {
	ctx, span := otel.Tracer("t").Start(context.Background(), "publish")
	defer span.End()
	headers := amqp.Table{}
	inject(ctx, headers)
	if delayed {
		headers[headerDelayed] = true
	}
	return headers, span.SpanContext()
}

func TestConsumerContinuesThePublishersTrace(t *testing.T) {
	recorder(t)
	headers, producer := published(false)
	if headers["traceparent"] == "" {
		t.Fatal("no traceparent header")
	}
	_, span := startConsume(context.Background(), "q", amqp.Delivery{Headers: headers, MessageId: "m"})
	span.End()
	got := span.(sdktrace.ReadOnlySpan)
	if got.SpanContext().TraceID() != producer.TraceID() || got.Parent().SpanID() != producer.SpanID() || got.SpanKind() != trace.SpanKindConsumer {
		t.Fatalf("consumer span not a child of the publisher: %+v", got.Parent())
	}
}

func TestDelayedMessageStartsALinkedTrace(t *testing.T) {
	recorder(t)
	headers, producer := published(true)
	_, span := startConsume(context.Background(), "q", amqp.Delivery{Headers: headers})
	span.End()
	got := span.(sdktrace.ReadOnlySpan)
	if got.SpanContext().TraceID() == producer.TraceID() || got.Parent().IsValid() {
		t.Fatal("a delayed message continued the old trace")
	}
	if len(got.Links()) != 1 || got.Links()[0].SpanContext.SpanID() != producer.SpanID() {
		t.Fatalf("links %+v", got.Links())
	}
}

func TestConsumerWithoutTraceHeadersStartsFresh(t *testing.T) {
	recorder(t)
	_, span := startConsume(context.Background(), "q", amqp.Delivery{})
	span.End()
	if !span.SpanContext().IsValid() || span.(sdktrace.ReadOnlySpan).Parent().IsValid() {
		t.Fatal("expected a new root span")
	}
}
