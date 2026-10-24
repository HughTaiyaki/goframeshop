package events_test

// The goods.changed consumer against the Compose MySQL (banner database):
//
//	GOFRAMESHOP_IT=1 go test ./app/banner/internal/events/ -v

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/events"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"github.com/hughtaiyaki/goframeshop/utility/ittest"
)

func changed(t *testing.T, version uint64, c eventbus.GoodsChanged) amqp.Delivery {
	t.Helper()
	payload, _ := json.Marshal(c)
	body, _ := json.Marshal(eventbus.Envelope{SchemaVersion: 1, EventID: "it-" + uuid.NewString()[:30], Type: eventbus.TypeGoodsChanged,
		AggregateVersion: version, Payload: payload})
	return amqp.Delivery{Body: body}
}

func TestIntegrationGoodsChanged(t *testing.T) {
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose MySQL")
	}
	link := ittest.MySQL(t, "banner")
	if err := gdb.SetConfig(gdb.Config{"default": gdb.ConfigGroup{{Link: link}}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	goodsID := uint64(time.Now().UnixNano() % 1e12)
	repo := repository.NewPosition()
	id, err := repo.Create(ctx, &service.Position{FileID: 1, PicURL: "https://img/x.png", GoodsID: goodsID, GoodsName: "旧名字"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		dao.PositionInfo.Ctx(ctx).Unscoped().Where("id", id).Delete()
		dao.PositionInfo.DB().Exec(ctx, "DELETE FROM inbox_event WHERE consumer = ? AND event_id LIKE 'it-%'", events.GoodsConsumer)
	})
	slot := func() *service.Position {
		all, _ := repo.List(ctx, 1, 50, false)
		for _, p := range all {
			if p.ID == id {
				return p
			}
		}
		t.Fatal("slot missing")
		return nil
	}
	visible := func() bool {
		all, _ := repo.List(ctx, 1, 50, true)
		for _, p := range all {
			if p.ID == id {
				return true
			}
		}
		return false
	}
	x := events.NewGoods(dao.PositionInfo.DB())

	// Version 20 arrives before version 10: 10 must not undo 20.
	if err := x.Handle(ctx, changed(t, 20, eventbus.GoodsChanged{GoodsID: goodsID, Name: "新名字", OnShelf: false})); err != nil {
		t.Fatal(err)
	}
	if err := x.Handle(ctx, changed(t, 10, eventbus.GoodsChanged{GoodsID: goodsID, Name: "更旧的名字", OnShelf: true})); err != nil {
		t.Fatal(err)
	}
	if p := slot(); p.GoodsName != "新名字" || p.GoodsOnShelf || visible() {
		t.Fatalf("after out-of-order events: %+v visible=%v", p, visible())
	}
	// Back on shelf: visible again.
	x.Handle(ctx, changed(t, 30, eventbus.GoodsChanged{GoodsID: goodsID, Name: "新名字", OnShelf: true}))
	if !slot().GoodsOnShelf || !visible() {
		t.Fatal("back on shelf, still hidden")
	}
	// Deleted: hidden, name kept.
	x.Handle(ctx, changed(t, 40, eventbus.GoodsChanged{GoodsID: goodsID, Name: "新名字", OnShelf: true, Deleted: true}))
	if p := slot(); p.GoodsOnShelf || p.GoodsName != "新名字" {
		t.Fatalf("after delete: %+v", p)
	}
	// The same delivery twice is applied once (inbox).
	d := changed(t, 50, eventbus.GoodsChanged{GoodsID: goodsID, Name: "再改名", OnShelf: true})
	x.Handle(ctx, d)
	dao.PositionInfo.Ctx(ctx).Data("goods_name", "管理员改的").Where("id", id).Update()
	x.Handle(ctx, d)
	if slot().GoodsName != "管理员改的" {
		t.Fatal("a redelivered event was applied again")
	}
}
