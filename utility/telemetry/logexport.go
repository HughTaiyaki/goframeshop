package telemetry

// Log export (task 0020): the same structured lines that go to stdout are
// also sent over OTLP to the collector, which stores them in Loki. Each
// line becomes one log record: the message is the body, the contract fields
// and the event attributes are attributes, severity and time map across.

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

// logExporter is the OTLP logger once initLogExport has run; nil means
// stdout only.
var logExporter atomic.Pointer[otellog.Logger]

func initLogExport(ctx context.Context, endpoint string, res *resource.Resource) (func(context.Context) error, error) {
	exporter, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpoint(endpoint), otlploggrpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter, sdklog.WithExportInterval(time.Second))),
	)
	logger := provider.Logger(Instrumentation)
	logExporter.Store(&logger)
	return func(ctx context.Context) error {
		logExporter.Store(nil)
		return provider.Shutdown(ctx)
	}, nil
}

// exportLog sends one formatted line, if export is on.
func exportLog(ctx context.Context, at time.Time, r *logRecord) {
	logger := logExporter.Load()
	if logger == nil {
		return
	}
	var rec otellog.Record
	rec.SetTimestamp(at)
	rec.SetObservedTimestamp(time.Now())
	rec.SetSeverity(otelSeverity(r.Severity))
	rec.SetSeverityText(r.Severity)
	rec.SetBody(otellog.StringValue(r.Message))
	attrs := []otellog.KeyValue{}
	for _, f := range []struct{ key, value string }{
		{"event.name", r.EventName}, {"operation_id", r.OperationID}, {"order_id", r.OrderID},
		{"error.type", r.ErrorType}, {"error.stack", r.Stack},
	} {
		if f.value != "" {
			attrs = append(attrs, otellog.String(f.key, f.value))
		}
	}
	for k, v := range r.Attributes {
		attrs = append(attrs, otellog.KeyValue{Key: k, Value: logValue(v)})
	}
	rec.AddAttributes(attrs...)
	// The record takes its trace from ctx. Like on stdout, only a recorded
	// trace is named: an unsampled one cannot be found in Jaeger.
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() && !sc.IsSampled() {
		ctx = trace.ContextWithSpanContext(ctx, trace.SpanContext{})
	}
	(*logger).Emit(ctx, rec)
}

func otelSeverity(s string) otellog.Severity {
	switch s {
	case "DEBUG":
		return otellog.SeverityDebug
	case "WARN":
		return otellog.SeverityWarn
	case "ERROR":
		return otellog.SeverityError
	case "FATAL":
		return otellog.SeverityFatal
	default:
		return otellog.SeverityInfo
	}
}

func logValue(v any) otellog.Value {
	switch x := v.(type) {
	case string:
		return otellog.StringValue(x)
	case bool:
		return otellog.BoolValue(x)
	case int:
		return otellog.IntValue(x)
	case int64:
		return otellog.Int64Value(x)
	case uint32:
		return otellog.Int64Value(int64(x))
	case uint64:
		return otellog.Int64Value(int64(x))
	case float64:
		return otellog.Float64Value(x)
	default:
		return otellog.StringValue(fmt.Sprint(x))
	}
}
