// Package events holds the goods service's event consumers.
package events

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/repository"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

// CartConsumer is the inbox name of this consumer.
const CartConsumer = "goods.cart-order-placed"

// Cart takes the goods of an order checked out from the cart out of the
// shopper's cart. Orders not from the cart are acknowledged and ignored.
//
// The event is a fact ("this order was placed at PlacedAt"), checked against
// the cart as it is now: rows changed after the order are left alone, and
// the inbox makes a redelivery a no-op.
type Cart struct {
	db gdb.DB
}

func NewCart(db gdb.DB) *Cart { return &Cart{db: db} }

func (x *Cart) Handle(ctx context.Context, d amqp.Delivery) error {
	_, err := eventbus.Handle(ctx, x.db, CartConsumer, d.Body, func(ctx context.Context, tx gdb.TX, e *eventbus.Envelope) error {
		var placed eventbus.OrderPlaced
		if err := json.Unmarshal(e.Payload, &placed); err != nil || placed.UserID == 0 || placed.PlacedAt.IsZero() {
			g.Log().Errorf(ctx, "order.placed %s has no user or order time; ignored", e.EventID)
			return nil
		}
		if !placed.FromCart {
			return nil
		}
		n, err := repository.RemoveOrderedTx(ctx, tx, placed)
		if err == nil {
			g.Log().Infof(ctx, "order %s placed from the cart: %d cart row(s) reduced or removed", placed.OrderNo, n)
		}
		return err
	})
	return err
}
