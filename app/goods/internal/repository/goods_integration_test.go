package repository_test

import (
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

// The hot list against real MySQL: the WHERE on sort, the two-key ORDER BY
// and paging. The goods sit in a category id no real category uses, so the
// test sees only its own rows whatever else is in the database.
func TestIntegrationHotListFiltersAndOrders(t *testing.T) {
	ctx := integration(t)
	category := uint64(900_000_000 + time.Now().UnixNano()%100_000_000)
	repo := repository.NewGoods()
	create := func(sort uint, status uint) uint64 {
		id, err := repo.Create(ctx, &entity.GoodsInfo{
			Name: "IT " + t.Name(), PriceCents: 1, Level1CategoryId: category, Status: status, Sort: sort,
		}, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			db := dao.Inventory.DB()
			_, _ = db.Exec(ctx, "DELETE FROM inventory WHERE goods_id = ?", id)
			_, _ = db.Exec(ctx, "DELETE FROM goods_info WHERE id = ?", id)
		})
		return id
	}
	create(0, service.StatusOnShelf) // not hot
	low := create(5, service.StatusOnShelf)
	tieOld := create(20, service.StatusOnShelf)
	tieNew := create(20, service.StatusOnShelf)
	high := create(90, service.StatusOnShelf)
	create(99, service.StatusOffShelf) // hot but off shelf

	filter := service.GoodsFilter{CategoryID: category, Shelf: service.ShelfOn, HotOnly: true}
	total, err := repo.Count(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("count = %d, want 4", total)
	}
	want := []uint64{high, tieNew, tieOld, low}
	var got []uint64
	for page := 1; page <= 2; page++ {
		rows, err := repo.List(ctx, filter, page, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			got = append(got, row.Id)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (sort desc, then newest first)", got, want)
		}
	}

	all, err := repo.Count(ctx, service.GoodsFilter{CategoryID: category, Shelf: service.ShelfOn})
	if err != nil || all != 5 {
		t.Fatalf("plain count = %d, err = %v, want 5", all, err)
	}
}
