// Package category implements the category application service.
//
// The whole tree is small and read on almost every page, so ListAll is cached
// as one entry. Any write deletes that entry; the next read rebuilds it.
package category

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
	"golang.org/x/sync/singleflight"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type sCategory struct {
	categories service.CategoryRepository
	goods      service.GoodsRepository
	files      service.FileLookup
	cache      service.Cache
	loads      singleflight.Group
}

func New(categories service.CategoryRepository, goods service.GoodsRepository,
	files service.FileLookup, cache service.Cache) service.ICategory {
	return &sCategory{categories: categories, goods: goods, files: files, cache: cache}
}

func (s *sCategory) List(ctx context.Context, parentID uint64, page, size int) ([]*service.Category, int, error) {
	total, err := s.categories.CountByParent(ctx, parentID)
	if err != nil {
		return nil, 0, err
	}
	records, err := s.categories.ListByParent(ctx, parentID, page, size)
	if err != nil {
		return nil, 0, err
	}
	return toDomainList(records), total, nil
}

func (s *sCategory) ListAll(ctx context.Context) ([]*service.Category, error) {
	cached, found, err := s.cache.GetCategories(ctx)
	switch {
	case err != nil:
		g.Log().Warningf(ctx, "category cache read failed, falling back to MySQL: %v", err)
	case found:
		return cached, nil
	}

	loaded, err, _ := s.loads.Do("all", func() (any, error) {
		records, err := s.categories.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		return toDomainList(records), nil
	})
	if err != nil {
		return nil, err
	}
	categories := loaded.([]*service.Category)
	if err := s.cache.SetCategories(ctx, categories); err != nil {
		g.Log().Warningf(ctx, "category cache write failed: %v", err)
	}
	return categories, nil
}

func (s *sCategory) Create(ctx context.Context, in *service.CategoryInput) (uint64, error) {
	name, err := validName(in.Name)
	if err != nil {
		return 0, err
	}
	// level is derived, never supplied: a top-level category is level 1 and
	// a child is one deeper than its parent, up to three levels.
	level := uint(1)
	if in.ParentID != 0 {
		parent, err := s.categories.FindByID(ctx, in.ParentID)
		if err != nil {
			return 0, err
		}
		if parent == nil || parent.Level >= service.MaxCategoryLevel {
			return 0, service.ErrInvalidArgument
		}
		level = parent.Level + 1
	}
	iconURL, err := s.iconURL(ctx, in.IconFileID)
	if err != nil {
		return 0, err
	}
	id, err := s.categories.Create(ctx, &entity.CategoryInfo{
		ParentId:   in.ParentID,
		Level:      level,
		Name:       name,
		IconFileId: in.IconFileID,
		IconUrl:    iconURL,
		Sort:       uint(in.Sort),
	})
	if err != nil {
		return 0, err
	}
	s.invalidate(ctx)
	return id, nil
}

// Update changes the name, icon and sort order. The parent is ignored: moving
// a category would silently break the level1/level2/level3 chain that goods
// already store.
func (s *sCategory) Update(ctx context.Context, categoryID uint64, in *service.CategoryInput) error {
	record, err := s.existing(ctx, categoryID)
	if err != nil {
		return err
	}
	name, err := validName(in.Name)
	if err != nil {
		return err
	}
	iconURL, err := s.iconURL(ctx, in.IconFileID)
	if err != nil {
		return err
	}
	record.Name = name
	record.IconFileId = in.IconFileID
	record.IconUrl = iconURL
	record.Sort = uint(in.Sort)
	if err := s.categories.Update(ctx, record); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// Delete refuses while anything still points at the category. Deleting it
// anyway would leave orphaned children and goods filed under nothing.
func (s *sCategory) Delete(ctx context.Context, categoryID uint64) error {
	if _, err := s.existing(ctx, categoryID); err != nil {
		return err
	}
	children, err := s.categories.CountByParent(ctx, categoryID)
	if err != nil {
		return err
	}
	// Off-shelf goods count too: they still sit in this category.
	goods, err := s.goods.Count(ctx, service.GoodsFilter{CategoryID: categoryID})
	if err != nil {
		return err
	}
	if children > 0 || goods > 0 {
		return service.ErrFailedPrecondition
	}
	if err := s.categories.SoftDelete(ctx, categoryID); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

func (s *sCategory) existing(ctx context.Context, categoryID uint64) (*entity.CategoryInfo, error) {
	record, err := s.categories.FindByID(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, service.ErrNotFound
	}
	return record, nil
}

// iconURL resolves an icon through the file service; 0 means no icon. A file
// that does not exist is a bad argument, not a missing category.
func (s *sCategory) iconURL(ctx context.Context, fileID uint64) (string, error) {
	if fileID == 0 {
		return "", nil
	}
	url, err := s.files.URL(ctx, fileID)
	if errors.Is(err, service.ErrNotFound) {
		return "", service.ErrInvalidArgument
	}
	return url, err
}

func (s *sCategory) invalidate(ctx context.Context) {
	if err := s.cache.DeleteCategories(ctx); err != nil {
		g.Log().Warningf(ctx, "category cache delete failed, stale for up to one TTL: %v", err)
	}
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > service.MaxCategoryNameRunes {
		return "", service.ErrInvalidArgument
	}
	return name, nil
}

func toDomainList(records []*entity.CategoryInfo) []*service.Category {
	categories := make([]*service.Category, 0, len(records))
	for _, record := range records {
		category := &service.Category{
			ID:         record.Id,
			ParentID:   record.ParentId,
			Level:      uint32(record.Level),
			Name:       record.Name,
			IconFileID: record.IconFileId,
			IconURL:    record.IconUrl,
			Sort:       uint32(record.Sort),
		}
		if record.CreatedAt != nil {
			category.CreatedAt = record.CreatedAt.Time
		}
		if record.UpdatedAt != nil {
			category.UpdatedAt = record.UpdatedAt.Time
		}
		categories = append(categories, category)
	}
	return categories
}
