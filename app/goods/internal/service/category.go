package service

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
)

// ICategory is the category application service.
type ICategory interface {
	List(ctx context.Context, parentID uint64, page, size int) ([]*Category, int, error)
	ListAll(ctx context.Context) ([]*Category, error)
	Create(ctx context.Context, in *CategoryInput) (uint64, error)
	Update(ctx context.Context, categoryID uint64, in *CategoryInput) error
	Delete(ctx context.Context, categoryID uint64) error
}

// CategoryRepository is the persistence seam for category_info. Every read
// skips soft-deleted rows.
type CategoryRepository interface {
	Create(ctx context.Context, c *entity.CategoryInfo) (uint64, error)
	FindByID(ctx context.Context, id uint64) (*entity.CategoryInfo, error)
	Update(ctx context.Context, c *entity.CategoryInfo) error
	SoftDelete(ctx context.Context, id uint64) error
	ListByParent(ctx context.Context, parentID uint64, page, size int) ([]*entity.CategoryInfo, error)
	CountByParent(ctx context.Context, parentID uint64) (int, error)
	ListAll(ctx context.Context) ([]*entity.CategoryInfo, error)
}
