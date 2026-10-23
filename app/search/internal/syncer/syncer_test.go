package syncer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/go-mysql-org/go-mysql/schema"

	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

func event(table string, columns []string, rows ...[]any) *canal.RowsEvent {
	t := &schema.Table{Schema: "goods", Name: table}
	for _, c := range columns {
		t.Columns = append(t.Columns, schema.TableColumn{Name: c})
	}
	return &canal.RowsEvent{Table: t, Rows: rows}
}

func TestGoodsIDsFindTheIDColumnByName(t *testing.T) {
	// The id column is found by name, wherever it sits in the row.
	info := event("goods_info", []string{"id", "name"}, []any{int64(7), "a"}, []any{int64(7), "b"})
	if got := GoodsIDs(info); len(got) != 1 || got[0] != 7 {
		t.Fatalf("goods_info update pair: %v", got)
	}
	stock := event("inventory", []string{"goods_id", "available"}, []any{uint64(3), 1}, []any{uint64(4), 2})
	if got := GoodsIDs(stock); len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Fatalf("inventory: %v", got)
	}
	images := event("goods_images", []string{"id", "goods_id", "url"}, []any{int64(100), int64(9), "u"})
	if got := GoodsIDs(images); len(got) != 1 || got[0] != 9 {
		t.Fatalf("goods_images must use goods_id, not the image id: %v", got)
	}
	if got := GoodsIDs(event("category_info", []string{"id"}, []any{int64(1)})); got != nil {
		t.Fatalf("unwatched table: %v", got)
	}
}

type fakeSource struct{ docs map[uint64]*service.GoodsDoc }

func (f *fakeSource) Load(_ context.Context, id uint64) (*service.GoodsDoc, error) {
	return f.docs[id], nil
}
func (f *fakeSource) LoadAll(context.Context, func(*service.GoodsDoc) error) error { return nil }

type fakeIndex struct {
	service.Index
	failures atomic.Int32
	upserts  map[uint64]bool
	deletes  map[uint64]bool
}

func (f *fakeIndex) Upsert(_ context.Context, d *service.GoodsDoc) error {
	if f.failures.Add(-1) >= 0 {
		return errors.New("es down")
	}
	f.upserts[d.GoodsID] = true
	return nil
}

func (f *fakeIndex) Delete(_ context.Context, id uint64) error {
	f.deletes[id] = true
	return nil
}

func TestApplyUpsertsOrDeletesFromTheCurrentState(t *testing.T) {
	src := &fakeSource{docs: map[uint64]*service.GoodsDoc{1: {GoodsID: 1}}}
	idx := &fakeIndex{upserts: map[uint64]bool{}, deletes: map[uint64]bool{}}
	s := New(idx, src, Config{})
	s.minBackoff, s.maxBackoff = time.Millisecond, time.Millisecond

	s.Apply(context.Background(), 1) // on shelf: indexed
	s.Apply(context.Background(), 2) // off shelf, deleted or missing: removed
	if !idx.upserts[1] || !idx.deletes[2] || idx.upserts[2] {
		t.Fatalf("upserts=%v deletes=%v", idx.upserts, idx.deletes)
	}
}

func TestApplyRetriesUntilTheIndexIsBack(t *testing.T) {
	src := &fakeSource{docs: map[uint64]*service.GoodsDoc{1: {GoodsID: 1}}}
	idx := &fakeIndex{upserts: map[uint64]bool{}, deletes: map[uint64]bool{}}
	idx.failures.Store(3)
	s := New(idx, src, Config{})
	s.minBackoff, s.maxBackoff = time.Millisecond, 2*time.Millisecond

	s.Apply(context.Background(), 1)
	if !idx.upserts[1] {
		t.Fatal("the change was dropped instead of retried")
	}
}

func TestApplyStopsWhenTheServiceStops(t *testing.T) {
	src := &fakeSource{docs: map[uint64]*service.GoodsDoc{1: {GoodsID: 1}}}
	idx := &fakeIndex{upserts: map[uint64]bool{}, deletes: map[uint64]bool{}}
	idx.failures.Store(1 << 30)
	s := New(idx, src, Config{})
	s.minBackoff, s.maxBackoff = time.Millisecond, time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { s.Apply(ctx, 1); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Apply kept retrying after shutdown")
	}
}
