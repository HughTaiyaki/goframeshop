// Package events is the banner service's consumer of goods.changed.
package events

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/repository"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

// GoodsConsumer is the inbox name of this consumer.
const GoodsConsumer = "banner.goods-changed"

// Goods keeps hand-picked slots in step with their goods: a rename shows the
// new name, and a goods taken off shelf or deleted disappears from the home
// page (the back office still lists the slot, marked unavailable).
//
// Events can arrive out of order; each slot remembers the version of the
// last one applied and ignores older ones.
type Goods struct {
	db gdb.DB
}

func NewGoods(db gdb.DB) *Goods { return &Goods{db: db} }

func (x *Goods) Handle(ctx context.Context, d amqp.Delivery) error {
	_, err := eventbus.Handle(ctx, x.db, GoodsConsumer, d.Body, func(ctx context.Context, tx gdb.TX, e *eventbus.Envelope) error {
		var changed eventbus.GoodsChanged
		if err := json.Unmarshal(e.Payload, &changed); err != nil || changed.GoodsID == 0 {
			g.Log().Errorf(ctx, "goods.changed %s has no goods id; ignored", e.EventID)
			return nil
		}
		n, err := repository.ApplyGoodsChangedTx(ctx, tx, changed, e.AggregateVersion)
		if err == nil && n > 0 {
			g.Log().Infof(ctx, "goods %d changed (version %d, on shelf %v): %d slot(s) updated",
				changed.GoodsID, e.AggregateVersion, changed.OnShelf, n)
		}
		return err
	})
	return err
}
