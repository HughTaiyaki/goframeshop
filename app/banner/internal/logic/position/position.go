// Package position implements the hand-picked goods slots.
package position

import (
	"context"
	"errors"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/logic/check"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
)

type sPosition struct {
	repo    service.PositionRepository
	files   service.Files
	catalog service.Catalog
}

func New(repo service.PositionRepository, files service.Files, catalog service.Catalog) service.IPosition {
	return &sPosition{repo: repo, files: files, catalog: catalog}
}

func (s *sPosition) List(ctx context.Context, page, size int, availableOnly bool) ([]*service.Position, int, error) {
	if err := check.Page(page, size); err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx, availableOnly)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.repo.List(ctx, page, size, availableOnly)
	return items, total, err
}

func (s *sPosition) Create(ctx context.Context, in *service.PositionInput) (uint64, error) {
	p, err := s.build(ctx, in)
	if err != nil {
		return 0, err
	}
	return s.repo.Create(ctx, p)
}

func (s *sPosition) Update(ctx context.Context, id uint64, in *service.PositionInput) error {
	ok, err := s.repo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return service.ErrNotFound
	}
	p, err := s.build(ctx, in)
	if err != nil {
		return err
	}
	p.ID = id
	return s.repo.Update(ctx, p)
}

func (s *sPosition) Delete(ctx context.Context, id uint64) error {
	ok, err := s.repo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return service.ErrNotFound
	}
	return s.repo.SoftDelete(ctx, id)
}

// build validates the input and copies the image URL and goods name in. The
// tutorial took the goods name from the form, so a slot could show any name
// next to any goods.
func (s *sPosition) build(ctx context.Context, in *service.PositionInput) (*service.Position, error) {
	link, err := check.Link(in.Link)
	if err != nil {
		return nil, err
	}
	if in.GoodsID == 0 {
		return nil, service.ErrInvalidArgument
	}
	picURL, err := check.PicURL(ctx, s.files, in.FileID)
	if err != nil {
		return nil, err
	}
	name, err := s.catalog.GoodsName(ctx, in.GoodsID)
	if errors.Is(err, service.ErrNotFound) {
		return nil, service.ErrInvalidArgument
	}
	if err != nil {
		return nil, err
	}
	return &service.Position{FileID: in.FileID, PicURL: picURL, GoodsID: in.GoodsID, GoodsName: name, Link: link, Sort: in.Sort}, nil
}
