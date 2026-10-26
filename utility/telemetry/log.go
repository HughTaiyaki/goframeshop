package telemetry

// Structured logs (design §9.1, task 0019). One JSON object per line with a
// fixed set of fields, so a reader (a person with jq, or the diagnosis
// agent) can filter by service, trace, order or event instead of guessing
// from text. The handler is installed for every GoFrame logger, so the
// existing g.Log() calls and GoFrame's own logs all come out in this format.

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"go.opentelemetry.io/otel/trace"
)

// Business events (design §9.1). The names are a contract with whatever
// reads the logs: add new ones, never rename.
const (
	EventOrderTransition      = "order.transition"
	EventInventoryReserved    = "inventory.reserved"
	EventInventoryReleased    = "inventory.released"
	EventPaymentObserved      = "payment.observed"
	EventPaymentRefundRequest = "payment.refund.requested"
	EventMessageRedelivered   = "message.redelivered"
	EventWorkflowRetry        = "workflow.retry"
)

// Fields are the structured parts of a business event. OrderID,
// OperationID and ErrorType are top-level contract fields; Attributes holds
// what is specific to the event, and must only ever carry ids, states,
// error codes and counts: never a token, credential, phone number or
// address.
type Fields struct {
	OrderID     string
	OperationID string
	ErrorType   string
	Attributes  map[string]any
}

type eventKey struct{}

type event struct {
	name   string
	fields Fields
}

// Event logs a business event: INFO, or WARN when f.ErrorType is set.
func Event(ctx context.Context, name string, f Fields, format string, args ...any) {
	ctx = context.WithValue(ctx, eventKey{}, &event{name: name, fields: f})
	if f.ErrorType != "" {
		g.Log().Warningf(ctx, format, args...)
		return
	}
	g.Log().Infof(ctx, format, args...)
}

// logRecord is one line. Field order and names follow design §9.1; the
// contract fields are always present, empty when unknown.
type logRecord struct {
	Timestamp   string         `json:"timestamp"`
	Severity    string         `json:"severity"`
	Service     string         `json:"service.name"`
	Version     string         `json:"service.version"`
	Environment string         `json:"deployment.environment.name"`
	EventName   string         `json:"event.name"`
	TraceID     string         `json:"trace_id"`
	SpanID      string         `json:"span_id"`
	OperationID string         `json:"operation_id"`
	OrderID     string         `json:"order_id"`
	ErrorType   string         `json:"error.type"`
	Message     string         `json:"message"`
	Attributes  map[string]any `json:"attributes,omitempty"`
	Stack       string         `json:"error.stack,omitempty"`
}

type logIdentity struct {
	service, version, environment string
}

// InstallLogging makes every GoFrame logger write JSON lines for service,
// unless logger.format is "text". Start calls it first, so even the
// process's earliest lines are structured.
func InstallLogging(ctx context.Context, service string) {
	if strings.EqualFold(g.Cfg().MustGet(ctx, "logger.format", "json").String(), "text") {
		return
	}
	id := logIdentity{
		service:     service,
		version:     version(),
		environment: environment(ctx),
	}
	glog.SetDefaultHandler(id.handle)
}

// JSONHandler is the handler InstallLogging installs, for a given identity.
// Tests use it to capture what a service would write.
func JSONHandler(service, version, environment string) glog.Handler {
	return logIdentity{service: service, version: version, environment: environment}.handle
}

func (id logIdentity) handle(ctx context.Context, in *glog.HandlerInput) {
	record := id.record(ctx, in)
	in.Buffer.Write(encode(record))
	exportLog(ctx, in.Time, record)
	in.Next(ctx)
}

func (id logIdentity) format(ctx context.Context, in *glog.HandlerInput) []byte {
	return encode(id.record(ctx, in))
}

func (id logIdentity) record(ctx context.Context, in *glog.HandlerInput) *logRecord {
	message := in.Content
	if len(in.Values) > 0 {
		if message != "" {
			message += " "
		}
		message += in.ValuesContent()
	}
	record := logRecord{
		Timestamp:   in.Time.Format("2006-01-02T15:04:05.000Z07:00"),
		Severity:    severity(in.Level),
		Service:     id.service,
		Version:     id.version,
		Environment: id.environment,
		Message:     scrubLog(strings.TrimRight(message, "\n")),
		Stack:       strings.TrimSpace(in.Stack),
	}
	// Only a trace that is really recorded: an unsampled one (a background
	// poll) cannot be found in Jaeger, and naming it would fake a trace.
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() && sc.IsSampled() {
		record.TraceID, record.SpanID = sc.TraceID().String(), sc.SpanID().String()
	}
	if e, ok := ctx.Value(eventKey{}).(*event); ok {
		record.EventName = e.name
		record.OrderID = e.fields.OrderID
		record.OperationID = e.fields.OperationID
		record.ErrorType = e.fields.ErrorType
		record.Attributes = e.fields.Attributes
	}
	return &record
}

func encode(record *logRecord) []byte {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(record); err != nil {
		// Attributes that cannot be encoded: keep the line, drop them.
		record.Attributes = nil
		buf.Reset()
		_ = encoder.Encode(record)
	}
	return buf.Bytes()
}

func severity(level int) string {
	switch {
	case level >= glog.LEVEL_PANI:
		return "FATAL"
	case level >= glog.LEVEL_ERRO:
		return "ERROR"
	case level >= glog.LEVEL_WARN:
		return "WARN"
	case level >= glog.LEVEL_INFO:
		return "INFO"
	default:
		return "DEBUG"
	}
}

var (
	// scheme://user:password@host, as in amqp:// and redis:// URLs.
	urlCredentialPattern = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^\s/@:]+:[^\s/@]+@`)
	// GoFrame's MySQL link, mysql:user:password@tcp(...).
	dsnCredentialPattern = regexp.MustCompile(`(mysql|mariadb|pgsql|mssql|sqlite|oracle|clickhouse):[^\s@:]+:[^\s@]*@`)
	// A mainland China mobile number, not part of a longer digit run.
	phonePattern = regexp.MustCompile(`(^|[^0-9])1[3-9][0-9]{9}([^0-9]|$)`)
)

// scrubLog removes credentials and phone numbers from a log message. Code
// must not log them in the first place; this is the safety net.
func scrubLog(s string) string {
	s = scrub(s)
	s = urlCredentialPattern.ReplaceAllString(s, "${1}"+redacted+"@")
	s = dsnCredentialPattern.ReplaceAllString(s, "${1}:"+redacted+"@")
	return phonePattern.ReplaceAllString(s, "${1}"+redacted+"${2}")
}
