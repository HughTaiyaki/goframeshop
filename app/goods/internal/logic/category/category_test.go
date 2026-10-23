package category

import (
	"context"
	"errors"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/testfake"
)

func setup() (service.ICategory, *testfake.Categories, *testfake.Goods, *testfake.Cache) {
	categories, goods, cache := testfake.NewCategories(), testfake.NewGoods(), testfake.NewCache()
	files := testfake.Files{7: "https://img/icon.png"}
	return New(categories, goods, files, cache), categories, goods, cache
}

func TestLevelIsDerivedAndCappedAtThree(t *testing.T) {
	ctx := context.Background()
	svc, rows, _, _ := setup()

	l1, err := svc.Create(ctx, &service.CategoryInput{Name: " 家电 "})
	if err != nil {
		t.Fatal(err)
	}
	l2, _ := svc.Create(ctx, &service.CategoryInput{ParentID: l1, Name: "电视"})
	l3, _ := svc.Create(ctx, &service.CategoryInput{ParentID: l2, Name: "全面屏"})
	for id, want := range map[uint64]uint{l1: 1, l2: 2, l3: 3} {
		if rows.Rows[id].Level != want {
			t.Fatalf("category %d level = %d, want %d", id, rows.Rows[id].Level, want)
		}
	}
	if rows.Rows[l1].Name != "家电" {
		t.Fatal("name not trimmed")
	}
	for name, in := range map[string]*service.CategoryInput{
		"fourth level":   {ParentID: l3, Name: "x"},
		"missing parent": {ParentID: 999, Name: "x"},
		"blank name":     {Name: "  "},
		"long name":      {Name: "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一"},
		"unknown icon":   {Name: "x", IconFileID: 404},
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
	id, err := svc.Create(ctx, &service.CategoryInput{Name: "有图标", IconFileID: 7})
	if err != nil || rows.Rows[id].IconUrl != "https://img/icon.png" {
		t.Fatalf("icon not resolved: %v %+v", err, rows.Rows[id])
	}
}

func TestListAllIsCachedAndEveryWriteInvalidatesIt(t *testing.T) {
	ctx := context.Background()
	svc, rows, _, cache := setup()
	id, _ := svc.Create(ctx, &service.CategoryInput{Name: "家电"})

	if _, err := svc.ListAll(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ListAll(ctx); err != nil {
		t.Fatal(err)
	}
	if rows.ListAlls != 1 {
		t.Fatalf("tree read from MySQL %d times, want 1", rows.ListAlls)
	}

	writes := []func() error{
		func() error { _, err := svc.Create(ctx, &service.CategoryInput{Name: "数码"}); return err },
		func() error { return svc.Update(ctx, id, &service.CategoryInput{Name: "大家电", Sort: 3}) },
		func() error { return svc.Delete(ctx, id) },
	}
	for i, write := range writes {
		if _, err := svc.ListAll(ctx); err != nil {
			t.Fatal(err)
		}
		if err := write(); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		if cache.HasCats {
			t.Fatalf("write %d left the cached tree in place", i)
		}
	}

	cache.Down = true
	if _, err := svc.ListAll(ctx); err != nil {
		t.Fatalf("ListAll with cache down: %v", err)
	}
}

func TestUpdateNeverMovesACategory(t *testing.T) {
	ctx := context.Background()
	svc, rows, _, _ := setup()
	l1, _ := svc.Create(ctx, &service.CategoryInput{Name: "家电"})
	other, _ := svc.Create(ctx, &service.CategoryInput{Name: "数码"})
	l2, _ := svc.Create(ctx, &service.CategoryInput{ParentID: l1, Name: "电视"})

	if err := svc.Update(ctx, l2, &service.CategoryInput{ParentID: other, Name: "电视机"}); err != nil {
		t.Fatal(err)
	}
	if rows.Rows[l2].ParentId != l1 || rows.Rows[l2].Name != "电视机" {
		t.Fatalf("got %+v", rows.Rows[l2])
	}
	if err := svc.Update(ctx, 999, &service.CategoryInput{Name: "x"}); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("missing: err = %v", err)
	}
}

func TestDeleteRefusesWhileInUse(t *testing.T) {
	ctx := context.Background()
	svc, _, goods, _ := setup()
	l1, _ := svc.Create(ctx, &service.CategoryInput{Name: "家电"})
	l2, _ := svc.Create(ctx, &service.CategoryInput{ParentID: l1, Name: "电视"})
	// Off shelf still counts: the goods is in the category either way.
	goodsID, _ := goods.Create(ctx, &entity.GoodsInfo{Name: "TV", Level1CategoryId: l1, Level2CategoryId: l2, Status: service.StatusOffShelf}, 0)

	if err := svc.Delete(ctx, l1); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("with a child: err = %v", err)
	}
	if err := svc.Delete(ctx, l2); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("with goods: err = %v", err)
	}
	_ = goods.SoftDelete(ctx, goodsID)
	if err := svc.Delete(ctx, l2); err != nil {
		t.Fatalf("after goods removed: %v", err)
	}
	if err := svc.Delete(ctx, l1); err != nil {
		t.Fatalf("after child removed: %v", err)
	}
	if err := svc.Delete(ctx, l1); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("twice: err = %v", err)
	}
}
