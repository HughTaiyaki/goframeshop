package position

import (
	"context"
	"errors"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/logic/rotation"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
)

type files map[uint64]string

func (f files) URL(_ context.Context, id uint64) (string, error) {
	if u, ok := f[id]; ok {
		return u, nil
	}
	return "", service.ErrNotFound
}

type catalog map[uint64]string

func (c catalog) GoodsName(_ context.Context, id uint64) (string, error) {
	if n, ok := c[id]; ok {
		return n, nil
	}
	return "", service.ErrNotFound
}

type positions struct {
	rows   map[uint64]*service.Position
	nextID uint64
}

func (p *positions) Create(_ context.Context, x *service.Position) (uint64, error) {
	p.nextID++
	c := *x
	c.ID = p.nextID
	p.rows[c.ID] = &c
	return c.ID, nil
}
func (p *positions) Exists(_ context.Context, id uint64) (bool, error) {
	_, ok := p.rows[id]
	return ok, nil
}
func (p *positions) Update(_ context.Context, x *service.Position) error {
	c := *x
	p.rows[x.ID] = &c
	return nil
}
func (p *positions) SoftDelete(_ context.Context, id uint64) error { delete(p.rows, id); return nil }
func (p *positions) Count(context.Context, bool) (int, error)      { return len(p.rows), nil }
func (p *positions) List(context.Context, int, int, bool) ([]*service.Position, error) {
	var out []*service.Position
	for _, r := range p.rows {
		out = append(out, r)
	}
	return out, nil
}

func TestPositionCopiesImageAndGoodsName(t *testing.T) {
	ctx := context.Background()
	repo := &positions{rows: map[uint64]*service.Position{}}
	svc := New(repo, files{1: "https://img/1.png", 2: "https://img/2.png"}, catalog{9: "电视 X1"})

	id, err := svc.Create(ctx, &service.PositionInput{FileID: 1, GoodsID: 9, Sort: 3})
	if err != nil {
		t.Fatal(err)
	}
	got := repo.rows[id]
	if got.PicURL != "https://img/1.png" || got.GoodsName != "电视 X1" || got.Link != "" {
		t.Fatalf("stored %+v", got)
	}

	bad := map[string]*service.PositionInput{
		"unknown file":    {FileID: 99, GoodsID: 9},
		"unknown goods":   {FileID: 1, GoodsID: 77},
		"no goods":        {FileID: 1},
		"no file":         {GoodsID: 9},
		"javascript link": {FileID: 1, GoodsID: 9, Link: "javascript:alert(1)"},
	}
	for name, in := range bad {
		if _, err := svc.Create(ctx, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	if err := svc.Update(ctx, id, &service.PositionInput{FileID: 2, GoodsID: 9, Link: "https://shop/x"}); err != nil {
		t.Fatal(err)
	}
	if repo.rows[id].PicURL != "https://img/2.png" || repo.rows[id].Link != "https://shop/x" {
		t.Fatalf("after update %+v", repo.rows[id])
	}
	if err := svc.Update(ctx, 404, &service.PositionInput{FileID: 1, GoodsID: 9}); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}
	if err := svc.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, id); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if _, _, err := svc.List(ctx, 0, 10, false); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("page 0: %v", err)
	}
}

// The carousel shares the same rules for images and links.
func TestRotationRules(t *testing.T) {
	ctx := context.Background()
	svc := rotation.New(&rotationRepo{rows: map[uint64]*service.Rotation{}}, files{1: "https://img/1.png"})
	if _, err := svc.Create(ctx, &service.RotationInput{FileID: 1, Link: "https://shop/sale"}); err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string]*service.RotationInput{
		"unknown file": {FileID: 2},
		"data link":    {FileID: 1, Link: "data:text/html,x"},
	} {
		if _, err := svc.Create(ctx, in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := svc.Delete(ctx, 404); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}

type rotationRepo struct {
	rows   map[uint64]*service.Rotation
	nextID uint64
}

func (r *rotationRepo) Create(_ context.Context, x *service.Rotation) (uint64, error) {
	r.nextID++
	c := *x
	c.ID = r.nextID
	r.rows[c.ID] = &c
	return c.ID, nil
}
func (r *rotationRepo) Exists(_ context.Context, id uint64) (bool, error) {
	_, ok := r.rows[id]
	return ok, nil
}
func (r *rotationRepo) Update(_ context.Context, x *service.Rotation) error {
	c := *x
	r.rows[x.ID] = &c
	return nil
}
func (r *rotationRepo) SoftDelete(_ context.Context, id uint64) error { delete(r.rows, id); return nil }
func (r *rotationRepo) Count(context.Context) (int, error)            { return len(r.rows), nil }
func (r *rotationRepo) List(context.Context, int, int) ([]*service.Rotation, error) {
	return nil, nil
}
