package eventbus

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
