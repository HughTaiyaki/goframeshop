// Package eventbus is the transactional outbox and inbox (design §8).
//
// Publishing: Append writes the event into the service's own outbox_event
// table inside the business transaction. The Relay later publishes PENDING
// rows and marks them SENT only after RabbitMQ confirms. Nothing is ever
// published from "after the commit" code, so a crash cannot lose an event;
// at worst it is sent twice.
//
// Consuming: Handle records (consumer, event_id) in inbox_event inside the
// same transaction as the business change. A redelivered event finds its row
// and is acknowledged without being applied again.
//
// Events are facts, not commands: a consumer checks the current state of
// whatever the event is about before acting on it.
package eventbus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// SchemaVersion of the envelope below.
const SchemaVersion = 1

// Envelope is what travels on the wire (design §7.3).
type Envelope struct {
	SchemaVersion int    `json:"schema_version"`
	EventID       string `json:"event_id"`
	Type          string `json:"type"`
	AggregateID   string `json:"aggregate_id"`
	// AggregateVersion is the outbox row id. Rows for one aggregate are
	// written while its row is locked, so a larger version is a later change.
	AggregateVersion uint64    `json:"aggregate_version"`
	OccurredAt       time.Time `json:"occurred_at"`
	// DeliverAt is set on delayed events: when they were meant to arrive.
	DeliverAt *time.Time `json:"deliver_at,omitempty"`
	// Traceparent is the W3C trace context of the publish (also in the
	// message headers), so the event can be followed back to its cause.
	Traceparent string          `json:"traceparent,omitempty"`
	Payload     json.RawMessage `json:"payload"`
}

// Event is what a service appends to its outbox.
type Event struct {
	// Type is also the routing key, e.g. "goods.changed".
	Type        string
	AggregateID string
	Payload     any
	// DeliverAt delays delivery until then (a TTL on the message); nil
	// delivers at once.
	DeliverAt *time.Time
}

// hash is the fingerprint of a delivery, to spot one event id reused for a
// different message.
func hash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
