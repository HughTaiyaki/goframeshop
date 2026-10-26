package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/hughtaiyaki/goframeshop/utility/failpoint"
	"github.com/hughtaiyaki/goframeshop/utility/rabbitmq"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

const (
	outboxTable  = "outbox_event"
	statePending = "PENDING"
	stateSent    = "SENT"
)

// Append adds e to the outbox inside tx, the transaction of the business
// change it describes. It must not be called outside one.
func Append(ctx context.Context, tx gdb.TX, e Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	data := g.Map{
		"event_id":     uuid.NewString(),
		"type":         e.Type,
		"aggregate_id": e.AggregateID,
		"payload":      string(payload),
	}
	if e.DeliverAt != nil {
		data["deliver_at"] = gtime.New(*e.DeliverAt)
	}
	// Remember who wrote it: the relay publishes in that trace.
	if tp := traceparent(ctx); tp != "" {
		data["traceparent"] = tp
	}
	_, err = tx.Model(outboxTable).Ctx(ctx).Data(data).Insert()
	return err
}

// Publisher is what the relay needs from the transport.
type Publisher interface {
	Publish(ctx context.Context, m rabbitmq.Message) error
}

// RelaySettings come from config (outbox.*).
type RelaySettings struct {
	Interval time.Duration // how often to look for pending events
	Lease    time.Duration // how long a claimed batch is ours (design: 15s)
	Batch    int
}

// Relay publishes a database's PENDING outbox rows. Several instances may
// run at once: each claims a batch with a lease (owner, lease_until) in a
// short transaction, publishes outside it, and records each result only if
// it still owns the row.
type Relay struct {
	db       gdb.DB
	pub      Publisher
	settings RelaySettings
	owner    string
	now      func() time.Time
}

func NewRelay(db gdb.DB, pub Publisher, settings RelaySettings, now func() time.Time) *Relay {
	host, _ := os.Hostname()
	return &Relay{db: db, pub: pub, settings: settings, now: now,
		owner: fmt.Sprintf("%s/%d/%s", host, os.Getpid(), uuid.NewString()[:8])}
}

// Run publishes until ctx ends.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.settings.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.Pass(ctx); err != nil {
				g.Log().Warningf(ctx, "outbox relay pass failed: %v", err)
			}
		}
	}
}

type outboxRow struct {
	Id          uint64
	EventId     string
	Type        string
	AggregateId string
	Payload     string
	DeliverAt   *gtime.Time
	Traceparent string
	Attempts    int
	CreatedAt   *gtime.Time
}

// Pass claims one batch of due events, publishes them in id order and
// records each outcome. It returns how many were confirmed.
func (r *Relay) Pass(ctx context.Context) (int, error) {
	rows, err := r.claim(ctx)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	sent := 0
	for _, row := range rows {
		if r.publish(ctx, row) {
			sent++
		}
	}
	return sent, nil
}

// publish sends one event inside a Producer span. The span continues the
// trace of whoever wrote the event (its stored traceparent), so a trace shows
// the request, then this publish, then the consumer: even when the publish
// happens seconds later, or only after the broker came back.
func (r *Relay) publish(ctx context.Context, row *outboxRow) bool {
	parent := ctx
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.operation.type", "publish"),
			attribute.String("messaging.destination.name", rabbitmq.Exchange),
			attribute.String("messaging.rabbitmq.destination.routing_key", row.Type),
			attribute.String("messaging.message.id", row.EventId),
			attribute.String("event.aggregate_id", row.AggregateId),
			attribute.Int("event.attempt", row.Attempts+1),
		),
	}
	if row.Traceparent != "" {
		parent = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier{"traceparent": row.Traceparent})
	} else {
		opts = append(opts, trace.WithNewRoot())
	}
	ctx, span := otel.Tracer(telemetry.Instrumentation).Start(parent, "publish "+row.Type, opts...)
	defer span.End()

	envelope := Envelope{
		SchemaVersion:    SchemaVersion,
		EventID:          row.EventId,
		Type:             row.Type,
		AggregateID:      row.AggregateId,
		AggregateVersion: row.Id,
		OccurredAt:       row.CreatedAt.Time.UTC(),
		Traceparent:      traceparent(ctx),
		Payload:          json.RawMessage(row.Payload),
	}
	msg := rabbitmq.Message{RoutingKey: row.Type, MessageID: row.EventId}
	if row.DeliverAt != nil {
		at := row.DeliverAt.Time.UTC()
		envelope.DeliverAt = &at
		msg.Delayed = true
		msg.TTL = row.DeliverAt.Time.Sub(r.now())
		if msg.TTL == 0 {
			msg.TTL = -1 // due now: expire at once rather than never
		}
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		r.failed(ctx, span, row, err)
		return false
	}
	msg.Body = body
	// An error here is an unknown outcome: the broker may have the message.
	// It is sent again with the same event id later, and the consumer's
	// inbox absorbs the repeat.
	if err := r.pub.Publish(ctx, msg); err != nil {
		r.failed(ctx, span, row, err)
		return false
	}
	// Fault experiment T8: the broker has it, the row still says PENDING.
	_ = failpoint.Inject("eventbus.relay.after-publish")
	if err := r.mark(ctx, row.Id, g.Map{"state": stateSent, "sent_at": gtime.New(r.now()), "owner": nil, "lease_until": nil, "last_error": nil}); err != nil {
		// Published but not marked: it will be sent again. Harmless.
		g.Log().Warningf(ctx, "outbox event %s published, but marking it sent failed: %v", row.EventId, err)
		return false
	}
	g.Log().Debugf(ctx, "outbox event %s (%s %s) published", row.EventId, row.Type, row.AggregateId)
	return true
}

// traceparent is ctx's W3C trace context, or "" without one.
func traceparent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier["traceparent"]
}

// claim takes up to Batch due rows nobody holds, in one short transaction.
// SKIP LOCKED lets two relays claim different rows instead of waiting.
func (r *Relay) claim(ctx context.Context) ([]*outboxRow, error) {
	var rows []*outboxRow
	now := gtime.New(r.now())
	err := r.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		all, err := tx.GetAll(
			"SELECT id, event_id, type, aggregate_id, payload, deliver_at, IFNULL(traceparent, '') AS traceparent, attempts, created_at FROM "+outboxTable+
				" WHERE state = ? AND next_attempt_at <= ? AND (lease_until IS NULL OR lease_until < ?)"+
				" ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED",
			statePending, now, now, r.settings.Batch)
		if err != nil {
			return err
		}
		if err := all.Structs(&rows); err != nil || len(rows) == 0 {
			return err
		}
		ids := make([]uint64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.Id)
		}
		_, err = tx.Model(outboxTable).Ctx(ctx).
			Data(g.Map{"owner": r.owner, "lease_until": gtime.New(r.now().Add(r.settings.Lease))}).
			WhereIn("id", ids).Update()
		return err
	})
	return rows, err
}

func (r *Relay) failed(ctx context.Context, span trace.Span, row *outboxRow, cause error) {
	span.SetStatus(codes.Error, cause.Error())
	attempts := row.Attempts + 1
	next := r.now().Add(backoff(attempts))
	msg := cause.Error()
	if len(msg) > 255 {
		msg = msg[:255]
	}
	g.Log().Warningf(ctx, "outbox event %s (%s %s) attempt %d failed, retry at %s: %v",
		row.EventId, row.Type, row.AggregateId, attempts, next.Format(time.RFC3339), cause)
	if err := r.mark(ctx, row.Id, g.Map{
		"attempts": attempts, "next_attempt_at": gtime.New(next), "last_error": msg, "owner": nil, "lease_until": nil,
	}); err != nil {
		g.Log().Warningf(ctx, "outbox event %s: record failure: %v", row.EventId, err)
	}
}

// mark updates a row only while this relay still holds it: after the lease
// ran out another relay may own it, and its result wins.
func (r *Relay) mark(ctx context.Context, id uint64, data g.Map) error {
	_, err := r.db.Model(outboxTable).Ctx(ctx).Data(data).
		Where("id", id).Where("owner", r.owner).Update()
	return err
}

// backoff: 1s, 2s, 4s ... capped at 1 minute. Pending events never give up:
// the broker being down for an hour must not lose them.
func backoff(attempt int) time.Duration {
	if attempt > 6 {
		return time.Minute
	}
	return time.Second << (attempt - 1)
}

// Backlog reports the events of db not yet published: how many, and when
// the oldest was written (zero when there is none). It feeds the "outbox"
// step of trade_workflow_backlog (design §9.2).
func Backlog(ctx context.Context, db gdb.DB) (int64, time.Time, error) {
	row, err := db.GetOne(ctx, "SELECT COUNT(*) AS n, MIN(created_at) AS oldest FROM "+outboxTable+" WHERE state = ?", statePending)
	if err != nil {
		return 0, time.Time{}, err
	}
	var oldest time.Time
	if t := row["oldest"].GTime(); t != nil {
		oldest = t.Time
	}
	return row["n"].Int64(), oldest, nil
}
