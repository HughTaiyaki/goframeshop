package cache

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

func newCache(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client, err := gredis.New(&gredis.Config{Address: server.Addr()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	return NewRedis(client, DefaultSettings), server
}

func TestGoodsRoundTrip(t *testing.T) {
	ctx := context.Background()
	c, server := newCache(t)
	created := time.Date(2026, 10, 19, 8, 30, 15, 123_000_000, time.UTC)
	want := &service.Goods{
		ID: 7, Name: "电视", PriceCents: 199900, OnShelf: true,
		Images:    []service.GoodsImage{{ID: 1, FileID: 9, URL: "https://img/a.png", Sort: 1}},
		CreatedAt: created,
	}

	if _, found, err := c.GetGoods(ctx, 7); err != nil || found {
		t.Fatalf("empty cache: found=%v err=%v", found, err)
	}
	if err := c.SetGoods(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, found, err := c.GetGoods(ctx, 7)
	if err != nil || !found || got.Name != "电视" || got.Images[0].URL != "https://img/a.png" || !got.CreatedAt.Equal(created) {
		t.Fatalf("got %+v found=%v err=%v", got, found, err)
	}

	// The TTL is one hour ±10%.
	ttl := server.TTL("goods:v2:detail:7")
	if ttl < 54*time.Minute || ttl > 66*time.Minute {
		t.Fatalf("ttl = %s, want within 1h ±10%%", ttl)
	}

	if err := c.DeleteGoods(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := c.GetGoods(ctx, 7); found {
		t.Fatal("still cached after delete")
	}
}

func TestAbsentMarker(t *testing.T) {
	ctx := context.Background()
	c, server := newCache(t)
	if err := c.SetGoodsAbsent(ctx, 404); err != nil {
		t.Fatal(err)
	}
	got, found, err := c.GetGoods(ctx, 404)
	if err != nil || !found || got != nil {
		t.Fatalf("got %+v found=%v err=%v; want nil, true, nil", got, found, err)
	}
	if ttl := server.TTL("goods:v2:detail:404"); ttl > 70*time.Second {
		t.Fatalf("absent marker lives %s; it must be short so a new product appears quickly", ttl)
	}
	server.FastForward(2 * time.Minute)
	if _, found, _ := c.GetGoods(ctx, 404); found {
		t.Fatal("absent marker did not expire")
	}
}

func TestCorruptEntryIsAMiss(t *testing.T) {
	ctx := context.Background()
	c, server := newCache(t)
	_ = server.Set("goods:v2:detail:1", "{not json")
	if _, found, err := c.GetGoods(ctx, 1); found || err == nil {
		t.Fatalf("found=%v err=%v; want a miss with an error", found, err)
	}
}

func TestCategoriesRoundTrip(t *testing.T) {
	ctx := context.Background()
	c, _ := newCache(t)
	if err := c.SetCategories(ctx, []*service.Category{{ID: 1, Name: "家电", Level: 1}}); err != nil {
		t.Fatal(err)
	}
	got, found, err := c.GetCategories(ctx)
	if err != nil || !found || len(got) != 1 || got[0].Name != "家电" {
		t.Fatalf("got %+v found=%v err=%v", got, found, err)
	}
	// An empty tree is still a cached answer, not a miss.
	_ = c.SetCategories(ctx, []*service.Category{})
	if got, found, _ := c.GetCategories(ctx); !found || len(got) != 0 {
		t.Fatalf("empty tree: found=%v len=%d", found, len(got))
	}
	_ = c.DeleteCategories(ctx)
	if _, found, _ := c.GetCategories(ctx); found {
		t.Fatal("still cached after delete")
	}
}

// When Redis is gone every call must fail quickly, so the caller can fall
// back to MySQL without spending its request deadline.
func TestDownRedisFailsFast(t *testing.T) {
	ctx := context.Background()
	c, server := newCache(t)
	server.Close()

	started := time.Now()
	if _, _, err := c.GetGoods(ctx, 1); err == nil {
		t.Fatal("expected an error")
	}
	if err := c.SetGoods(ctx, &service.Goods{ID: 1}); err == nil {
		t.Fatal("expected an error")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("two calls against a dead Redis took %s", elapsed)
	}
}

// Errors must not echo the command arguments: for SET that is the cached value.
func TestErrorsDoNotLeakCachedValues(t *testing.T) {
	ctx := context.Background()
	c, server := newCache(t)
	server.Close()
	err := c.SetGoods(ctx, &service.Goods{ID: 1, Name: "SECRET-NAME"})
	if err == nil || strings.Contains(err.Error(), "SECRET-NAME") {
		t.Fatalf("err = %v", err)
	}
}
