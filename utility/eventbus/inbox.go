package eventbus

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/hughtaiyaki/goframeshop/utility/failpoint"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

const (
	inboxTable      = "inbox_event"
	quarantineTable = "quarantine_event"
)

// Apply is the business step of a consumer. It runs inside the inbox
// transaction: whatever it writes through tx commits together with the
// "seen" record. Returning an error rolls both back and the message is
// delivered again, so only return one for failures worth retrying (the
// database is down); a business refusal ("the order is already paid") is
// a successful outcome and returns nil.
type Apply func(ctx context.Context, tx gdb.TX, e *Envelope) error

// Handle processes one delivery for consumer. nil means "acknowledge":
// applied now, applied before, or quarantined because it can never be
// applied. An error means "deliver again later".
//
// The second return value reports whether Apply ran and committed now.
func Handle(ctx context.Context, db gdb.DB, consumer string, body []byte, apply Apply) (bool, error) {
	var e Envelope
	if err := json.Unmarshal(body, &e); err != nil || e.EventID == "" || e.Type == "" || e.SchemaVersion != SchemaVersion {
		return false, quarantine(ctx, db, consumer, e.EventID, "malformed envelope", body)
	}
	fingerprint := contentHash(&e)
	applied := false
	conflict := false
	err := db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		result, err := tx.Model(inboxTable).Ctx(ctx).Data(g.Map{
			"consumer": consumer, "event_id": e.EventID, "payload_hash": fingerprint,
		}).InsertIgnore()
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n == 0 {
			if err != nil {
				return err
			}
			// Seen before (a concurrent copy waited on our insert's lock).
			seen, err := tx.Model(inboxTable).Ctx(ctx).Fields("payload_hash").
				Where("consumer", consumer).Where("event_id", e.EventID).Value()
			if err != nil {
				return err
			}
			// Rows written before task 0021 hold the hash of the whole body.
			conflict = seen.String() != fingerprint && seen.String() != hash(body)
			return nil
		}
		if err := apply(ctx, tx, &e); err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if applied {
		// Fault experiment T7: committed, not yet acknowledged.
		_ = failpoint.Inject("eventbus.inbox.after-commit")
	}
	if conflict {
		return false, quarantine(ctx, db, consumer, e.EventID, "event id reused with a different payload", body)
	}
	if !applied {
		telemetry.Event(ctx, telemetry.EventMessageRedelivered, telemetry.Fields{Attributes: map[string]any{
			"consumer": consumer, "event_id": e.EventID, "event_type": e.Type,
		}}, "consumer %s: event %s already processed; acknowledging", consumer, e.EventID)
	}
	return applied, nil
}

// quarantine keeps a message that can never be processed. If even that
// fails, the error makes the message stay in the queue.
func quarantine(ctx context.Context, db gdb.DB, consumer, eventID, reason string, body []byte) error {
	g.Log().Errorf(ctx, "consumer %s: event %q quarantined: %s", consumer, eventID, reason)
	if len(eventID) > 64 {
		eventID = eventID[:64]
	}
	_, err := db.Model(quarantineTable).Ctx(ctx).Data(g.Map{
		"consumer": consumer, "event_id": eventID, "reason": reason, "body": body,
	}).Insert()
	return err
}

// contentHash fingerprints what makes an event that event: its type,
// aggregate, version and payload. Transport details are left out on
// purpose: each publish of the same event (a relay retry after a crash)
// carries its own traceparent, and a redelivery must still be recognised as
// the same event, not quarantined as a conflicting one (task 0021, T8).
func contentHash(e *Envelope) string {
	canonical, _ := json.Marshal(struct {
		Type             string          `json:"type"`
		AggregateID      string          `json:"aggregate_id"`
		AggregateVersion uint64          `json:"aggregate_version"`
		Payload          json.RawMessage `json:"payload"`
	}{e.Type, e.AggregateID, e.AggregateVersion, e.Payload})
	return hash(canonical)
}
