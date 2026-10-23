package eventbus

import "time"

// Payloads of events that cross service boundaries. Like a protobuf message,
// they are a contract: add fields freely, never change or remove one.

// TypeGoodsChanged is published by goods whenever a goods' name, shelf
// status or existence may have changed (rabbitmq.KeyGoodsChanged).
const TypeGoodsChanged = "goods.changed"

// GoodsChanged carries the goods as it is after the change. Consumers keep
// the newest one by the envelope's AggregateVersion.
type GoodsChanged struct {
	GoodsID uint64 `json:"goodsId"`
	Name    string `json:"name"`
	OnShelf bool   `json:"onShelf"`
	Deleted bool   `json:"deleted"`
}

// TypeOrderPlaced is published by order when an order's stock is reserved
// and it becomes payable (rabbitmq.KeyOrderPlaced). An order that never got
// its stock is never "placed".
const TypeOrderPlaced = "order.placed"

// OrderPlaced describes a placed order. FromCart says the shopper checked
// out from the cart, so the cart service takes the bought goods out of it.
type OrderPlaced struct {
	OrderNo string `json:"orderNo"`
	UserID  uint64 `json:"userId"`
	// PlacedAt is when the shopper submitted the order. Cart rows changed
	// after it are a new intent and are left alone.
	PlacedAt time.Time         `json:"placedAt"`
	FromCart bool              `json:"fromCart"`
	Items    []OrderPlacedItem `json:"items"`
}

// OrderPlacedItem is one goods of a placed order.
type OrderPlacedItem struct {
	GoodsID  uint64 `json:"goodsId"`
	Quantity uint32 `json:"quantity"`
}
