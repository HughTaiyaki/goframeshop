package telemetry

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const token = "eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJlLXZhbHVl"

func span(scope string, attrs []attribute.KeyValue, events []sdktrace.Event, status string) sdktrace.ReadOnlySpan {
	return tracetest.SpanStub{
		Name:                 "s",
		InstrumentationScope: instrumentation.Scope{Name: scope},
		Attributes:           attrs,
		Events:               events,
		Status:               sdktrace.Status{Code: codes.Error, Description: status},
	}.Snapshot()
}

func everything(s sdktrace.ReadOnlySpan) string {
	var b strings.Builder
	for _, kv := range s.Attributes() {
		b.WriteString(kv.Value.Emit() + "\n")
	}
	for _, e := range s.Events() {
		for _, kv := range e.Attributes {
			b.WriteString(kv.Value.Emit() + "\n")
		}
	}
	b.WriteString(s.Status().Description)
	return b.String()
}

func TestRedactRemovesSecrets(t *testing.T) {
	in := span("github.com/gogf/gf/v2/net/ghttp", []attribute.KeyValue{
		attribute.String("note", "called with Bearer "+token),
		attribute.Int("http.status_code", 200),
	}, []sdktrace.Event{
		{Name: "http.request", Attributes: []attribute.KeyValue{
			attribute.String("http.request.url", "/frontend/order?orderNo=O1"),
			attribute.String("http.request.headers", `{"Authorization":"Bearer `+token+`","Cookie":["a=1","b=2"],"X-Paysim-Signature":"c2ln","Content-Type":"application/json"}`),
		}},
		{Name: "grpc.request", Attributes: []attribute.KeyValue{
			attribute.String("grpc.metadata.outgoing", `{"authorization":["Bearer `+token+`"],"traceparent":["00-abc"]}`),
		}},
		{Name: "redis.execution", Attributes: []attribute.KeyValue{
			attribute.String("redis.execution.command", "SET"),
			attribute.String("redis.execution.arguments", `["goods:v1:detail:2","{\"name\":\"电视\"}"]`),
		}},
	}, "rpc failed: Bearer "+token+"\nstack line 1\nstack line 2")

	out := Redact(in)
	all := everything(out)
	for _, secret := range []string{token, "c2ln", "a=1", "电视", "stack line"} {
		if strings.Contains(all, secret) {
			t.Errorf("%q survived:\n%s", secret, all)
		}
	}
	// What is not secret survives, so the trace stays useful.
	for _, kept := range []string{"application/json", "traceparent", "SET", "/frontend/order?orderNo=O1", "rpc failed"} {
		if !strings.Contains(all, kept) {
			t.Errorf("%q was removed:\n%s", kept, all)
		}
	}
	if out.Attributes()[1].Value.AsInt64() != 200 {
		t.Error("non-string attribute changed")
	}
}

func TestRedactSQLErrors(t *testing.T) {
	out := Redact(span("github.com/gogf/gf/v2/database/gdb", nil, nil,
		"Error 1062 (23000): Duplicate entry '13800000000' for key 'uk_phone', INSERT INTO user_info(phone) VALUES('13800000000')"))
	if got := out.Status().Description; got != "database error: Error 1062 (23000)" {
		t.Fatalf("status %q", got)
	}
	out = Redact(span("github.com/gogf/gf/v2/database/gdb", nil, nil, "connection refused while querying address table"))
	if got := out.Status().Description; got != "database error" {
		t.Fatalf("status %q", got)
	}
}

func TestRedactSQLStatements(t *testing.T) {
	sql := "INSERT INTO `order_info`(`order_no`,`user_id`,`consignee_phone`,`consignee_address`,`remark`) " +
		"VALUES('O1',26,'13912345678','广东省东莞市 A101','it''s \\'quoted\\'')"
	out := Redact(span("github.com/gogf/gf/v2/database/gdb", []attribute.KeyValue{attribute.String("db.statement", sql)}, nil, ""))
	got := out.Attributes()[0].Value.AsString()
	want := "INSERT INTO `order_info`(`order_no`,`user_id`,`consignee_phone`,`consignee_address`,`remark`) VALUES('?',26,'?','?','?')"
	if got != want {
		t.Fatalf("statement\n got %s\nwant %s", got, want)
	}
}

func TestRedactUnparsableHeaders(t *testing.T) {
	out := Redact(span("x", nil, []sdktrace.Event{{Name: "e", Attributes: []attribute.KeyValue{
		attribute.String("http.request.headers", "map[Authorization:secret]"),
	}}}, ""))
	if v := out.Events()[0].Attributes[0].Value.AsString(); v != redacted {
		t.Fatalf("unparsable headers kept: %q", v)
	}
}

// The exporter hands only redacted spans to the next exporter.
func TestExporterRedactsBeforeExport(t *testing.T) {
	sink := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(&redactingExporter{next: sink}))
	_, s := provider.Tracer("t").Start(context.Background(), "op")
	s.SetAttributes(attribute.String("auth", "Bearer "+token))
	s.End()
	got := sink.GetSpans()
	if len(got) != 1 || strings.Contains(got[0].Attributes[0].Value.AsString(), token) {
		t.Fatalf("exported %+v", got)
	}
}

func TestQuietIsValidButNotSampled(t *testing.T) {
	sc := trace.SpanContextFromContext(Quiet(context.Background()))
	if !sc.IsValid() || sc.IsSampled() {
		t.Fatalf("quiet span context: valid=%v sampled=%v", sc.IsValid(), sc.IsSampled())
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())))
	_, child := provider.Tracer("t").Start(Quiet(context.Background()), "poll")
	if child.IsRecording() {
		t.Fatal("a child of a quiet context is recorded")
	}
}
