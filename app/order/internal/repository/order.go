// Package repository implements the persistence seams with GoFrame DAOs.
package repository

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

type orderRepository struct{}

// NewOrder returns the MySQL-backed order repository.
func NewOrder() service.OrderRepository {
	return &orderRepository{}
}

// Create writes the order and all its lines in one transaction: an order is
// never visible without its goods.
func (r *orderRepository) Create(ctx context.Context, o *service.Order) (uint64, error) {
	var id int64
	err := dao.OrderInfo.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var err error
		id, err = dao.OrderInfo.Ctx(ctx).TX(tx).Data(do.OrderInfo{
			OrderNo:          o.No,
			UserId:           o.UserID,
			RequestId:        o.RequestID,
			RequestHash:      o.RequestHash,
			Status:           o.Status,
			CloseReason:      o.CloseReason,
			TotalCents:       o.TotalCents,
			ConsigneeName:    o.ConsigneeName,
			ConsigneePhone:   o.ConsigneePhone,
			ConsigneeAddress: o.ConsigneeAddress,
			Remark:           o.Remark,
			FromCart:         o.FromCart,
			ReserveDeadline:  gtime.New(o.ReserveDeadline),
			CreatedAt:        createdAt(o.CreatedAt),
		}).InsertAndGetId()
		if err != nil {
			return err
		}
		lines := make([]do.OrderGoods, 0, len(o.Items))
		for _, item := range o.Items {
			lines = append(lines, do.OrderGoods{
				OrderId:        id,
				GoodsId:        item.GoodsID,
				GoodsName:      item.GoodsName,
				CoverUrl:       item.CoverURL,
				UnitPriceCents: item.UnitPriceCents,
				Quantity:       item.Quantity,
				SubtotalCents:  item.SubtotalCents,
			})
		}
		_, err = dao.OrderGoods.Ctx(ctx).TX(tx).Data(lines).Insert()
		return err
	})
	if utility.IsDuplicateKey(err) {
		return 0, service.ErrAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *orderRepository) FindByNo(ctx context.Context, orderNo string) (*service.Order, error) {
	return r.findOne(ctx, dao.OrderInfo.Ctx(ctx).Where(dao.OrderInfo.Columns().OrderNo, orderNo))
}

func (r *orderRepository) FindByRequest(ctx context.Context, userID uint64, requestID string) (*service.Order, error) {
	columns := dao.OrderInfo.Columns()
	return r.findOne(ctx, dao.OrderInfo.Ctx(ctx).Where(columns.UserId, userID).Where(columns.RequestId, requestID))
}

func (r *orderRepository) findOne(ctx context.Context, model *gdb.Model) (*service.Order, error) {
	orders, err := r.load(ctx, model.Limit(1))
	if err != nil || len(orders) == 0 {
		return nil, err
	}
	return orders[0], nil
}

func (r *orderRepository) NosByID(ctx context.Context, ids []uint64) (map[uint64]string, error) {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	columns := dao.OrderInfo.Columns()
	all, err := dao.OrderInfo.Ctx(ctx).Fields(columns.Id, columns.OrderNo).WhereIn(columns.Id, ids).All()
	if err != nil {
		return nil, err
	}
	for _, record := range all {
		out[record[columns.Id].Uint64()] = record[columns.OrderNo].String()
	}
	return out, nil
}

func (r *orderRepository) List(ctx context.Context, f *service.ListFilter) ([]*service.Order, error) {
	return r.load(ctx, r.filtered(ctx, f).OrderDesc(dao.OrderInfo.Columns().Id).Page(f.Page, f.Size))
}

func (r *orderRepository) Count(ctx context.Context, f *service.ListFilter) (int, error) {
	return r.filtered(ctx, f).Count()
}

func (r *orderRepository) filtered(ctx context.Context, f *service.ListFilter) *gdb.Model {
	columns := dao.OrderInfo.Columns()
	model := dao.OrderInfo.Ctx(ctx)
	if f.UserID != 0 {
		model = model.Where(columns.UserId, f.UserID)
	}
	if f.OrderNo != "" {
		model = model.Where(columns.OrderNo, f.OrderNo)
	}
	if f.Status != "" {
		model = model.Where(columns.Status, f.Status)
	}
	if f.CreatedFrom != nil {
		model = model.WhereGTE(columns.CreatedAt, *f.CreatedFrom)
	}
	if f.CreatedTo != nil {
		model = model.WhereLTE(columns.CreatedAt, *f.CreatedTo)
	}
	return model
}

// Transition is one conditional UPDATE: it changes the row only while its
// status is still one of c.From. The affected-row count tells the caller
// whether it won, which is what makes concurrent cancel/expire safe.
func (r *orderRepository) Transition(ctx context.Context, orderID uint64, c service.Change) (bool, error) {
	columns := dao.OrderInfo.Columns()
	data := gdb.Map{columns.Status: c.To}
	if c.CloseReason != "" {
		data[columns.CloseReason] = c.CloseReason
	}
	if c.ExpiresAt != nil {
		data[columns.ExpiresAt] = gtime.New(*c.ExpiresAt)
	}
	if c.PaidAt != nil {
		data[columns.PaidAt] = gtime.New(*c.PaidAt)
	}
	if c.ClosedAt != nil {
		data[columns.ClosedAt] = gtime.New(*c.ClosedAt)
	}
	if len(c.Events) == 0 {
		return transition(ctx, dao.OrderInfo.Ctx(ctx), orderID, c.From, data)
	}
	// With events: the change and its outbox rows commit together.
	var changed bool
	err := dao.OrderInfo.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var err error
		changed, err = transition(ctx, dao.OrderInfo.Ctx(ctx).TX(tx), orderID, c.From, data)
		if err != nil || !changed {
			return err
		}
		for _, e := range c.Events {
			if err := eventbus.Append(ctx, tx, e); err != nil {
				return err
			}
		}
		return nil
	})
	return changed, err
}

// createdAt keeps the service's own clock when it set one: order.placed
// carries the same instant, and the cart compares its rows against it.
// Unset (nil) leaves it to GoFrame, which fills in now.
func createdAt(t time.Time) *gtime.Time {
	if t.IsZero() {
		return nil
	}
	return gtime.New(t)
}

func transition(ctx context.Context, model *gdb.Model, orderID uint64, from []string, data gdb.Map) (bool, error) {
	result, err := model.Data(data).
		Where(dao.OrderInfo.Columns().Id, orderID).
		WhereIn(dao.OrderInfo.Columns().Status, from).
		Update()
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

// ExpireTx closes an unpaid order whose time is up, inside tx (the expiry
// consumer's inbox transaction). Checking expires_at here, not trusting the
// message, means an early or stray message changes nothing.
func ExpireTx(ctx context.Context, tx gdb.TX, orderNo string, now time.Time) (bool, error) {
	columns := dao.OrderInfo.Columns()
	result, err := dao.OrderInfo.Ctx(ctx).TX(tx).Data(gdb.Map{
		columns.Status:      service.StatusClosing,
		columns.CloseReason: service.ReasonPaymentTimeout,
	}).Where(columns.OrderNo, orderNo).
		Where(columns.Status, service.StatusPendingPayment).
		WhereLTE(columns.ExpiresAt, gtime.New(now)).
		Update()
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *orderRepository) Due(ctx context.Context, now time.Time, limit int) ([]*service.Order, error) {
	columns := dao.OrderInfo.Columns()
	model := dao.OrderInfo.Ctx(ctx)
	return r.load(ctx, model.
		Where(model.Builder().
			WhereIn(columns.Status, []string{service.StatusReserving, service.StatusPaymentConfirming, service.StatusClosing}).
			WhereOr(model.Builder().
				Where(columns.Status, service.StatusPendingPayment).
				WhereLTE(columns.ExpiresAt, gtime.New(now)))).
		OrderAsc(columns.Id).
		Limit(limit))
}

// load reads orders and then all their lines in one more query.
func (r *orderRepository) load(ctx context.Context, model *gdb.Model) ([]*service.Order, error) {
	all, err := model.All()
	if err != nil {
		return nil, err
	}
	var rows []*entity.OrderInfo
	if err := all.Structs(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]uint64, 0, len(rows))
	orders := make([]*service.Order, 0, len(rows))
	byID := make(map[uint64]*service.Order, len(rows))
	for _, row := range rows {
		o := toOrder(row)
		ids = append(ids, o.ID)
		orders = append(orders, o)
		byID[o.ID] = o
	}
	lineColumns := dao.OrderGoods.Columns()
	lines, err := dao.OrderGoods.Ctx(ctx).WhereIn(lineColumns.OrderId, ids).OrderAsc(lineColumns.GoodsId).All()
	if err != nil {
		return nil, err
	}
	var items []*entity.OrderGoods
	if err := lines.Structs(&items); err != nil {
		return nil, err
	}
	for _, item := range items {
		o := byID[item.OrderId]
		o.Items = append(o.Items, service.OrderItem{
			GoodsID:        item.GoodsId,
			GoodsName:      item.GoodsName,
			CoverURL:       item.CoverUrl,
			UnitPriceCents: item.UnitPriceCents,
			Quantity:       uint32(item.Quantity),
			SubtotalCents:  item.SubtotalCents,
		})
	}
	return orders, nil
}

func toOrder(row *entity.OrderInfo) *service.Order {
	o := &service.Order{
		ID:               row.Id,
		No:               row.OrderNo,
		UserID:           row.UserId,
		RequestID:        row.RequestId,
		RequestHash:      row.RequestHash,
		Status:           row.Status,
		CloseReason:      row.CloseReason,
		TotalCents:       row.TotalCents,
		ConsigneeName:    row.ConsigneeName,
		ConsigneePhone:   row.ConsigneePhone,
		ConsigneeAddress: row.ConsigneeAddress,
		Remark:           row.Remark,
		FromCart:         row.FromCart == 1,
	}
	if row.ReserveDeadline != nil {
		o.ReserveDeadline = row.ReserveDeadline.Time
	}
	if row.ExpiresAt != nil {
		t := row.ExpiresAt.Time
		o.ExpiresAt = &t
	}
	if row.PaidAt != nil {
		t := row.PaidAt.Time
		o.PaidAt = &t
	}
	if row.ClosedAt != nil {
		t := row.ClosedAt.Time
		o.ClosedAt = &t
	}
	if row.CreatedAt != nil {
		o.CreatedAt = row.CreatedAt.Time
	}
	return o
}
