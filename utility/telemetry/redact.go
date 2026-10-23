package telemetry

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// What GoFrame records that must not reach the trace backend:
//
//   - every HTTP request/response header (Authorization, Cookie, the payment
//     simulator's signature) in the span events "http.request" / "http.response";
//   - every gRPC metadata entry, including the forwarded authorization, in
//     "grpc.metadata.incoming" / "grpc.metadata.outgoing";
//   - the arguments of every Redis command: cache keys and cached values;
//   - every SQL statement with its values filled in ("db.statement"):
//     names, phone numbers, addresses, password hashes, event payloads;
//   - "%+v" of a failed SQL statement as the span status, which can include
//     the statement with its values (names, phone numbers, addresses).
//
// redactingExporter rewrites spans just before export. The collector has a
// second, coarser filter (deploy/otel/collector.yaml).

const redacted = "[REDACTED]"

// headerKeys are the header and metadata names whose values are secrets,
// lower-cased.
var headerKeys = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-paysim-signature":  true,
}

// mapAttributes hold a JSON object of headers or metadata.
var mapAttributes = map[attribute.Key]bool{
	"http.request.headers":   true,
	"http.response.headers":  true,
	"grpc.metadata.incoming": true,
	"grpc.metadata.outgoing": true,
}

// sqlAttributes hold SQL text; their string literals are masked.
var sqlAttributes = map[attribute.Key]bool{
	"db.statement":  true,
	"db.query.text": true,
}

// dropAttributes are replaced wholesale.
var dropAttributes = map[attribute.Key]bool{
	"redis.execution.arguments": true,
}

var (
	// Anything that still looks like a credential, wherever it appears.
	bearerPattern = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)
	jwtPattern    = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	// A SQL string literal: '...' with '' or \' inside.
	sqlStringPattern = regexp.MustCompile(`'(?:[^'\\]|\\.|'')*'`)
	// The MySQL error number, the only part of a SQL error worth keeping.
	mysqlErrorPattern = regexp.MustCompile(`Error \d+( \([0-9A-Z]+\))?`)
)

type redactingExporter struct {
	next sdktrace.SpanExporter
}

func (e *redactingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	out := make([]sdktrace.ReadOnlySpan, 0, len(spans))
	for _, s := range spans {
		out = append(out, Redact(s))
	}
	return e.next.ExportSpans(ctx, out)
}

func (e *redactingExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}

// Redact returns a copy of s without secrets.
func Redact(s sdktrace.ReadOnlySpan) sdktrace.ReadOnlySpan {
	stub := tracetest.SpanStubFromReadOnlySpan(s)
	stub.Attributes = redactAttributes(stub.Attributes)
	for i := range stub.Events {
		stub.Events[i].Attributes = redactAttributes(stub.Events[i].Attributes)
	}
	stub.Status.Description = redactStatus(stub.InstrumentationScope.Name, stub.Status.Description)
	return stub.Snapshot()
}

func redactAttributes(attrs []attribute.KeyValue) []attribute.KeyValue {
	if len(attrs) == 0 {
		return attrs
	}
	out := make([]attribute.KeyValue, len(attrs))
	for i, kv := range attrs {
		switch {
		case dropAttributes[kv.Key]:
			out[i] = attribute.String(string(kv.Key), redacted)
		case sqlAttributes[kv.Key]:
			out[i] = attribute.String(string(kv.Key), sqlStringPattern.ReplaceAllString(kv.Value.AsString(), "'?'"))
		case mapAttributes[kv.Key]:
			out[i] = attribute.String(string(kv.Key), redactHeaderMap(kv.Value.AsString()))
		case kv.Value.Type() == attribute.STRING:
			out[i] = attribute.String(string(kv.Key), scrub(kv.Value.AsString()))
		default:
			out[i] = kv
		}
	}
	return out
}

// redactHeaderMap masks secret entries of a JSON object of headers. Text
// that is not such an object is withheld entirely: better to lose it than
// to guess.
func redactHeaderMap(text string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		if text == "" || text == "{}" {
			return text
		}
		return redacted
	}
	for k := range m {
		if headerKeys[strings.ToLower(k)] {
			m[k] = redacted
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return redacted
	}
	return scrub(string(b))
}

func scrub(s string) string {
	s = bearerPattern.ReplaceAllString(s, "Bearer "+redacted)
	return jwtPattern.ReplaceAllString(s, redacted)
}

// redactStatus keeps a status readable without its payload: SQL errors keep
// only the MySQL error number, everything else only its first line.
func redactStatus(scope, description string) string {
	if description == "" {
		return ""
	}
	if strings.Contains(scope, "database/gdb") {
		if m := mysqlErrorPattern.FindString(description); m != "" {
			return "database error: " + m
		}
		return "database error"
	}
	if i := strings.IndexByte(description, '\n'); i >= 0 {
		description = description[:i]
	}
	if len(description) > 300 {
		description = description[:300]
	}
	return scrub(description)
}
