package search

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

type fakeIndex struct {
	service.Index
	got *service.Query
}

func (f *fakeIndex) Search(_ context.Context, q *service.Query) ([]service.Hit, int, error) {
	f.got = q
	return nil, 0, nil
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	idx := &fakeIndex{}
	svc := New(idx)
	ok := func() *service.Query { return &service.Query{Keyword: " 电视 ", Page: 1, Size: 10} }

	if _, _, err := svc.Search(ctx, ok()); err != nil || idx.got.Keyword != "电视" {
		t.Fatalf("valid query: err=%v keyword=%q", err, idx.got.Keyword)
	}
	// The deepest allowed page.
	if _, _, err := svc.Search(ctx, &service.Query{Page: 100, Size: 100}); err != nil {
		t.Fatalf("page*size = 10000: %v", err)
	}
	bad := map[string]func(*service.Query){
		"long keyword":           func(q *service.Query) { q.Keyword = strings.Repeat("词", 101) },
		"long brand":             func(q *service.Query) { q.Brand = strings.Repeat("b", 31) },
		"page 0":                 func(q *service.Query) { q.Page = 0 },
		"size 0":                 func(q *service.Query) { q.Size = 0 },
		"size 101":               func(q *service.Query) { q.Size = 101 },
		"past the result window": func(q *service.Query) { q.Page, q.Size = 101, 100 },
		"min above max":          func(q *service.Query) { q.MinPriceCents, q.MaxPriceCents = 500, 100 },
		"unknown sort":           func(q *service.Query) { q.Sort = 99 },
	}
	for name, edit := range bad {
		q := ok()
		edit(q)
		if _, _, err := svc.Search(ctx, q); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
	// Only a max price is a valid range.
	if _, _, err := svc.Search(ctx, &service.Query{MaxPriceCents: 100, Page: 1, Size: 10}); err != nil {
		t.Fatalf("max only: %v", err)
	}
}
