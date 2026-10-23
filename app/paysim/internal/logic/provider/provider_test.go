package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/fake"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
)

var t0 = time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func setup() (service.IProvider, *fake.Repository, *clock) {
	repo := fake.New()
	c := &clock{now: t0}
	return New(repo, c.Now), repo, c
}

func create(t *testing.T, p service.IProvider, id string) *service.Payment {
	t.Helper()
	got, err := p.Create(context.Background(), input(id, 1990))
	if err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
	return got
}

func input(id string, amount uint64) *service.CreateInput {
	return &service.CreateInput{
		PaymentID: id, OrderNo: "O-" + id, AmountCents: amount,
		Currency: service.Currency, ExpiresAt: t0.Add(15 * time.Minute),
	}
}

func TestCreateIsIdempotent(t *testing.T) {
	p, _, _ := setup()
	ctx := context.Background()
	first := create(t, p, "P1")
	if first.State != service.StateCreated || first.AmountCents != 1990 {
		t.Fatalf("created %+v", first)
	}
	again := create(t, p, "P1")
	if again.State != service.StateCreated {
		t.Fatalf("retry: %+v", again)
	}
	if _, err := p.Create(ctx, input("P1", 2000)); !errors.Is(err, service.ErrConflict) {
		t.Fatalf("different amount: %v", err)
	}
	other := input("P1", 1990)
	other.OrderNo = "O-other"
	if _, err := p.Create(ctx, other); !errors.Is(err, service.ErrConflict) {
		t.Fatalf("different order: %v", err)
	}
	bad := input("P2", 1990)
	bad.Currency = "USD"
	if _, err := p.Create(ctx, bad); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("USD: %v", err)
	}
}

func TestCloseBeforeCreateLeavesATombstone(t *testing.T) {
	p, _, _ := setup()
	ctx := context.Background()
	closed, err := p.Close(ctx, "P1", "O-P1")
	if err != nil || closed.State != service.StateClosed || closed.AmountCents != 0 {
		t.Fatalf("close: %+v %v", closed, err)
	}
	late := create(t, p, "P1")
	if late.State != service.StateClosed {
		t.Fatalf("a late create reopened the payment: %+v", late)
	}
	if _, err := p.Pay(ctx, "P1", true); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("paid a closed payment: %v", err)
	}
}

func TestPayAndCloseExcludeEachOther(t *testing.T) {
	p, _, _ := setup()
	ctx := context.Background()

	create(t, p, "P1")
	paid, err := p.Pay(ctx, "P1", true)
	if err != nil || paid.State != service.StateSucceeded || paid.TxID == "" || paid.PaidAt == nil {
		t.Fatalf("pay: %+v %v", paid, err)
	}
	afterPay, err := p.Close(ctx, "P1", "O-P1")
	if err != nil || afterPay.State != service.StateSucceeded {
		t.Fatalf("close after pay must report SUCCEEDED, got %+v %v", afterPay, err)
	}
	again, err := p.Pay(ctx, "P1", true)
	if err != nil || again.TxID != paid.TxID {
		t.Fatalf("paying twice charged twice: %+v %v", again, err)
	}

	create(t, p, "P2")
	if closed, err := p.Close(ctx, "P2", "O-P2"); err != nil || closed.State != service.StateClosed {
		t.Fatalf("close: %+v %v", closed, err)
	}
	if _, err := p.Pay(ctx, "P2", true); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("pay after close: %v", err)
	}
}

func TestConcurrentPayAndClose(t *testing.T) {
	p, repo, _ := setup()
	ctx := context.Background()
	for i := 0; i < 200; i++ {
		id := fmt.Sprintf("P%d", i)
		create(t, p, id)
		var wg sync.WaitGroup
		var payErr error
		var closed *service.Payment
		wg.Add(2)
		go func() { defer wg.Done(); _, payErr = p.Pay(ctx, id, false) }()
		go func() { defer wg.Done(); closed, _ = p.Close(ctx, id, "O-"+id) }()
		wg.Wait()
		final, _ := repo.Find(ctx, id)
		switch final.State {
		case service.StateSucceeded:
			if payErr != nil || closed.State != service.StateSucceeded {
				t.Fatalf("%s paid, but pay=%v close saw %s", id, payErr, closed.State)
			}
		case service.StateClosed:
			if payErr == nil || closed.State != service.StateClosed {
				t.Fatalf("%s closed, but pay succeeded or close saw %s", id, closed.State)
			}
		default:
			t.Fatalf("%s ended %s", id, final.State)
		}
	}
}

func TestPayAfterExpiry(t *testing.T) {
	p, _, c := setup()
	create(t, p, "P1")
	c.Add(15 * time.Minute)
	if _, err := p.Pay(context.Background(), "P1", true); !errors.Is(err, service.ErrExpired) {
		t.Fatalf("paid after expiry: %v", err)
	}
}

func TestPayWithoutNotification(t *testing.T) {
	p, _, _ := setup()
	create(t, p, "P1")
	create(t, p, "P2")
	silent, _ := p.Pay(context.Background(), "P1", false)
	loud, _ := p.Pay(context.Background(), "P2", true)
	if silent.NextNotifyAt != nil || loud.NextNotifyAt == nil {
		t.Fatalf("notify flags: silent %v loud %v", silent.NextNotifyAt, loud.NextNotifyAt)
	}
}

func TestRefund(t *testing.T) {
	p, _, _ := setup()
	ctx := context.Background()
	create(t, p, "P1")
	if _, err := p.Refund(ctx, "R1", "P1", 1990); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("refunded an unpaid payment: %v", err)
	}
	if _, err := p.Refund(ctx, "R1", "nope", 1990); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("unknown payment: %v", err)
	}
	p.Pay(ctx, "P1", false)
	if _, err := p.Refund(ctx, "R1", "P1", 1000); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("partial refund: %v", err)
	}
	r, err := p.Refund(ctx, "R1", "P1", 1990)
	if err != nil || r.ID != "R1" {
		t.Fatalf("refund: %+v %v", r, err)
	}
	if again, err := p.Refund(ctx, "R1", "P1", 1990); err != nil || again.ID != "R1" {
		t.Fatalf("same refund again: %+v %v", again, err)
	}
	if _, err := p.Refund(ctx, "R2", "P1", 1990); !errors.Is(err, service.ErrConflict) {
		t.Fatalf("second refund id: %v", err)
	}
	q, _ := p.Query(ctx, "P1")
	if q.State != service.StateRefunded {
		t.Fatalf("state after refund: %s", q.State)
	}
	if closed, _ := p.Close(ctx, "P1", "O-P1"); closed.State != service.StateRefunded {
		t.Fatalf("close changed a refunded payment: %s", closed.State)
	}
}

func TestQueryUnknown(t *testing.T) {
	p, _, _ := setup()
	if _, err := p.Query(context.Background(), "P1"); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("query: %v", err)
	}
}
