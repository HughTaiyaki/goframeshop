package repository

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
)

type commentRepository struct{}

// NewComment returns the MySQL-backed comment repository.
func NewComment() service.CommentRepository {
	return &commentRepository{}
}

// Create writes only the fields the caller may set. Passing the request or
// the whole entity would let created_at, deleted_at or id be written too.
func (r *commentRepository) Create(ctx context.Context, c *entity.CommentInfo) (uint64, error) {
	id, err := dao.CommentInfo.Ctx(ctx).Data(do.CommentInfo{
		ParentId:   c.ParentId,
		UserId:     c.UserId,
		ObjectType: c.ObjectType,
		ObjectId:   c.ObjectId,
		Content:    c.Content,
	}).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// FindByID also returns soft-deleted rows, so callers can tell "deleted" from
// "never existed" when they need to. Callers decide what to expose.
func (r *commentRepository) FindByID(ctx context.Context, id uint64) (*entity.CommentInfo, error) {
	one, err := dao.CommentInfo.Ctx(ctx).Unscoped().
		Where(dao.CommentInfo.Columns().Id, id).
		One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var comment entity.CommentInfo
	if err := one.Struct(&comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

// ListByObject returns one page of live comments on an object, newest first.
func (r *commentRepository) ListByObject(ctx context.Context, objectType uint32, objectID uint64, page, size int) ([]*entity.CommentInfo, error) {
	columns := dao.CommentInfo.Columns()
	all, err := dao.CommentInfo.Ctx(ctx).
		Where(columns.ObjectType, objectType).
		Where(columns.ObjectId, objectID).
		WhereNull(columns.DeletedAt).
		OrderDesc(columns.Id).
		Page(page, size).
		All()
	if err != nil {
		return nil, err
	}
	comments := make([]*entity.CommentInfo, 0, all.Len())
	if err := all.Structs(&comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *commentRepository) CountByObject(ctx context.Context, objectType uint32, objectID uint64) (int, error) {
	columns := dao.CommentInfo.Columns()
	return dao.CommentInfo.Ctx(ctx).
		Where(columns.ObjectType, objectType).
		Where(columns.ObjectId, objectID).
		WhereNull(columns.DeletedAt).
		Count()
}

// SoftDelete hides the comment but keeps the row, so replies still have a
// parent to point at. The user_id condition scopes the write itself.
func (r *commentRepository) SoftDelete(ctx context.Context, userID, id uint64) error {
	columns := dao.CommentInfo.Columns()
	_, err := dao.CommentInfo.Ctx(ctx).
		Data(gdb.Map{columns.DeletedAt: gtime.Now()}).
		Where(columns.Id, id).
		Where(columns.UserId, userID).
		WhereNull(columns.DeletedAt).
		Update()
	return err
}
