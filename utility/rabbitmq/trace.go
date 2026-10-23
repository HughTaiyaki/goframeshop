package rabbitmq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

// Trace context travels in the message headers (traceparent, tracestate),
// the same W3C format HTTP and gRPC use (design §9.1).

// headerDelayed marks a message published with a delay. Its consumer starts
// a new trace that links back to the publisher instead of continuing it: a
// payment expiry fifteen minutes later is not part of the request that
// created the order, and a span that long would make that trace unreadable.
const headerDelayed = "x-delayed"

// headerCarrier adapts amqp.Table to the propagation API.
type headerCarrier amqp.Table

func (c headerCarrier) Get(key string) string {
	if v, ok := c[key].(string); ok {
		return v
	}
	return ""
}

func (c headerCarrier) Set(key, value string) { c[key] = value }

func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// inject writes ctx's trace context into headers.
func inject(ctx context.Context, headers amqp.Table) {
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier(headers))
}

// startConsume opens the span for one delivery: a child of the publisher's
// span, or for a delayed message a new trace linked to it.
func startConsume(ctx context.Context, queue string, d amqp.Delivery) (context.Context, trace.Span) {
	headers := d.Headers
	if headers == nil {
		headers = amqp.Table{}
	}
	producer := otel.GetTextMapPropagator().Extract(ctx, headerCarrier(headers))
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.operation.type", "process"),
			attribute.String("messaging.destination.name", queue),
			attribute.String("messaging.rabbitmq.destination.routing_key", d.RoutingKey),
			attribute.String("messaging.message.id", d.MessageId),
			attribute.Bool("messaging.redelivered", d.Redelivered),
		),
	}
	parent := producer
	if delayed, _ := headers[headerDelayed].(bool); delayed {
		opts = append(opts, trace.WithNewRoot(), trace.WithLinks(trace.LinkFromContext(producer)))
		parent = ctx
	}
	return otel.Tracer(telemetry.Instrumentation).Start(parent, "consume "+queue, opts...)
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, fmt.Sprint(err))
	}
	span.End()
}
