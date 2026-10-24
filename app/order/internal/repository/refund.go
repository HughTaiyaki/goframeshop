package repository

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type refundRepository struct {
	orders service.OrderRepository
}

// NewRefund returns the MySQL-backed refund repository. It uses the order
// repository to turn order ids back into order numbers.
func NewRefund(orders service.OrderRepository) service.RefundRepository {
	return &refundRepository{orders: orders}
}

func (r *refundRepository) Create(ctx context.Context, refund *service.Refund) (uint64, error) {
	id, err := dao.RefundInfo.Ctx(ctx).Data(do.RefundInfo{
		RefundNo:    refund.No,
		OrderId:     refund.OrderID,
		UserId:      refund.UserID,
		GoodsId:     refund.GoodsID,
		Reason:      refund.Reason,
		AmountCents: refund.AmountCents,
		Status:      refund.Status,
	}).InsertAndGetId()
	if utility.IsDuplicateKey(err) {
		return 0, service.ErrAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *refundRepository) FindByNo(ctx context.Context, refundNo string) (*service.Refund, error) {
	refunds, err := r.load(ctx, dao.RefundInfo.Ctx(ctx).Where(dao.RefundInfo.Columns().RefundNo, refundNo).Limit(1))
	if err != nil || len(refunds) == 0 {
		return nil, err
	}
	return refunds[0], nil
}

func (r *refundRepository) List(ctx context.Context, userID uint64, status string, page, size int) ([]*service.Refund, error) {
	return r.load(ctx, r.filtered(ctx, userID, status).OrderDesc(dao.RefundInfo.Columns().Id).Page(page, size))
}

func (r *refundRepository) Count(ctx context.Context, userID uint64, status string) (int, error) {
	return r.filtered(ctx, userID, status).Count()
}

func (r *refundRepository) ByOrder(ctx context.Context, orderID uint64) ([]*service.Refund, error) {
	columns := dao.RefundInfo.Columns()
	return r.load(ctx, dao.RefundInfo.Ctx(ctx).Where(columns.OrderId, orderID).OrderAsc(columns.Id))
}

func (r *refundRepository) DueApproved(ctx context.Context, limit int) ([]*service.Refund, error) {
	columns := dao.RefundInfo.Columns()
	return r.load(ctx, dao.RefundInfo.Ctx(ctx).
		Where(columns.Status, service.RefundApproved).OrderAsc(columns.Id).Limit(limit))
}

// Transition is a conditional UPDATE: only the caller that finds the row
// still in From changes it.
func (r *refundRepository) Transition(ctx context.Context, refundID uint64, c service.RefundChange) (bool, error) {
	columns := dao.RefundInfo.Columns()
	data := gdb.Map{columns.Status: c.To}
	if c.ReviewNote != nil {
		data[columns.ReviewNote] = *c.ReviewNote
	}
	if c.ReviewedAt != nil {
		data[columns.ReviewedAt] = gtime.New(*c.ReviewedAt)
	}
	if c.RefundedAt != nil {
		data[columns.RefundedAt] = gtime.New(*c.RefundedAt)
	}
	result, err := dao.RefundInfo.Ctx(ctx).Data(data).
		Where(columns.Id, refundID).Where(columns.Status, c.From).Update()
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *refundRepository) Attempted(ctx context.Context, refundID uint64) error {
	columns := dao.RefundInfo.Columns()
	_, err := dao.RefundInfo.Ctx(ctx).Data(gdb.Map{columns.Attempts: gdb.Raw(columns.Attempts + " + 1")}).
		Where(columns.Id, refundID).Update()
	return err
}

func (r *refundRepository) filtered(ctx context.Context, userID uint64, status string) *gdb.Model {
	columns := dao.RefundInfo.Columns()
	model := dao.RefundInfo.Ctx(ctx)
	if userID != 0 {
		model = model.Where(columns.UserId, userID)
	}
	if status != "" {
		model = model.Where(columns.Status, status)
	}
	return model
}

func (r *refundRepository) load(ctx context.Context, model *gdb.Model) ([]*service.Refund, error) {
	all, err := model.All()
	if err != nil {
		return nil, err
	}
	var rows []*entity.RefundInfo
	if err := all.Structs(&rows); err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.OrderId)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	nos, err := r.orders.NosByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	names, err := goodsNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*service.Refund, 0, len(rows))
	for _, row := range rows {
		refund := &service.Refund{
			ID: row.Id, No: row.RefundNo, OrderID: row.OrderId, OrderNo: nos[row.OrderId],
			UserID: row.UserId, GoodsID: row.GoodsId, Reason: row.Reason, Status: row.Status,
			AmountCents: row.AmountCents, GoodsName: names[[2]uint64{row.OrderId, row.GoodsId}],
			ReviewNote: row.ReviewNote, ReviewedAt: timeOf(row.ReviewedAt), RefundedAt: timeOf(row.RefundedAt),
			Attempts: int(row.Attempts),
		}
		if row.CreatedAt != nil {
			refund.CreatedAt = row.CreatedAt.Time
		}
		out = append(out, refund)
	}
	return out, nil
}

// goodsNames reads the goods names copied into the orders, keyed by
// (order id, goods id), in one query.
func goodsNames(ctx context.Context, orderIDs []uint64) (map[[2]uint64]string, error) {
	columns := dao.OrderGoods.Columns()
	var lines []*entity.OrderGoods
	err := dao.OrderGoods.Ctx(ctx).Fields(columns.OrderId, columns.GoodsId, columns.GoodsName).
		WhereIn(columns.OrderId, orderIDs).Scan(&lines)
	if err != nil {
		return nil, err
	}
	out := make(map[[2]uint64]string, len(lines))
	for _, line := range lines {
		out[[2]uint64{line.OrderId, line.GoodsId}] = line.GoodsName
	}
	return out, nil
}
