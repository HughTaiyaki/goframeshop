package repository

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

// backlogQueries are the waiting work of the recovery worker by step
// (design §9.2 trade_workflow_backlog): how many, and since when the oldest
// has waited in that state. updated_at is when an order last changed, i.e.
// when it entered the state it is still in.
var backlogQueries = map[string]struct {
	sql  string
	args []any
}{
	"reserve": {"SELECT COUNT(*) AS n, MIN(created_at) AS oldest FROM order_info WHERE status = ?", []any{service.StatusReserving}},
	"commit":  {"SELECT COUNT(*) AS n, MIN(updated_at) AS oldest FROM order_info WHERE status = ?", []any{service.StatusPaymentConfirming}},
	"close":   {"SELECT COUNT(*) AS n, MIN(updated_at) AS oldest FROM order_info WHERE status = ?", []any{service.StatusClosing}},
	"tech_refund": {"SELECT COUNT(*) AS n, MIN(created_at) AS oldest FROM tech_refund WHERE state = ?",
		[]any{service.TechRefundPending}},
	"after_sale_refund": {"SELECT COUNT(*) AS n, MIN(reviewed_at) AS oldest FROM refund_info WHERE status = ?",
		[]any{service.RefundApproved}},
}

// Backlog reads every step of the order service, including its outbox.
func Backlog(ctx context.Context) (map[string]telemetry.Backlog, error) {
	db := dao.OrderInfo.DB()
	out := make(map[string]telemetry.Backlog, len(backlogQueries)+1)
	for step, q := range backlogQueries {
		row, err := db.GetOne(ctx, q.sql, q.args...)
		if err != nil {
			return nil, err
		}
		b := telemetry.Backlog{Count: row["n"].Int64()}
		if t := row["oldest"].GTime(); t != nil {
			b.Oldest = t.Time
		}
		out[step] = b
	}
	n, oldest, err := eventbus.Backlog(ctx, db)
	if err != nil {
		return nil, err
	}
	out["outbox"] = telemetry.Backlog{Count: n, Oldest: oldest}
	return out, nil
}

// CountPendingPayment is trade_order_pending_payment.
func CountPendingPayment(ctx context.Context) (int64, error) {
	n, err := dao.OrderInfo.Ctx(ctx).Where(dao.OrderInfo.Columns().Status, service.StatusPendingPayment).Count()
	return int64(n), err
}
