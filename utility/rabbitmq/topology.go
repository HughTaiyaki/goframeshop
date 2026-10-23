// Package rabbitmq is the shop's message transport: one topic exchange, the
// queues every flow needs, a publisher that only reports success once the
// broker has confirmed the message, and a consumer loop with manual acks.
//
// It moves bytes and reports what the broker said. It does not decide whether
// a business step happened: that is what the outbox and inbox tables are for
// (utility/eventbus, design §8).
package rabbitmq

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// Exchange carries every domain event, routed by event type.
const Exchange = "shop.events"

// Routing keys (event types) and queues. Every routing key that is published
// must be bound to at least one queue: publishing is mandatory, and an
// unroutable event is treated as a failure and retried, never silently lost.
const (
	// order.payment_pending: an order became payable. Published with a
	// per-message TTL that ends at the order's expiresAt.
	KeyOrderPaymentPending = "order.payment_pending"
	// order.expiry.due: the same message after its TTL ran out.
	KeyOrderExpiryDue = "order.expiry.due"
	// goods.changed: a goods' name, shelf status or existence changed.
	KeyGoodsChanged = "goods.changed"

	// QueueOrderExpiryDelay holds payment_pending messages until they
	// expire. Nobody consumes it; RabbitMQ dead-letters each message to
	// order.expiry.due when its TTL runs out. This is the delayed message,
	// built from plain durable queues instead of the single-node delayed
	// exchange plugin.
	QueueOrderExpiryDelay = "order.expiry.delay"
	// QueueOrderExpiry is consumed by the order service: close the order if
	// it is still unpaid.
	QueueOrderExpiry = "order.expiry"
	// QueueBannerGoods is consumed by the banner service: keep the goods
	// copied into hand-picked slots up to date.
	QueueBannerGoods = "banner.goods-changed"
)

// Queue is one durable queue and the routing keys bound to it.
type Queue struct {
	Name     string
	Bindings []string
	Args     amqp.Table
}

// Topology is every queue in the shop. Producers and consumers both declare
// all of it at startup (declaring is idempotent), so whichever starts first,
// a published event always has somewhere to go.
var Topology = []Queue{
	{
		Name:     QueueOrderExpiryDelay,
		Bindings: []string{KeyOrderPaymentPending},
		Args: amqp.Table{
			"x-dead-letter-exchange":    Exchange,
			"x-dead-letter-routing-key": KeyOrderExpiryDue,
		},
	},
	{Name: QueueOrderExpiry, Bindings: []string{KeyOrderExpiryDue}},
	{Name: QueueBannerGoods, Bindings: []string{KeyGoodsChanged}},
}

// Declare creates the exchange and every queue and binding in Topology.
func Declare(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return err
	}
	for _, q := range Topology {
		if _, err := ch.QueueDeclare(q.Name, true, false, false, false, q.Args); err != nil {
			return err
		}
		for _, key := range q.Bindings {
			if err := ch.QueueBind(q.Name, key, Exchange, false, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
