package eventbus_test

// Integration tests against the Compose MySQL (orders database) and RabbitMQ.
//
//	make infra-up && make db-init
//	GOFRAMESHOP_IT=1 go test ./utility/eventbus/ -v
//
// Each test uses its own routing keys ("it.<random>") bound to its own
// temporary queues, and deletes its outbox, inbox and quarantine rows.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"github.com/hughtaiyaki/goframeshop/utility/ittest"
	"github.com/hughtaiyaki/goframeshop/utility/rabbitmq"
)

type env struct {
	ctx context.Context
	db  gdb.DB
	url string
	ch  *amqp.Channel
	key string // this test's routing key prefix
}

func setup(t *testing.T) *env {
	t.Helper()
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose MySQL and RabbitMQ")
	}
	link := ittest.MySQL(t, "orders")
	if err := gdb.SetConfig(gdb.Config{"default": gdb.ConfigGroup{{Link: link, MaxOpenConnCount: 32}}}); err != nil {
		t.Fatal(err)
	}
	db, err := gdb.Instance()
	if err != nil {
		t.Fatal(err)
	}
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = ittest.RabbitMQ(t)
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatalf("cannot reach RabbitMQ: %v", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	if err := rabbitmq.Declare(ch); err != nil {
		t.Fatal(err)
	}
	e := &env{ctx: context.Background(), db: db, url: url, ch: ch, key: "it." + uuid.NewString()[:8]}
	t.Cleanup(func() {
		db.Exec(e.ctx, "DELETE FROM outbox_event WHERE type LIKE ?", e.key+"%")
		db.Exec(e.ctx, "DELETE FROM inbox_event WHERE consumer LIKE ?", e.key+"%")
		db.Exec(e.ctx, "DELETE FROM quarantine_event WHERE consumer LIKE ?", e.key+"%")
		conn.Close()
	})
	return e
}

// queue declares a temporary queue bound to key and returns its deliveries.
func (e *env) queue(t *testing.T, key string, args amqp.Table) <-chan amqp.Delivery {
	t.Helper()
	q, err := e.ch.QueueDeclare("", false, true, true, false, args)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.ch.QueueBind(q.Name, key, rabbitmq.Exchange, false, nil); err != nil {
		t.Fatal(err)
	}
	if args != nil {
		return nil // a delay queue: nobody consumes it
	}
	d, err := e.ch.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func receive(t *testing.T, d <-chan amqp.Delivery, within time.Duration) *amqp.Delivery {
	t.Helper()
	select {
	case m := <-d:
		return &m
	case <-time.After(within):
		return nil
	}
}

func TestIntegrationPublisher(t *testing.T) {
	e := setup(t)
	pub := rabbitmq.NewPublisher(e.url, 5*time.Second)
	defer pub.Close()

	ok := e.queue(t, e.key+".ok", nil)
	if err := pub.Publish(e.ctx, rabbitmq.Message{RoutingKey: e.key + ".ok", MessageID: "m1", Body: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if m := receive(t, ok, 3*time.Second); m == nil || m.MessageId != "m1" || m.DeliveryMode != amqp.Persistent {
		t.Fatalf("delivery: %+v", m)
	}
	// Nobody bound: the broker returns it, and Publish says so.
	err := pub.Publish(e.ctx, rabbitmq.Message{RoutingKey: e.key + ".nobody", MessageID: "m2", Body: []byte(`{}`)})
	if !errors.Is(err, rabbitmq.ErrUnroutable) {
		t.Fatalf("unroutable: %v", err)
	}
	// The channel is still usable after a return.
	if err := pub.Publish(e.ctx, rabbitmq.Message{RoutingKey: e.key + ".ok", MessageID: "m3", Body: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	receive(t, ok, 3*time.Second)

	// A delayed message: parked in a queue nobody reads, dead-lettered to
	// the due key when its TTL runs out.
	e.queue(t, e.key+".later", amqp.Table{"x-dead-letter-exchange": rabbitmq.Exchange, "x-dead-letter-routing-key": e.key + ".due"})
	due := e.queue(t, e.key+".due", nil)
	start := time.Now()
	if err := pub.Publish(e.ctx, rabbitmq.Message{RoutingKey: e.key + ".later", MessageID: "m4", Body: []byte(`{}`), TTL: 700 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	m := receive(t, due, 5*time.Second)
	if m == nil || m.MessageId != "m4" {
		t.Fatalf("delayed message never came: %+v", m)
	}
	if waited := time.Since(start); waited < 600*time.Millisecond {
		t.Fatalf("delivered after %s, before its TTL", waited)
	}
}

func appendEvents(t *testing.T, e *env, typ string, n int) {
	t.Helper()
	err := e.db.Transaction(e.ctx, func(ctx context.Context, tx gdb.TX) error {
		for i := 0; i < n; i++ {
			if err := eventbus.Append(ctx, tx, eventbus.Event{Type: typ, AggregateID: fmt.Sprint(i), Payload: map[string]int{"n": i}}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, e *env, where string, args ...any) int {
	t.Helper()
	v, err := e.db.GetValue(e.ctx, "SELECT COUNT(*) FROM outbox_event WHERE "+where, args...)
	if err != nil {
		t.Fatal(err)
	}
	return v.Int()
}

func TestIntegrationRelay(t *testing.T) {
	e := setup(t)
	pub := rabbitmq.NewPublisher(e.url, 5*time.Second)
	defer pub.Close()
	settings := eventbus.RelaySettings{Interval: time.Second, Lease: 15 * time.Second, Batch: 100}

	// A rolled-back transaction leaves no event behind.
	_ = e.db.Transaction(e.ctx, func(ctx context.Context, tx gdb.TX) error {
		eventbus.Append(ctx, tx, eventbus.Event{Type: e.key + ".a", AggregateID: "x", Payload: 1})
		return errors.New("business step failed")
	})
	if n := count(t, e, "type = ?", e.key+".a"); n != 0 {
		t.Fatalf("%d events survived a rollback", n)
	}

	// Two relays race over 60 events: every event is published exactly once.
	got := e.queue(t, e.key+".a", nil)
	appendEvents(t, e, e.key+".a", 60)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			relay := eventbus.NewRelay(e.db, rabbitmq.NewPublisher(e.url, 5*time.Second), eventbus.RelaySettings{Interval: time.Second, Lease: 15 * time.Second, Batch: 7}, time.Now)
			for j := 0; j < 10; j++ {
				if _, err := relay.Pass(e.ctx); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	seen := map[string]bool{}
	for {
		m := receive(t, got, time.Second)
		if m == nil {
			break
		}
		var env eventbus.Envelope
		if err := json.Unmarshal(m.Body, &env); err != nil || env.EventID != m.MessageId || env.SchemaVersion != 1 || env.AggregateVersion == 0 {
			t.Fatalf("envelope %s: %v", m.Body, err)
		}
		if seen[env.EventID] {
			t.Fatalf("event %s published twice", env.EventID)
		}
		seen[env.EventID] = true
	}
	if len(seen) != 60 || count(t, e, "type = ? AND state = 'SENT' AND owner IS NULL", e.key+".a") != 60 {
		t.Fatalf("published %d, sent rows %d", len(seen), count(t, e, "type = ? AND state = 'SENT'", e.key+".a"))
	}

	// An event nobody listens to stays PENDING with a retry scheduled.
	appendEvents(t, e, e.key+".unbound", 1)
	relay := eventbus.NewRelay(e.db, pub, settings, time.Now)
	if _, err := relay.Pass(e.ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, e, "type = ? AND state = 'PENDING' AND attempts = 1 AND next_attempt_at > NOW(3) AND owner IS NULL AND last_error LIKE '%no queue%'", e.key+".unbound") != 1 {
		t.Fatal("failed publish not recorded for retry")
	}
	// Not due yet: the next pass leaves it alone.
	relay.Pass(e.ctx)
	if count(t, e, "type = ? AND attempts = 1", e.key+".unbound") != 1 {
		t.Fatal("retried before its backoff")
	}
}

func TestIntegrationInbox(t *testing.T) {
	e := setup(t)
	consumer := e.key + ".consumer"
	body := func(id, payload string) []byte {
		b, _ := json.Marshal(eventbus.Envelope{SchemaVersion: 1, EventID: id, Type: "it", AggregateID: "1", AggregateVersion: 1, Payload: json.RawMessage(payload)})
		return b
	}
	applied := 0
	apply := func(ctx context.Context, tx gdb.TX, ev *eventbus.Envelope) error { applied++; return nil }

	id := uuid.NewString()
	for i := 0; i < 3; i++ {
		if _, err := eventbus.Handle(e.ctx, e.db, consumer, body(id, `{"a":1}`), apply); err != nil {
			t.Fatal(err)
		}
	}
	if applied != 1 {
		t.Fatalf("applied %d times", applied)
	}
	// Same id, different payload: quarantined and acknowledged.
	if _, err := eventbus.Handle(e.ctx, e.db, consumer, body(id, `{"a":2}`), apply); err != nil || applied != 1 {
		t.Fatalf("conflict: %v, applied %d", err, applied)
	}
	// Garbage: quarantined and acknowledged.
	if _, err := eventbus.Handle(e.ctx, e.db, consumer, []byte("not json"), apply); err != nil {
		t.Fatal(err)
	}
	q, _ := e.db.GetValue(e.ctx, "SELECT COUNT(*) FROM quarantine_event WHERE consumer = ?", consumer)
	if q.Int() != 2 {
		t.Fatalf("quarantined %d", q.Int())
	}
	// A failing step rolls back the inbox row too: the next delivery applies.
	other := uuid.NewString()
	if _, err := eventbus.Handle(e.ctx, e.db, consumer, body(other, `{}`), func(context.Context, gdb.TX, *eventbus.Envelope) error {
		return errors.New("database hiccup")
	}); err == nil {
		t.Fatal("failure acknowledged")
	}
	if ok, err := eventbus.Handle(e.ctx, e.db, consumer, body(other, `{}`), apply); err != nil || !ok || applied != 2 {
		t.Fatalf("redelivery after failure: %v %v applied %d", ok, err, applied)
	}
	// Concurrent copies of one event: applied once.
	race := uuid.NewString()
	var wg sync.WaitGroup
	var mu sync.Mutex
	n := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			eventbus.Handle(e.ctx, e.db, consumer, body(race, `{}`), func(context.Context, gdb.TX, *eventbus.Envelope) error {
				mu.Lock()
				n++
				mu.Unlock()
				return nil
			})
		}()
	}
	wg.Wait()
	if n != 1 {
		t.Fatalf("concurrent copies applied %d times", n)
	}
}

// The trace of whoever wrote an event reaches the message: the relay
// publishes in a span of that trace and passes it on in the headers and the
// envelope.
func TestIntegrationTraceContextTravelsWithTheEvent(t *testing.T) {
	e := setup(t)
	sink := tracetest.NewInMemoryExporter()
	oldProvider, oldProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(sink)))
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTracerProvider(oldProvider); otel.SetTextMapPropagator(oldProp) })

	got := e.queue(t, e.key+".traced", nil)
	ctx, request := otel.Tracer("t").Start(e.ctx, "request")
	err := e.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return eventbus.Append(ctx, tx, eventbus.Event{Type: e.key + ".traced", AggregateID: "1", Payload: 1})
	})
	request.End()
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := e.db.GetValue(e.ctx, "SELECT traceparent FROM outbox_event WHERE type = ?", e.key+".traced")
	if !strings.Contains(stored.String(), request.SpanContext().TraceID().String()) {
		t.Fatalf("stored traceparent %q", stored)
	}

	relay := eventbus.NewRelay(e.db, rabbitmq.NewPublisher(e.url, 5*time.Second), eventbus.RelaySettings{Interval: time.Second, Lease: 15 * time.Second, Batch: 10}, time.Now)
	if _, err := relay.Pass(e.ctx); err != nil {
		t.Fatal(err)
	}
	m := receive(t, got, 3*time.Second)
	if m == nil {
		t.Fatal("not delivered")
	}
	header, _ := m.Headers["traceparent"].(string)
	var env eventbus.Envelope
	json.Unmarshal(m.Body, &env)
	trace := request.SpanContext().TraceID().String()
	if !strings.Contains(header, trace) || !strings.Contains(env.Traceparent, trace) {
		t.Fatalf("header %q envelope %q, want trace %s", header, env.Traceparent, trace)
	}
	var publish bool
	for _, s := range sink.GetSpans() {
		if s.Name == "publish "+e.key+".traced" && s.SpanKind == oteltrace.SpanKindProducer && s.Parent.TraceID().String() == trace {
			publish = true
		}
	}
	if !publish {
		t.Fatal("no producer span in the request's trace")
	}
}
