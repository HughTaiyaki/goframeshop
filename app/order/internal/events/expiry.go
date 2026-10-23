// Package events connects the order service to RabbitMQ: the outbox relay
// that publishes its events, and the consumer of order.expiry.due.
package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

// ExpiryConsumer is the inbox name of the expiry consumer.
const ExpiryConsumer = "order.expiry"

// Expiry closes orders when their delayed payment_pending message arrives:
//
//	inbox transaction: record the event + PENDING_PAYMENT → CLOSING (only if
//	                   really expired, by the stored expires_at)
//	commit, then Settle: close the payment, release the stock
//	acknowledge
//
// The CLOSING state is the durable record of the remaining work: if Settle
// fails or the process dies after the commit, the recovery scan finishes it.
// That is why the remote calls can happen after the commit.
type Expiry struct {
	db     gdb.DB
	orders service.IOrder
	now    func() time.Time
}

func NewExpiry(db gdb.DB, orders service.IOrder, now func() time.Time) *Expiry {
	return &Expiry{db: db, orders: orders, now: now}
}

func (x *Expiry) Handle(ctx context.Context, d amqp.Delivery) error {
	var payload service.PaymentPending
	closed := false
	_, err := eventbus.Handle(ctx, x.db, ExpiryConsumer, d.Body, func(ctx context.Context, tx gdb.TX, e *eventbus.Envelope) error {
		if err := json.Unmarshal(e.Payload, &payload); err != nil || payload.OrderNo == "" {
			g.Log().Errorf(ctx, "expiry event %s has no order number; ignored", e.EventID)
			return nil
		}
		var err error
		closed, err = repository.ExpireTx(ctx, tx, payload.OrderNo, x.now())
		return err
	})
	if err != nil {
		return err
	}
	if closed {
		g.Log().Infof(ctx, "order %s: payment time is up; closing (expiry message)", payload.OrderNo)
		if err := x.orders.Settle(ctx, payload.OrderNo); err != nil {
			g.Log().Warningf(ctx, "order %s: settle after expiry failed, the recovery scan will finish it: %v", payload.OrderNo, err)
		}
	}
	return nil
}
