package repository_test

// Integration tests for the simulator's row-locked decisions against a real
// MySQL: the fake's mutex cannot show that SELECT ... FOR UPDATE and the
// duplicate-insert retry really serialise Pay against Close.
//
//	make infra-up && make db-init
//	GOFRAMESHOP_IT=1 go test ./app/paysim/internal/repository/ -run Integration -v
//
// PAYSIM_IT_LINK overrides the connection. Payment ids are unique per run and
// the rows are deleted afterwards.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/dao"
	providerlogic "github.com/hughtaiyaki/goframeshop/app/paysim/internal/logic/provider"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
)

func integration(t *testing.T) (context.Context, string) {
	t.Helper()
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose MySQL")
	}
	link := os.Getenv("PAYSIM_IT_LINK")
	if link == "" {
		link = "mysql:shop:change-me@tcp(127.0.0.1:3306)/paysim?loc=Local&parseTime=true"
	}
	if err := gdb.SetConfig(gdb.Config{"default": gdb.ConfigGroup{{Link: link, MaxOpenConnCount: 64}}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := dao.Payment.DB().Exec(ctx, "SELECT 1"); err != nil {
		t.Fatalf("cannot reach MySQL: %v", err)
	}
	prefix := fmt.Sprintf("IT%d-", time.Now().UnixNano())
	t.Cleanup(func() {
		dao.Refund.Ctx(ctx).WhereLike(dao.Refund.Columns().PaymentId, prefix+"%").Delete()
		dao.Payment.Ctx(ctx).WhereLike(dao.Payment.Columns().PaymentId, prefix+"%").Delete()
	})
	return ctx, prefix
}

func TestIntegrationPayAndCloseRace(t *testing.T) {
	ctx, prefix := integration(t)
	repo := repository.New()
	p := providerlogic.New(repo, time.Now)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("%s%d", prefix, i)
		if _, err := p.Create(ctx, &service.CreateInput{
			PaymentID: id, OrderNo: "O" + id, AmountCents: 100, Currency: "CNY", ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var payErr, closeErr error
		var closed *service.Payment
		wg.Add(2)
		go func() { defer wg.Done(); _, payErr = p.Pay(ctx, id, false) }()
		go func() { defer wg.Done(); closed, closeErr = p.Close(ctx, id, "O"+id) }()
		wg.Wait()
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		final, _ := repo.Find(ctx, id)
		paid := final.State == service.StateSucceeded
		if paid != (payErr == nil) || closed.State != final.State {
			t.Fatalf("%s: final %s, pay err %v, close saw %s", id, final.State, payErr, closed.State)
		}
		if paid && final.TxID == "" {
			t.Fatalf("%s paid without a transaction id", id)
		}
	}
}

func TestIntegrationConcurrentFirstRequests(t *testing.T) {
	ctx, prefix := integration(t)
	repo := repository.New()
	p := providerlogic.New(repo, time.Now)
	id := prefix + "first"
	// Half create, half close, all at once, on a payment that does not exist:
	// exactly one insert wins and everyone agrees on the outcome.
	var wg sync.WaitGroup
	states := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var got *service.Payment
			var err error
			if i%2 == 0 {
				got, err = p.Create(ctx, &service.CreateInput{
					PaymentID: id, OrderNo: "O" + id, AmountCents: 100, Currency: "CNY", ExpiresAt: time.Now().Add(time.Hour),
				})
			} else {
				got, err = p.Close(ctx, id, "O"+id)
			}
			if err != nil {
				t.Error(err)
				return
			}
			states <- got.State
		}(i)
	}
	wg.Wait()
	close(states)
	final, _ := repo.Find(ctx, id)
	if final.State != service.StateClosed {
		t.Fatalf("closes ran, yet the payment is %s", final.State)
	}
	if _, err := p.Pay(ctx, id, false); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("paid after close: %v", err)
	}
}

func TestIntegrationRefundOnce(t *testing.T) {
	ctx, prefix := integration(t)
	p := providerlogic.New(repository.New(), time.Now)
	id := prefix + "refund"
	p.Create(ctx, &service.CreateInput{PaymentID: id, OrderNo: "O" + id, AmountCents: 100, Currency: "CNY", ExpiresAt: time.Now().Add(time.Hour)})
	if _, err := p.Pay(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.Refund(ctx, "late-"+id, id, 100); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	n, _ := dao.Refund.Ctx(ctx).Where(dao.Refund.Columns().PaymentId, id).Count()
	if n != 1 {
		t.Fatalf("%d refunds for one payment", n)
	}
	// Fully refunded: nothing more can go back (task 0018 allows parts, not
	// more than was paid).
	if _, err := p.Refund(ctx, "other-"+id, id, 100); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("second refund id: %v", err)
	}
}

// Partial refunds of one payment arriving together never add up to more
// than was paid: the payment row lock serialises them (task 0018).
func TestIntegrationPartialRefundsNeverExceedThePayment(t *testing.T) {
	ctx, prefix := integration(t)
	p := providerlogic.New(repository.New(), time.Now)
	id := prefix + "parts"
	p.Create(ctx, &service.CreateInput{PaymentID: id, OrderNo: "O" + id, AmountCents: 100, Currency: "CNY", ExpiresAt: time.Now().Add(time.Hour)})
	if _, err := p.Pay(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, tooMuch := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Refund(ctx, fmt.Sprintf("part%d-%s", i, id), id, 30)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, service.ErrInvalidArgument):
				tooMuch++
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok != 3 || tooMuch != 7 {
		t.Fatalf("%d refunds of 30 went through and %d were refused, want 3 and 7", ok, tooMuch)
	}
	if q, _ := p.Query(ctx, id); q.State != service.StateSucceeded {
		t.Fatalf("state after 90 of 100: %s", q.State)
	}
	if _, err := p.Refund(ctx, "rest-"+id, id, 10); err != nil {
		t.Fatal(err)
	}
	if q, _ := p.Query(ctx, id); q.State != service.StateRefunded {
		t.Fatalf("state after 100 of 100: %s", q.State)
	}
	n, _ := dao.Refund.Ctx(ctx).Where(dao.Refund.Columns().PaymentId, id).Count()
	if n != 4 {
		t.Fatalf("%d refund rows, want 4", n)
	}
}
