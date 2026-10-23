// Package repository implements the persistence seams with GoFrame DAOs.
package repository

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// markColumns are the column names shared by praise_info and collection_info,
// copied from each table's generated Columns() so no name is typed by hand.
type markColumns struct {
	Id, UserId, ObjectType, ObjectId, CreatedAt string
}

// markRepository serves either mark table. The two tables are identical, so
// one implementation bound to a different DAO avoids two copies drifting apart.
type markRepository struct {
	model   func(ctx context.Context) *gdb.Model
	columns markColumns
}

// NewPraise returns the MySQL-backed praise repository.
func NewPraise() service.MarkRepository {
	c := dao.PraiseInfo.Columns()
	return &markRepository{
		model:   dao.PraiseInfo.Ctx,
		columns: markColumns{Id: c.Id, UserId: c.UserId, ObjectType: c.ObjectType, ObjectId: c.ObjectId, CreatedAt: c.CreatedAt},
	}
}

// NewCollection returns the MySQL-backed collection repository.
func NewCollection() service.MarkRepository {
	c := dao.CollectionInfo.Columns()
	return &markRepository{
		model:   dao.CollectionInfo.Ctx,
		columns: markColumns{Id: c.Id, UserId: c.UserId, ObjectType: c.ObjectType, ObjectId: c.ObjectId, CreatedAt: c.CreatedAt},
	}
}

func (r *markRepository) Create(ctx context.Context, m *service.Mark) (uint64, error) {
	id, err := r.model(ctx).Data(gdb.Map{
		r.columns.UserId:     m.UserID,
		r.columns.ObjectType: m.ObjectType,
		r.columns.ObjectId:   m.ObjectID,
	}).InsertAndGetId()
	if utility.IsDuplicateKey(err) {
		return 0, service.ErrAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *markRepository) FindByID(ctx context.Context, id uint64) (*service.Mark, error) {
	one, err := r.model(ctx).Where(r.columns.Id, id).One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	return r.toDomain(one), nil
}

// ListByUser returns one page of the user's marks of one object type, newest
// first. The fixed ordering is what makes paging deterministic.
func (r *markRepository) ListByUser(ctx context.Context, userID uint64, objectType uint32, page, size int) ([]*service.Mark, error) {
	all, err := r.model(ctx).
		Where(r.columns.UserId, userID).
		Where(r.columns.ObjectType, objectType).
		OrderDesc(r.columns.Id).
		Page(page, size).
		All()
	if err != nil {
		return nil, err
	}
	marks := make([]*service.Mark, 0, all.Len())
	for _, record := range all {
		marks = append(marks, r.toDomain(record))
	}
	return marks, nil
}

func (r *markRepository) CountByUser(ctx context.Context, userID uint64, objectType uint32) (int, error) {
	return r.model(ctx).
		Where(r.columns.UserId, userID).
		Where(r.columns.ObjectType, objectType).
		Count()
}

// Delete removes the row for real: a mark carries no history worth keeping,
// and a soft-deleted row would block the unique key from being reused.
// The user_id condition is repeated here so the write itself is scoped, not
// only the check before it.
func (r *markRepository) Delete(ctx context.Context, userID, id uint64) error {
	_, err := r.model(ctx).
		Where(r.columns.Id, id).
		Where(r.columns.UserId, userID).
		Delete()
	return err
}

func (r *markRepository) toDomain(record gdb.Record) *service.Mark {
	return &service.Mark{
		ID:         record[r.columns.Id].Uint64(),
		UserID:     record[r.columns.UserId].Uint64(),
		ObjectType: record[r.columns.ObjectType].Uint32(),
		ObjectID:   record[r.columns.ObjectId].Uint64(),
		CreatedAt:  record[r.columns.CreatedAt].GTime(),
	}
}
