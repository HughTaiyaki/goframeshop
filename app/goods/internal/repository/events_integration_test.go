package repository_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

// Every catalogue write leaves a goods.changed row in the outbox, written in
// the same transaction, with increasing ids (the version banner relies on).
func TestIntegrationGoodsWritesRecordEvents(t *testing.T) {
	ctx := integration(t)
	repo := repository.NewGoods()
	id := newGoods(t, ctx, 1)
	aggregate := strconv.FormatUint(id, 10)
	t.Cleanup(func() {
		dao.GoodsInfo.DB().Exec(ctx, "DELETE FROM outbox_event WHERE type = ? AND aggregate_id = ?", eventbus.TypeGoodsChanged, aggregate)
	})

	if err := repo.SetStatus(ctx, id, service.StatusOffShelf); err != nil {
		t.Fatal(err)
	}
	g, _ := repo.FindByID(ctx, id)
	if err := repo.Update(ctx, &entity.GoodsInfo{Id: id, Name: "IT renamed", PriceCents: 1, Level1CategoryId: g.Level1CategoryId}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SoftDelete(ctx, id); err != nil {
		t.Fatal(err)
	}

	all, err := dao.GoodsInfo.DB().GetAll(ctx,
		"SELECT id, payload, state FROM outbox_event WHERE type = ? AND aggregate_id = ? ORDER BY id", eventbus.TypeGoodsChanged, aggregate)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("%d events for 3 writes", len(all))
	}
	want := []eventbus.GoodsChanged{
		{GoodsID: id, Name: "IT " + t.Name(), OnShelf: false},
		{GoodsID: id, Name: "IT renamed", OnShelf: false},
		{GoodsID: id, Name: "IT renamed", OnShelf: false, Deleted: true},
	}
	for i, row := range all {
		var got eventbus.GoodsChanged
		if err := json.Unmarshal(row["payload"].Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got != want[i] || row["state"].String() != "PENDING" {
			t.Errorf("event %d: %+v (%s), want %+v", i, got, row["state"], want[i])
		}
	}
}
