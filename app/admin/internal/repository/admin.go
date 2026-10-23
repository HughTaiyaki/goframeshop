// Package repository implements the persistence seam with GoFrame DAOs.
package repository

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/admin/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type adminRepository struct{}

// NewAdmin returns the MySQL-backed administrator repository.
func NewAdmin() service.AdminRepository {
	return &adminRepository{}
}

func (r *adminRepository) Create(ctx context.Context, a *entity.AdminInfo) (uint64, error) {
	id, err := dao.AdminInfo.Ctx(ctx).Data(do.AdminInfo{
		Name:         a.Name,
		PasswordHash: a.PasswordHash,
		PasswordSalt: a.PasswordSalt,
		IsSuper:      a.IsSuper,
	}).InsertAndGetId()
	if utility.IsDuplicateKey(err) {
		return 0, service.ErrAdminExists
	}
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *adminRepository) FindByName(ctx context.Context, name string) (*entity.AdminInfo, error) {
	return r.findOne(ctx, dao.AdminInfo.Columns().Name, name)
}

func (r *adminRepository) FindByID(ctx context.Context, id uint64) (*entity.AdminInfo, error) {
	return r.findOne(ctx, dao.AdminInfo.Columns().Id, id)
}

// findOne skips soft-deleted rows: GoFrame adds "deleted_at IS NULL" itself
// because the table has a deleted_at column.
func (r *adminRepository) findOne(ctx context.Context, column string, value any) (*entity.AdminInfo, error) {
	one, err := dao.AdminInfo.Ctx(ctx).Where(column, value).One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var admin entity.AdminInfo
	if err := one.Struct(&admin); err != nil {
		return nil, err
	}
	return &admin, nil
}
