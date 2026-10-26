package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"go.opentelemetry.io/otel/trace"
)

var testIdentity = logIdentity{service: "order-service", version: "abc123", environment: "experiment"}

// captureDefault points g.Log() at a buffer through the JSON handler, the
// way InstallLogging does, and restores it afterwards.
func captureDefault(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger := g.Log()
	logger.SetWriter(&buf)
	logger.SetStdoutPrint(false)
	logger.SetHandlers(testIdentity.handle)
	t.Cleanup(func() {
		logger.SetWriter(nil)
		logger.SetStdoutPrint(true)
		logger.SetHandlers()
	})
	return &buf
}

func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("not JSON: %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

var contractFields = []string{
	"timestamp", "severity", "service.name", "service.version", "deployment.environment.name",
	"event.name", "trace_id", "span_id", "operation_id", "order_id", "error.type", "message",
}

func TestEveryLineHasTheContractFields(t *testing.T) {
	buf := captureDefault(t)
	g.Log().Infof(context.Background(), "order %s: paid", "O1")
	got := lines(t, buf)
	if len(got) != 1 {
		t.Fatalf("%d lines: %s", len(got), buf)
	}
	line := got[0]
	for _, field := range contractFields {
		if _, ok := line[field]; !ok {
			t.Errorf("field %q missing in %v", field, line)
		}
	}
	if line["message"] != "order O1: paid" || line["severity"] != "INFO" || line["service.name"] != "order-service" ||
		line["service.version"] != "abc123" || line["deployment.environment.name"] != "experiment" {
		t.Fatalf("line %v", line)
	}
	// Unknown correlation fields are empty, not absent and not invented.
	for _, field := range []string{"event.name", "trace_id", "span_id", "operation_id", "order_id", "error.type"} {
		if line[field] != "" {
			t.Errorf("%s = %v, want empty", field, line[field])
		}
	}
	if !strings.Contains(line["timestamp"].(string), "T") {
		t.Errorf("timestamp %v is not RFC 3339", line["timestamp"])
	}
}

func TestOnlyRecordedTracesAreNamed(t *testing.T) {
	buf := captureDefault(t)
	tid, _ := trace.TraceIDFromHex("5c087640125dd9a983f0f59709cf974a")
	sid, _ := trace.SpanIDFromHex("1d2e3f4a5b6c7d8e")
	sampled := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled,
	}))
	g.Log().Info(sampled, "recorded")
	// Quiet gives a valid but unsampled trace: Jaeger never sees it.
	g.Log().Info(Quiet(context.Background()), "poll")
	got := lines(t, buf)
	if got[0]["trace_id"] != tid.String() || got[0]["span_id"] != sid.String() {
		t.Fatalf("sampled: %v", got[0])
	}
	if got[1]["trace_id"] != "" || got[1]["span_id"] != "" {
		t.Fatalf("unsampled trace was named: %v", got[1])
	}
}

func TestSeverityAndMessageForms(t *testing.T) {
	buf := captureDefault(t)
	ctx := context.Background()
	g.Log().Debug(ctx, "d")
	g.Log().Info(ctx, "several", "values", 3)
	g.Log().Notice(ctx, "n")
	g.Log().Warningf(ctx, "w %d", 1)
	g.Log().Errorf(ctx, "e: %v", errors.New("boom"))
	got := lines(t, buf)
	want := []struct{ severity, message string }{
		{"DEBUG", "d"}, {"INFO", "several values 3"}, {"INFO", "n"}, {"WARN", "w 1"}, {"ERROR", "e: boom"},
	}
	if len(got) != len(want) {
		t.Fatalf("%d lines: %s", len(got), buf)
	}
	for i, w := range want {
		if got[i]["severity"] != w.severity || got[i]["message"] != w.message {
			t.Errorf("line %d = %v %q, want %v %q", i, got[i]["severity"], got[i]["message"], w.severity, w.message)
		}
	}
	if stack, _ := got[4]["error.stack"].(string); !strings.Contains(stack, "log_test.go") {
		t.Errorf("error line has no stack: %v", got[4])
	}
}

func TestEventFields(t *testing.T) {
	buf := captureDefault(t)
	ctx := context.Background()
	Event(ctx, EventOrderTransition, Fields{
		OrderID: "O1", Attributes: map[string]any{"from": "PENDING_PAYMENT", "to": "PAYMENT_CONFIRMING"},
	}, "order %s %s -> %s", "O1", "PENDING_PAYMENT", "PAYMENT_CONFIRMING")
	Event(ctx, EventPaymentRefundRequest, Fields{OrderID: "O2", ErrorType: "Unavailable"}, "refund failed")
	got := lines(t, buf)
	if got[0]["event.name"] != EventOrderTransition || got[0]["order_id"] != "O1" || got[0]["severity"] != "INFO" {
		t.Fatalf("transition: %v", got[0])
	}
	attrs := got[0]["attributes"].(map[string]any)
	if attrs["from"] != "PENDING_PAYMENT" || attrs["to"] != "PAYMENT_CONFIRMING" {
		t.Fatalf("attributes: %v", attrs)
	}
	if got[1]["error.type"] != "Unavailable" || got[1]["severity"] != "WARN" {
		t.Fatalf("failed event: %v", got[1])
	}
	// A plain line after an event carries none of its fields.
	g.Log().Info(ctx, "plain")
	if after := lines(t, buf)[2]; after["event.name"] != "" || after["order_id"] != "" || after["attributes"] != nil {
		t.Fatalf("event fields leaked: %v", after)
	}
}

func TestSecretsAreScrubbed(t *testing.T) {
	buf := captureDefault(t)
	ctx := context.Background()
	jwt := "eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiI0MiJ9.c2lnbmF0dXJlLWJ5dGVz"
	for _, msg := range []string{
		"Authorization: Bearer " + jwt,
		"token " + jwt,
		"dial amqp://shop:s3cret-pass@127.0.0.1:5672/ failed",
		"redis://default:s3cret-pass@127.0.0.1:6379/1",
		"open mysql:shop:s3cret-pass@tcp(127.0.0.1:3306)/orders?loc=Local",
		"contact 13800138000 now",
		"phone:13912345678",
	} {
		g.Log().Warning(ctx, msg)
	}
	out := buf.String()
	for _, secret := range []string{jwt, "s3cret-pass", "13800138000", "13912345678"} {
		if strings.Contains(out, secret) {
			t.Errorf("%q survived: %s", secret, out)
		}
	}
	// What is not a secret stays readable.
	for _, kept := range []string{"amqp://[REDACTED]@127.0.0.1:5672/", "mysql:[REDACTED]@tcp(127.0.0.1:3306)/orders", "dial", "contact"} {
		if !strings.Contains(out, kept) {
			t.Errorf("%q missing: %s", kept, out)
		}
	}
	// Order numbers and longer digit runs are not phone numbers.
	buf.Reset()
	g.Log().Info(ctx, "order O2026102316342440826703 total 13800138000999")
	if !strings.Contains(buf.String(), "O2026102316342440826703") || !strings.Contains(buf.String(), "13800138000999") {
		t.Fatalf("over-scrubbed: %s", buf)
	}
}

func TestTextFormatIsLeftAlone(t *testing.T) {
	var buf bytes.Buffer
	logger := glog.New()
	logger.SetWriter(&buf)
	logger.SetStdoutPrint(false)
	logger.Info(context.Background(), "plain text")
	if strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Fatalf("a logger without the handler wrote JSON: %s", buf.String())
	}
}
