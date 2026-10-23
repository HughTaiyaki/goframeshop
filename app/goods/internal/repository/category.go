package repository

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type categoryRepository struct{}

// NewCategory returns the MySQL-backed category repository.
func NewCategory() service.CategoryRepository {
	return &categoryRepository{}
}

func (r *categoryRepository) Create(ctx context.Context, c *entity.CategoryInfo) (uint64, error) {
	id, err := dao.CategoryInfo.Ctx(ctx).Data(do.CategoryInfo{
		ParentId:   c.ParentId,
		Level:      c.Level,
		Name:       c.Name,
		IconFileId: c.IconFileId,
		IconUrl:    c.IconUrl,
		Sort:       c.Sort,
	}).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *categoryRepository) FindByID(ctx context.Context, id uint64) (*entity.CategoryInfo, error) {
	one, err := dao.CategoryInfo.Ctx(ctx).Where(dao.CategoryInfo.Columns().Id, id).One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var category entity.CategoryInfo
	if err := one.Struct(&category); err != nil {
		return nil, err
	}
	return &category, nil
}

// Update writes only the fields a category may change; parent_id and level
// are fixed at creation.
func (r *categoryRepository) Update(ctx context.Context, c *entity.CategoryInfo) error {
	_, err := dao.CategoryInfo.Ctx(ctx).
		Data(do.CategoryInfo{
			Name:       c.Name,
			IconFileId: c.IconFileId,
			IconUrl:    c.IconUrl,
			Sort:       c.Sort,
		}).
		Where(dao.CategoryInfo.Columns().Id, c.Id).
		Update()
	return err
}

func (r *categoryRepository) SoftDelete(ctx context.Context, id uint64) error {
	_, err := dao.CategoryInfo.Ctx(ctx).
		Data(gdb.Map{dao.CategoryInfo.Columns().DeletedAt: gtime.Now()}).
		Where(dao.CategoryInfo.Columns().Id, id).
		Update()
	return err
}

func (r *categoryRepository) ListByParent(ctx context.Context, parentID uint64, page, size int) ([]*entity.CategoryInfo, error) {
	columns := dao.CategoryInfo.Columns()
	all, err := dao.CategoryInfo.Ctx(ctx).
		Where(columns.ParentId, parentID).
		OrderAsc(columns.Sort).
		OrderAsc(columns.Id).
		Page(page, size).
		All()
	if err != nil {
		return nil, err
	}
	return scanCategories(all)
}

func (r *categoryRepository) CountByParent(ctx context.Context, parentID uint64) (int, error) {
	return dao.CategoryInfo.Ctx(ctx).Where(dao.CategoryInfo.Columns().ParentId, parentID).Count()
}

// ListAll returns the whole tree ordered by level, sort and id: parents always
// come before their children, which makes building the tree a single pass.
func (r *categoryRepository) ListAll(ctx context.Context) ([]*entity.CategoryInfo, error) {
	columns := dao.CategoryInfo.Columns()
	all, err := dao.CategoryInfo.Ctx(ctx).
		OrderAsc(columns.Level).
		OrderAsc(columns.Sort).
		OrderAsc(columns.Id).
		All()
	if err != nil {
		return nil, err
	}
	return scanCategories(all)
}

func scanCategories(all gdb.Result) ([]*entity.CategoryInfo, error) {
	categories := make([]*entity.CategoryInfo, 0, all.Len())
	if err := all.Structs(&categories); err != nil {
		return nil, err
	}
	return categories, nil
}
