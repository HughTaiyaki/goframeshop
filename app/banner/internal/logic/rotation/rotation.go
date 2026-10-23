// Package rotation implements the home-page carousel.
package rotation

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/logic/check"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
)

type sRotation struct {
	repo  service.RotationRepository
	files service.Files
}

func New(repo service.RotationRepository, files service.Files) service.IRotation {
	return &sRotation{repo: repo, files: files}
}

func (s *sRotation) List(ctx context.Context, page, size int) ([]*service.Rotation, int, error) {
	if err := check.Page(page, size); err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.repo.List(ctx, page, size)
	return items, total, err
}

func (s *sRotation) Create(ctx context.Context, in *service.RotationInput) (uint64, error) {
	r, err := s.build(ctx, in)
	if err != nil {
		return 0, err
	}
	return s.repo.Create(ctx, r)
}

func (s *sRotation) Update(ctx context.Context, id uint64, in *service.RotationInput) error {
	if err := s.exists(ctx, id); err != nil {
		return err
	}
	r, err := s.build(ctx, in)
	if err != nil {
		return err
	}
	r.ID = id
	return s.repo.Update(ctx, r)
}

func (s *sRotation) Delete(ctx context.Context, id uint64) error {
	if err := s.exists(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}

func (s *sRotation) exists(ctx context.Context, id uint64) error {
	ok, err := s.repo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return service.ErrNotFound
	}
	return nil
}

func (s *sRotation) build(ctx context.Context, in *service.RotationInput) (*service.Rotation, error) {
	link, err := check.Link(in.Link)
	if err != nil {
		return nil, err
	}
	picURL, err := check.PicURL(ctx, s.files, in.FileID)
	if err != nil {
		return nil, err
	}
	return &service.Rotation{FileID: in.FileID, PicURL: picURL, Link: link, Sort: in.Sort}, nil
}
