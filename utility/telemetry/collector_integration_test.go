package telemetry_test

// The collector's second line of defence (deploy/otel/collector.yaml): a span
// sent WITHOUT the in-process redaction must still reach Jaeger without its
// secrets.
//
//	make infra-up
//	GOFRAMESHOP_IT=1 go test ./utility/telemetry/ -run Integration -v

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

func TestIntegrationCollectorScrubsUnredactedSpans(t *testing.T) {
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose collector and Jaeger")
	}
	ctx := context.Background()
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint("127.0.0.1:4317"), otlptracegrpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	service := "it-scrub-" + uuid.NewString()[:8]
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(resource.NewSchemaless(semconv.ServiceName(service))))
	secret := "s3cr3t-" + uuid.NewString()[:8]
	_, span := provider.Tracer("it").Start(ctx, "unredacted")
	span.SetAttributes(attribute.String("db.statement", "UPDATE user_info SET phone='"+secret+"' WHERE id=1"))
	span.AddEvent("http.request", trace.WithAttributes(
		attribute.String("http.request.headers", `{"Authorization":"Bearer `+secret+`","Content-Type":"application/json"}`)))
	span.AddEvent("grpc.request", trace.WithAttributes(
		attribute.String("grpc.metadata.outgoing", `{"authorization":["Bearer `+secret+`"]}`)))
	span.AddEvent("redis.execution", trace.WithAttributes(
		attribute.String("redis.execution.command", "SET"),
		attribute.String("redis.execution.arguments", `["k","`+secret+`"]`)))
	span.End()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}

	var body string
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	for i := 0; i < 30 && !strings.Contains(body, "unredacted"); i++ {
		time.Sleep(500 * time.Millisecond)
		res, err := client.Get("http://127.0.0.1:16686/api/traces?lookback=1h&service=" + service)
		if err != nil {
			continue
		}
		var out struct{ Data json.RawMessage }
		json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		body = string(out.Data)
	}
	if !strings.Contains(body, "unredacted") {
		t.Fatal("span never reached Jaeger")
	}
	if strings.Contains(body, secret) {
		t.Fatalf("secret reached Jaeger: %s", body)
	}
	for _, kept := range []string{"UPDATE user_info SET phone='?'", "application/json", "SET"} {
		if !strings.Contains(body, kept) {
			t.Errorf("%q missing: the scrub removed too much", kept)
		}
	}
}
