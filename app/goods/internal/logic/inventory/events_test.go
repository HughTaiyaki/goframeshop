package inventory

import (
	"context"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry/logcapture"
)

// Every Reserve and Release is logged with its operation and order ids and
// outcome; a repeated operation id says it was replayed (design §9.1).
func TestLedgerOperationsAreLogged(t *testing.T) {
	logs := logcapture.Start(t)
	ctx := context.Background()
	svc, _ := setup()

	svc.Reserve(ctx, "reserve-O1", "O1", items(1, 2, 2, 1))
	svc.Reserve(ctx, "reserve-O1", "O1", items(1, 2, 2, 1)) // a retry
	svc.Reserve(ctx, "reserve-O2", "O2", items(2, 99))      // not enough
	svc.Release(ctx, "release-O1", "O1", items(1, 2, 2, 1))
	svc.Reserve(ctx, "", "O3", items(1, 1)) // invalid: refused before any work

	reserved := logs.Events(telemetry.EventInventoryReserved)
	if len(reserved) != 3 {
		t.Fatalf("%d inventory.reserved events, want 3: %v", len(reserved), reserved)
	}
	first, retry, short := reserved[0], reserved[1], reserved[2]
	if first["operation_id"] != "reserve-O1" || first["order_id"] != "O1" ||
		first.Attr("outcome") != service.OutcomeSucceeded || first.Attr("replayed") != false || first.Attr("lines") != float64(2) {
		t.Fatalf("first reserve: %v", first)
	}
	if retry.Attr("replayed") != true || retry.Attr("outcome") != service.OutcomeSucceeded {
		t.Fatalf("retried reserve: %v", retry)
	}
	if short["order_id"] != "O2" || short.Attr("outcome") != service.OutcomeInsufficientStock {
		t.Fatalf("short reserve: %v", short)
	}
	released := logs.Events(telemetry.EventInventoryReleased)
	if len(released) != 1 || released[0]["operation_id"] != "release-O1" || released[0].Attr("outcome") != service.OutcomeSucceeded {
		t.Fatalf("inventory.released: %v", released)
	}
}

func TestIdempotencyConflictIsLoggedAsAnError(t *testing.T) {
	logs := logcapture.Start(t)
	ctx := context.Background()
	svc, _ := setup()
	svc.Reserve(ctx, "reserve-O1", "O1", items(1, 1))
	svc.Reserve(ctx, "reserve-O1", "O1", items(1, 5)) // same id, other items
	reserved := logs.Events(telemetry.EventInventoryReserved)
	if len(reserved) != 2 || reserved[1]["error.type"] != "IDEMPOTENCY_CONFLICT" || reserved[1]["severity"] != "WARN" {
		t.Fatalf("conflict: %v", reserved)
	}
}
