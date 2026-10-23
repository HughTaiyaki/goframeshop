// Package repository implements the persistence seam with GoFrame DAOs.
package repository

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/file/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/file/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type fileRepository struct{}

// NewFile returns the MySQL-backed file repository.
func NewFile() service.FileRepository {
	return &fileRepository{}
}

// Create writes only the listed fields, so ids and timestamps are never taken
// from the caller.
func (r *fileRepository) Create(ctx context.Context, f *entity.FileInfo) (uint64, error) {
	id, err := dao.FileInfo.Ctx(ctx).Data(do.FileInfo{
		UploaderId:  f.UploaderId,
		Name:        f.Name,
		Storage:     f.Storage,
		ObjectKey:   f.ObjectKey,
		Url:         f.Url,
		Size:        f.Size,
		ContentType: f.ContentType,
	}).InsertAndGetId()
	if utility.IsDuplicateKey(err) {
		return 0, service.ErrAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// FindByID skips soft-deleted rows: GoFrame adds "deleted_at IS NULL" itself
// because the table has a deleted_at column.
func (r *fileRepository) FindByID(ctx context.Context, id uint64) (*entity.FileInfo, error) {
	one, err := dao.FileInfo.Ctx(ctx).Where(dao.FileInfo.Columns().Id, id).One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var file entity.FileInfo
	if err := one.Struct(&file); err != nil {
		return nil, err
	}
	return &file, nil
}
