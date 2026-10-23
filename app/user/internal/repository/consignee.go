package repository

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/user/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/service"
)

type consigneeRepository struct{}

// NewConsignee returns the MySQL-backed address repository.
func NewConsignee() service.ConsigneeRepository {
	return &consigneeRepository{}
}

func (r *consigneeRepository) Create(ctx context.Context, c *entity.ConsigneeInfo) (uint64, error) {
	id, err := dao.ConsigneeInfo.Ctx(ctx).Data(c).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *consigneeRepository) FindByID(ctx context.Context, id uint64) (*entity.ConsigneeInfo, error) {
	one, err := dao.ConsigneeInfo.Ctx(ctx).
		Where(dao.ConsigneeInfo.Columns().Id, id).
		One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, nil
	}
	var consignee entity.ConsigneeInfo
	if err := one.Struct(&consignee); err != nil {
		return nil, err
	}
	return &consignee, nil
}

// ListByUser returns one page of this user's live addresses, newest first.
// The fixed ordering is what makes paging deterministic: without it, the same
// page number could return different rows on every request.
func (r *consigneeRepository) ListByUser(ctx context.Context, userID uint64, page, size int) ([]*entity.ConsigneeInfo, error) {
	columns := dao.ConsigneeInfo.Columns()
	all, err := dao.ConsigneeInfo.Ctx(ctx).
		Where(columns.UserId, userID).
		WhereNull(columns.DeletedAt).
		OrderDesc(columns.Id).
		Page(page, size).
		All()
	if err != nil {
		return nil, err
	}
	consignees := make([]*entity.ConsigneeInfo, 0, all.Len())
	if err := all.Structs(&consignees); err != nil {
		return nil, err
	}
	return consignees, nil
}

func (r *consigneeRepository) CountByUser(ctx context.Context, userID uint64) (int, error) {
	columns := dao.ConsigneeInfo.Columns()
	return dao.ConsigneeInfo.Ctx(ctx).
		Where(columns.UserId, userID).
		WhereNull(columns.DeletedAt).
		Count()
}

func (r *consigneeRepository) Update(ctx context.Context, c *entity.ConsigneeInfo) error {
	_, err := dao.ConsigneeInfo.Ctx(ctx).
		Data(consigneeColumns(c)).
		Where(dao.ConsigneeInfo.Columns().Id, c.Id).
		WhereNull(dao.ConsigneeInfo.Columns().DeletedAt).
		Update()
	return err
}

// UpdateAsDefault clears every other default address and writes c, inside one
// transaction. Two separate writes would leave the user with either two default
// addresses (first write fails) or none (second write fails).
func (r *consigneeRepository) UpdateAsDefault(ctx context.Context, c *entity.ConsigneeInfo) error {
	columns := dao.ConsigneeInfo.Columns()
	return dao.ConsigneeInfo.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := dao.ConsigneeInfo.Ctx(ctx).TX(tx).
			Data(gdb.Map{columns.IsDefault: 0}).
			Where(columns.UserId, c.UserId).
			WhereNull(columns.DeletedAt).
			Update()
		if err != nil {
			return err
		}
		_, err = dao.ConsigneeInfo.Ctx(ctx).TX(tx).
			Data(consigneeColumns(c)).
			Where(columns.Id, c.Id).
			Update()
		return err
	})
}

// SoftDelete hides the row instead of removing it, so orders that point at this
// address keep a resolvable reference.
func (r *consigneeRepository) SoftDelete(ctx context.Context, userID, id uint64) error {
	columns := dao.ConsigneeInfo.Columns()
	_, err := dao.ConsigneeInfo.Ctx(ctx).
		Data(gdb.Map{columns.DeletedAt: gtime.Now()}).
		Where(columns.Id, id).
		Where(columns.UserId, userID).
		WhereNull(columns.DeletedAt).
		Update()
	return err
}

// consigneeColumns lists only the mutable address fields. Passing the whole
// entity to Update would rewrite created_at and deleted_at as well.
func consigneeColumns(c *entity.ConsigneeInfo) gdb.Map {
	return gdb.Map{
		dao.ConsigneeInfo.Columns().IsDefault: normalizeFlag(c.IsDefault),
		dao.ConsigneeInfo.Columns().Name:      c.Name,
		dao.ConsigneeInfo.Columns().Phone:     c.Phone,
		dao.ConsigneeInfo.Columns().Province:  c.Province,
		dao.ConsigneeInfo.Columns().City:      c.City,
		dao.ConsigneeInfo.Columns().Town:      c.Town,
		dao.ConsigneeInfo.Columns().Street:    c.Street,
		dao.ConsigneeInfo.Columns().Detail:    c.Detail,
		dao.ConsigneeInfo.Columns().UpdatedAt: gtime.Now(),
	}
}

func normalizeFlag(flag uint) uint {
	if flag == 0 {
		return 0
	}
	return 1
}
