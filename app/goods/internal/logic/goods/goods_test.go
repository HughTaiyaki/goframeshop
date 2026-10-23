package goods

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/testfake"
)

type fixture struct {
	svc        service.IGoods
	goods      *testfake.Goods
	images     *testfake.Images
	categories *testfake.Categories
	cache      *testfake.Cache
	// a valid level1 → level2 → level3 chain, plus an unrelated level2
	l1, l2, l3, otherL2 uint64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		goods:      testfake.NewGoods(),
		images:     testfake.NewImages(),
		categories: testfake.NewCategories(),
		cache:      testfake.NewCache(),
	}
	ctx := context.Background()
	f.l1, _ = f.categories.Create(ctx, &entity.CategoryInfo{Level: 1, Name: "家电"})
	f.l2, _ = f.categories.Create(ctx, &entity.CategoryInfo{Level: 2, ParentId: f.l1, Name: "电视"})
	f.l3, _ = f.categories.Create(ctx, &entity.CategoryInfo{Level: 3, ParentId: f.l2, Name: "全面屏电视"})
	otherL1, _ := f.categories.Create(ctx, &entity.CategoryInfo{Level: 1, Name: "数码"})
	f.otherL2, _ = f.categories.Create(ctx, &entity.CategoryInfo{Level: 2, ParentId: otherL1, Name: "手机"})
	files := testfake.Files{1: "https://img/1.png", 2: "https://img/2.png"}
	f.svc = New(f.goods, f.goods.Stocks, f.images, f.categories, files, f.cache)
	return f
}

func (f *fixture) input() *service.GoodsInput {
	return &service.GoodsInput{
		Name: "  电视 X1  ", PriceCents: 199900,
		Level1CategoryID: f.l1, Level2CategoryID: f.l2, Level3CategoryID: f.l3,
		Brand: "ACME", Tags: "新品",
	}
}

func TestGetFillsTheCacheThenServesFromIt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, err := f.svc.Create(ctx, f.input(), 10)
	if err != nil {
		t.Fatal(err)
	}

	first, err := f.svc.Get(ctx, id, true)
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "电视 X1" || f.cache.Goods[id] == nil {
		t.Fatalf("got %+v, cached %v", first, f.cache.Goods[id])
	}
	reads := f.goods.Finds.Load()
	if _, err := f.svc.Get(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if f.goods.Finds.Load() != reads {
		t.Fatal("a cache hit still read MySQL")
	}
}

func TestMissingGoodsAreCachedAsAbsent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	for i := 0; i < 3; i++ {
		if _, err := f.svc.Get(ctx, 404, true); !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	}
	if got := f.goods.Finds.Load(); got != 1 {
		t.Fatalf("MySQL read %d times for a missing id, want 1", got)
	}
	if cached, ok := f.cache.Goods[404]; !ok || cached != nil {
		t.Fatal("absence was not cached")
	}
}

// The tutorial misses this: an id looked up before it existed must not stay
// "absent" once the product is created.
func TestCreateClearsAnAbsentMarker(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if _, err := f.svc.Get(ctx, 1, true); !errors.Is(err, service.ErrNotFound) {
		t.Fatal(err)
	}
	id, err := f.svc.Create(ctx, f.input(), 10)
	if err != nil || id != 1 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if _, err := f.svc.Get(ctx, 1, true); err != nil {
		t.Fatalf("new product hidden by a stale absent marker: %v", err)
	}
}

func TestEveryWriteInvalidatesTheCachedProduct(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, _ := f.svc.Create(ctx, f.input(), 10)

	writes := map[string]func() error{
		"update": func() error {
			in := f.input()
			in.PriceCents = 99
			return f.svc.Update(ctx, id, in)
		},
		"add image": func() error { _, err := f.svc.AddImage(ctx, id, 1, 0); return err },
		"remove image": func() error {
			images, _ := f.images.ListByGoods(ctx, id)
			return f.svc.RemoveImage(ctx, images[0].Id)
		},
		"delete": func() error { return f.svc.Delete(ctx, id) },
	}
	for _, name := range []string{"update", "add image", "remove image", "delete"} {
		if _, err := f.svc.Get(ctx, id, true); err != nil {
			t.Fatalf("before %s: %v", name, err)
		}
		if f.cache.Goods[id] == nil {
			t.Fatalf("before %s: not cached", name)
		}
		if err := writes[name](); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, ok := f.cache.Goods[id]; ok {
			t.Fatalf("%s left a stale cached product", name)
		}
	}
	if _, err := f.svc.Get(ctx, id, true); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("deleted product still served: %v", err)
	}
}

func TestReadsSurviveABrokenCache(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, _ := f.svc.Create(ctx, f.input(), 10)
	f.cache.Down = true

	if got, err := f.svc.Get(ctx, id, true); err != nil || got.ID != id {
		t.Fatalf("Get with cache down = %+v, %v", got, err)
	}
	// Writes still succeed: MySQL is the source of truth.
	if err := f.svc.Update(ctx, id, f.input()); err != nil {
		t.Fatalf("Update with cache down: %v", err)
	}
}

func TestConcurrentMissesReadMySQLOnce(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, _ := f.svc.Create(ctx, f.input(), 10)
	f.goods.Finds.Store(0)
	f.goods.FindGate = make(chan struct{})

	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.Get(ctx, id, true)
			errs <- err
		}()
	}
	// Let the requests pile up behind the first load, then release it.
	time.Sleep(50 * time.Millisecond)
	close(f.goods.FindGate)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := f.goods.Finds.Load(); got != 1 {
		t.Fatalf("50 concurrent misses read MySQL %d times, want 1", got)
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	edits := map[string]func(*service.GoodsInput){
		"blank name":              func(in *service.GoodsInput) { in.Name = "  " },
		"long name":               func(in *service.GoodsInput) { in.Name = strings.Repeat("名", 201) },
		"zero price":              func(in *service.GoodsInput) { in.PriceCents = 0 },
		"long brand":              func(in *service.GoodsInput) { in.Brand = strings.Repeat("b", 31) },
		"long detail":             func(in *service.GoodsInput) { in.DetailInfo = strings.Repeat("详", 16001) },
		"no level1":               func(in *service.GoodsInput) { in.Level1CategoryID = 0 },
		"unknown level1":          func(in *service.GoodsInput) { in.Level1CategoryID = 999 },
		"level2 used as level1":   func(in *service.GoodsInput) { in.Level1CategoryID = f.l2 },
		"level2 of another tree":  func(in *service.GoodsInput) { in.Level2CategoryID = f.otherL2; in.Level3CategoryID = 0 },
		"level3 without level2":   func(in *service.GoodsInput) { in.Level2CategoryID = 0 },
		"level3 not under level2": func(in *service.GoodsInput) { in.Level3CategoryID = f.l2 },
	}
	for name, edit := range edits {
		in := f.input()
		edit(in)
		if _, err := f.svc.Create(ctx, in, 10); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
	// level2 and level3 are optional.
	in := f.input()
	in.Level2CategoryID, in.Level3CategoryID = 0, 0
	if _, err := f.svc.Create(ctx, in, 10); err != nil {
		t.Fatalf("level1 only: %v", err)
	}
}

func TestUpdateNeverTouchesStock(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, _ := f.svc.Create(ctx, f.input(), 10)
	f.goods.Stocks.Set(id, service.Stock{Available: 7, Reserved: 2, Sold: 42}) // as if orders had run

	if err := f.svc.Update(ctx, id, f.input()); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.svc.Get(ctx, id, true); got.Stock != (service.Stock{Available: 7, Reserved: 2, Sold: 42}) {
		t.Fatalf("stock = %+v after an admin edit", got.Stock)
	}
	if err := f.svc.Update(ctx, 999, f.input()); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("update missing: err = %v", err)
	}
}

func TestImages(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, _ := f.svc.Create(ctx, f.input(), 10)

	if _, err := f.svc.AddImage(ctx, id, 2, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AddImage(ctx, id, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AddImage(ctx, id, 1, 9); !errors.Is(err, service.ErrAlreadyExists) {
		t.Fatalf("same file twice: err = %v", err)
	}
	if _, err := f.svc.AddImage(ctx, id, 77, 0); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("unknown file: err = %v, want ErrInvalidArgument", err)
	}
	if _, err := f.svc.AddImage(ctx, 999, 1, 0); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("unknown goods: err = %v, want ErrNotFound", err)
	}

	got, _ := f.svc.Get(ctx, id, true)
	if len(got.Images) != 2 || got.Images[0].URL != "https://img/1.png" || got.Images[1].URL != "https://img/2.png" {
		t.Fatalf("images not ordered by sort: %+v", got.Images)
	}
	items, total, _ := f.svc.List(ctx, service.GoodsFilter{}, 1, 10)
	if total != 1 || items[0].CoverURL != "https://img/1.png" {
		t.Fatalf("cover = %q", items[0].CoverURL)
	}
	if err := f.svc.RemoveImage(ctx, 999); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("remove missing: err = %v", err)
	}
}

func TestListFiltersByCategoryAtAnyLevel(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	full, _ := f.svc.Create(ctx, f.input(), 10)
	in := f.input()
	in.Level2CategoryID, in.Level3CategoryID = 0, 0
	topOnly, _ := f.svc.Create(ctx, in, 10)

	for categoryID, want := range map[uint64][]uint64{
		0:    {topOnly, full},
		f.l1: {topOnly, full},
		f.l2: {full},
		f.l3: {full},
	} {
		items, total, err := f.svc.List(ctx, service.GoodsFilter{CategoryID: categoryID}, 1, 10)
		if err != nil || total != len(want) || len(items) != len(want) {
			t.Fatalf("category %d: total=%d items=%d err=%v", categoryID, total, len(items), err)
		}
		for i := range want {
			if items[i].ID != want[i] {
				t.Fatalf("category %d: order %v", categoryID, items)
			}
		}
	}
}

func TestNewGoodsStartOffShelfAndArePublishedExplicitly(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, err := f.svc.Create(ctx, f.input(), 5)
	if err != nil {
		t.Fatal(err)
	}

	// Public callers cannot see it yet, administrators can.
	if _, err := f.svc.Get(ctx, id, false); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("public get of a new product: err = %v", err)
	}
	got, err := f.svc.Get(ctx, id, true)
	if err != nil || got.OnShelf || got.Stock.Available != 5 {
		t.Fatalf("admin get: %+v %v", got, err)
	}
	if _, total, _ := f.svc.List(ctx, service.GoodsFilter{Shelf: service.ShelfOn}, 1, 10); total != 0 {
		t.Fatalf("public list shows %d off-shelf goods", total)
	}

	if err := f.svc.SetShelf(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.cache.Goods[id]; ok {
		t.Fatal("shelf change left the cached product in place")
	}
	if got, err := f.svc.Get(ctx, id, false); err != nil || !got.OnShelf {
		t.Fatalf("public get after publishing: %+v %v", got, err)
	}

	// Taking it off shelf hides it again, even though it is cached.
	_, _ = f.svc.Get(ctx, id, false)
	if err := f.svc.SetShelf(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Get(ctx, id, false); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("public get after unpublishing: err = %v", err)
	}
	if err := f.svc.SetShelf(ctx, 999, true); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("missing goods: err = %v", err)
	}
}

// Stock is read live and never cached: a reservation must show up at once
// without invalidating the cached product.
func TestStockIsLiveNotCached(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, _ := f.svc.Create(ctx, f.input(), 5)
	if _, err := f.svc.Get(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if f.cache.Goods[id].Stock != (service.Stock{}) {
		t.Fatal("stock numbers were stored in the cache")
	}
	f.goods.Stocks.Set(id, service.Stock{Available: 1, Reserved: 4})
	got, _ := f.svc.Get(ctx, id, true)
	if got.Stock.Available != 1 || got.Stock.Reserved != 4 {
		t.Fatalf("stale stock %+v", got.Stock)
	}
	if f.cache.Goods[id].Stock != (service.Stock{}) {
		t.Fatal("a caller's live stock leaked into the shared cached value")
	}
}

func TestAdjustStock(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	id, _ := f.svc.Create(ctx, f.input(), 5)

	stock, err := f.svc.AdjustStock(ctx, id, 3)
	if err != nil || stock.Available != 8 {
		t.Fatalf("add: %+v %v", stock, err)
	}
	if _, err := f.svc.AdjustStock(ctx, id, -9); !errors.Is(err, service.ErrInsufficientStock) {
		t.Fatalf("below zero: err = %v", err)
	}
	if stock, err := f.svc.AdjustStock(ctx, id, -8); err != nil || stock.Available != 0 {
		t.Fatalf("to zero: %+v %v", stock, err)
	}
	if _, err := f.svc.AdjustStock(ctx, id, 0); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("zero delta: err = %v", err)
	}
	if _, err := f.svc.AdjustStock(ctx, 999, 1); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("missing goods: err = %v", err)
	}
}

func TestHotListKeepsSortedGoodsOnly(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	create := func(sort uint32, onShelf bool) uint64 {
		in := f.input()
		in.Sort = sort
		id, err := f.svc.Create(ctx, in, 10)
		if err != nil {
			t.Fatal(err)
		}
		if onShelf {
			if err := f.svc.SetShelf(ctx, id, true); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	cold := create(0, true)
	low := create(5, true)
	high := create(90, true)
	tieOld := create(20, true)
	tieNew := create(20, true)
	create(99, false) // hot but off shelf: never in the public hot list

	items, total, err := f.svc.List(ctx, service.GoodsFilter{Shelf: service.ShelfOn, HotOnly: true}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint64{high, tieNew, tieOld, low}
	if total != len(want) || len(items) != len(want) {
		t.Fatalf("total=%d items=%d, want %d", total, len(items), len(want))
	}
	for i, id := range want {
		if items[i].ID != id {
			t.Fatalf("position %d = %d, want %d (sort desc, then newest first)", i, items[i].ID, id)
		}
	}
	if items[0].Sort != 90 {
		t.Fatalf("summary sort = %d, want 90", items[0].Sort)
	}

	// Without HotOnly the list is unchanged: everything, newest first.
	all, total, _ := f.svc.List(ctx, service.GoodsFilter{Shelf: service.ShelfOn}, 1, 10)
	if total != 5 || all[0].ID != tieNew || all[4].ID != cold {
		t.Fatalf("plain list changed: %+v", all)
	}

	// Paging stays deterministic across equal weights.
	page2, _, _ := f.svc.List(ctx, service.GoodsFilter{Shelf: service.ShelfOn, HotOnly: true}, 2, 2)
	if len(page2) != 2 || page2[0].ID != tieOld || page2[1].ID != low {
		t.Fatalf("page 2 = %+v", page2)
	}
}

func TestSortIsValidatedAndEditedLikeOtherFields(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	in := f.input()
	in.Sort = service.MaxHotSort + 1
	if _, err := f.svc.Create(ctx, in, 1); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("sort above max: err = %v, want ErrInvalidArgument", err)
	}

	in.Sort = 7
	id, err := f.svc.Create(ctx, in, 1)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := f.svc.Get(ctx, id, true)
	if got.Sort != 7 || f.cache.Goods[id] == nil {
		t.Fatalf("sort = %d, cached %v", got.Sort, f.cache.Goods[id])
	}

	// Update is a full replacement: leaving sort out (0) takes the goods off
	// the hot list, and the cached detail is dropped.
	if err := f.svc.Update(ctx, id, f.input()); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.cache.Goods[id]; ok {
		t.Fatal("update left the cached product in place")
	}
	if got, _ := f.svc.Get(ctx, id, true); got.Sort != 0 {
		t.Fatalf("sort after update = %d, want 0", got.Sort)
	}
}
