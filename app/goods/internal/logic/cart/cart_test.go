package cart

import (
	"context"
	"errors"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/testfake"
)

const alice, bob uint64 = 1, 2

type fixture struct {
	svc    service.ICart
	cart   *testfake.Cart
	goods  *testfake.Goods
	images *testfake.Images
}

func newFixture() *fixture {
	f := &fixture{cart: testfake.NewCart(), goods: testfake.NewGoods(), images: testfake.NewImages()}
	f.svc = New(f.cart, f.goods, f.images, f.goods.Stocks)
	return f
}

func (f *fixture) goodsOnShelf(t *testing.T, name string, price uint64, stock uint32) uint64 {
	t.Helper()
	id, err := f.goods.Create(context.Background(), &entity.GoodsInfo{
		Name: name, PriceCents: price, Level1CategoryId: 1, Status: service.StatusOnShelf,
	}, stock)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAddAccumulatesInOneRow(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	tv := f.goodsOnShelf(t, "电视", 199900, 8)

	first, err := f.svc.Add(ctx, alice, tv, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.Add(ctx, alice, tv, 3)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Quantity != 5 || len(f.cart.Rows) != 1 {
		t.Fatalf("first %+v, second %+v, rows %d", first, second, len(f.cart.Rows))
	}
	if second.GoodsName != "电视" || second.PriceCents != 199900 || !second.Available || second.Stock != 8 {
		t.Fatalf("goods not joined: %+v", second)
	}
	// Bob's cart is his own row.
	if bobs, _ := f.svc.Add(ctx, bob, tv, 1); bobs.ID == first.ID {
		t.Fatal("two shoppers share a row")
	}
}

func TestQuantityLimitIsReportedNotTruncated(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	tv := f.goodsOnShelf(t, "电视", 1, 1000)
	if _, err := f.svc.Add(ctx, alice, tv, 60); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Add(ctx, alice, tv, 40); !errors.Is(err, service.ErrCartLimit) {
		t.Fatalf("60 + 40: err = %v, want ErrCartLimit", err)
	}
	if row, _ := f.cart.FindByGoods(ctx, alice, tv); row.Quantity != 60 {
		t.Fatalf("quantity after a refused add = %d, want 60", row.Quantity)
	}
	if _, err := f.svc.Add(ctx, alice, tv, 39); err != nil {
		t.Fatalf("60 + 39: %v", err)
	}
	for _, q := range []uint32{0, 100} {
		if _, err := f.svc.Add(ctx, alice, tv, q); !errors.Is(err, service.ErrInvalidArgument) {
			t.Fatalf("add %d: err = %v, want ErrInvalidArgument", q, err)
		}
	}
}

func TestCartHoldsAtMostMaxLines(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	var last uint64
	for i := 0; i < service.MaxCartLines; i++ {
		last = f.goodsOnShelf(t, "g", 1, 1)
		if _, err := f.svc.Add(ctx, alice, last, 1); err != nil {
			t.Fatal(err)
		}
	}
	extra := f.goodsOnShelf(t, "g", 1, 1)
	if _, err := f.svc.Add(ctx, alice, extra, 1); !errors.Is(err, service.ErrCartLimit) {
		t.Fatalf("line %d: err = %v, want ErrCartLimit", service.MaxCartLines+1, err)
	}
	// Adding to a row already in the cart needs no new line.
	if _, err := f.svc.Add(ctx, alice, last, 1); err != nil {
		t.Fatalf("add to an existing row in a full cart: %v", err)
	}
}

func TestOnlyGoodsOnShelfCanBeAdded(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	off, _ := f.goods.Create(ctx, &entity.GoodsInfo{Name: "下架", PriceCents: 1, Level1CategoryId: 1, Status: service.StatusOffShelf}, 1)
	gone := f.goodsOnShelf(t, "删除", 1, 1)
	_ = f.goods.SoftDelete(ctx, gone)
	for name, id := range map[string]uint64{"off shelf": off, "deleted": gone, "unknown": 999} {
		if _, err := f.svc.Add(ctx, alice, id, 1); !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}

func TestListShowsUnavailableGoods(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	tv := f.goodsOnShelf(t, "电视", 100, 5)
	stand := f.goodsOnShelf(t, "支架", 50, 0)
	gone := f.goodsOnShelf(t, "旧款", 10, 1)
	for _, id := range []uint64{tv, stand, gone} {
		if _, err := f.svc.Add(ctx, alice, id, 1); err != nil {
			t.Fatal(err)
		}
	}
	_ = f.goods.SetStatus(ctx, stand, service.StatusOffShelf)
	_ = f.goods.SoftDelete(ctx, gone)
	_, _ = f.svc.Add(ctx, bob, tv, 1)

	lines, total, err := f.svc.List(ctx, alice, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(lines) != 3 {
		t.Fatalf("total %d, lines %d, want 3", total, len(lines))
	}
	// Newest row first.
	byGoods := map[uint64]*service.CartLine{}
	for _, line := range lines {
		byGoods[line.GoodsID] = line
	}
	if lines[0].GoodsID != gone || lines[2].GoodsID != tv {
		t.Fatalf("order: %d, %d, %d", lines[0].GoodsID, lines[1].GoodsID, lines[2].GoodsID)
	}
	if l := byGoods[tv]; !l.Available || l.GoodsName != "电视" || l.Stock != 5 {
		t.Fatalf("on shelf: %+v", l)
	}
	if l := byGoods[stand]; l.Available || l.GoodsName != "支架" {
		t.Fatalf("off shelf: %+v", l)
	}
	if l := byGoods[gone]; l.Available || l.GoodsName != "" || l.PriceCents != 0 {
		t.Fatalf("deleted: %+v", l)
	}
	if _, _, err := f.svc.List(ctx, alice, 0, 10); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("page 0: err = %v", err)
	}
}

func TestOthersRowsAreNotFound(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	tv := f.goodsOnShelf(t, "电视", 100, 5)
	mine, _ := f.svc.Add(ctx, alice, tv, 1)

	if _, err := f.svc.Update(ctx, bob, mine.ID, 5); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("bob updates alice's row: err = %v, want ErrNotFound", err)
	}
	if n, err := f.svc.Delete(ctx, bob, []uint64{mine.ID}); err != nil || n != 0 {
		t.Fatalf("bob deletes alice's row: n = %d, err = %v", n, err)
	}
	if row, _ := f.cart.Find(ctx, alice, mine.ID); row == nil || row.Quantity != 1 {
		t.Fatalf("alice's row changed: %+v", row)
	}

	updated, err := f.svc.Update(ctx, alice, mine.ID, 7)
	if err != nil || updated.Quantity != 7 {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if _, err := f.svc.Update(ctx, alice, mine.ID, 100); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("update to 100: err = %v", err)
	}
	if n, err := f.svc.Delete(ctx, alice, []uint64{mine.ID, 999}); err != nil || n != 1 {
		t.Fatalf("delete: n = %d, err = %v", n, err)
	}
	if _, err := f.svc.Delete(ctx, alice, nil); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("delete nothing: err = %v", err)
	}
}

func TestEveryMethodNeedsACaller(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	if _, _, err := f.svc.List(ctx, 0, 1, 10); !errors.Is(err, service.ErrNoIdentity) {
		t.Fatalf("list: %v", err)
	}
	if _, err := f.svc.Add(ctx, 0, 1, 1); !errors.Is(err, service.ErrNoIdentity) {
		t.Fatalf("add: %v", err)
	}
	if _, err := f.svc.Update(ctx, 0, 1, 1); !errors.Is(err, service.ErrNoIdentity) {
		t.Fatalf("update: %v", err)
	}
	if _, err := f.svc.Delete(ctx, 0, []uint64{1}); !errors.Is(err, service.ErrNoIdentity) {
		t.Fatalf("delete: %v", err)
	}
}
