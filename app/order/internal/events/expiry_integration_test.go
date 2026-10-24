package events_test

// The expiry consumer against the Compose MySQL:
//
//	GOFRAMESHOP_IT=1 go test ./app/order/internal/events/ -v

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/events"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"github.com/hughtaiyaki/goframeshop/utility/ittest"
)

// settler records Settle calls; the rest of IOrder is not used.
type settler struct {
	service.IOrder
	mu    sync.Mutex
	calls []string
}

func (s *settler) Settle(_ context.Context, no string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, no)
	return nil
}

func order(t *testing.T, ctx context.Context, expires time.Time) *service.Order {
	t.Helper()
	no := fmt.Sprintf("ITX%d", time.Now().UnixNano()%1e15)
	o := &service.Order{
		No: no, UserID: 999999, RequestID: no, RequestHash: "it", Status: service.StatusPendingPayment,
		TotalCents: 100, ReserveDeadline: time.Now(), ExpiresAt: &expires,
		Items: []service.OrderItem{{GoodsID: 1, GoodsName: "it", UnitPriceCents: 100, Quantity: 1, SubtotalCents: 100}},
	}
	id, err := repository.NewOrder().Create(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	// Create does not take expires_at (it is set on the PENDING change).
	if _, err := dao.OrderInfo.Ctx(ctx).Data(dao.OrderInfo.Columns().ExpiresAt, expires).Where("id", id).Update(); err != nil {
		t.Fatal(err)
	}
	o.ID = id
	t.Cleanup(func() {
		dao.OrderGoods.Ctx(ctx).Where(dao.OrderGoods.Columns().OrderId, id).Delete()
		dao.OrderInfo.Ctx(ctx).Where(dao.OrderInfo.Columns().Id, id).Delete()
		dao.OrderInfo.DB().Exec(ctx, "DELETE FROM inbox_event WHERE consumer = ? AND event_id LIKE 'it-%'", events.ExpiryConsumer)
	})
	return o
}

func delivery(t *testing.T, no string) amqp.Delivery {
	t.Helper()
	payload, _ := json.Marshal(service.PaymentPending{OrderNo: no})
	body, _ := json.Marshal(eventbus.Envelope{SchemaVersion: 1, EventID: "it-" + uuid.NewString()[:30], Type: service.EventPaymentPending,
		AggregateID: no, AggregateVersion: 1, Payload: payload})
	return amqp.Delivery{Body: body}
}

func TestIntegrationExpiryConsumer(t *testing.T) {
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose MySQL")
	}
	link := ittest.MySQL(t, "orders")
	if err := gdb.SetConfig(gdb.Config{"default": gdb.ConfigGroup{{Link: link}}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	orders := &settler{}
	x := events.NewExpiry(dao.OrderInfo.DB(), orders, time.Now)
	status := func(no string) string {
		o, _ := repository.NewOrder().FindByNo(ctx, no)
		return o.Status
	}

	// Expired: closed in the inbox transaction, then settled; a redelivery
	// of the same message does nothing more.
	expired := order(t, ctx, time.Now().Add(-time.Second))
	d := delivery(t, expired.No)
	for i := 0; i < 3; i++ {
		if err := x.Handle(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if status(expired.No) != service.StatusClosing || len(orders.calls) != 1 {
		t.Fatalf("expired order: %s, settled %d times", status(expired.No), len(orders.calls))
	}

	// Early (clock skew, a stray message): acknowledged, order untouched;
	// the recovery scan closes it on time.
	early := order(t, ctx, time.Now().Add(time.Hour))
	if err := x.Handle(ctx, delivery(t, early.No)); err != nil {
		t.Fatal(err)
	}
	if status(early.No) != service.StatusPendingPayment || len(orders.calls) != 1 {
		t.Fatalf("early message closed the order: %s", status(early.No))
	}
}
