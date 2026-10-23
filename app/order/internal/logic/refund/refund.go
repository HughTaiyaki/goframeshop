// Package refund records after-sale requests.
//
// The original tutorial wrote whatever the client sent. Here a request must
// name a paid order that belongs to the caller, and goods that are in it.
package refund

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

type sRefund struct {
	refunds service.RefundRepository
	orders  service.OrderRepository
	now     func() time.Time
}

func New(refunds service.RefundRepository, orders service.OrderRepository, now func() time.Time) service.IRefund {
	return &sRefund{refunds: refunds, orders: orders, now: now}
}

func (s *sRefund) Create(ctx context.Context, userID uint64, orderNo string, goodsID uint64, reason string) (*service.Refund, error) {
	reason = strings.TrimSpace(reason)
	if userID == 0 || goodsID == 0 || reason == "" || utf8.RuneCountInString(reason) > service.MaxReasonRunes {
		return nil, service.ErrInvalidArgument
	}
	order, err := s.orders.FindByNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order == nil || order.UserID != userID {
		return nil, service.ErrNotFound
	}
	// Only money that was paid can be refunded. No order can be PAID until
	// the payment simulator exists, so this refuses everything for now.
	if order.Status != service.StatusPaid {
		return nil, service.ErrFailedPrecondition
	}
	inOrder := false
	for _, item := range order.Items {
		inOrder = inOrder || item.GoodsID == goodsID
	}
	if !inOrder {
		return nil, service.ErrInvalidArgument
	}
	refund := &service.Refund{
		No:      newRefundNo(s.now()),
		OrderID: order.ID,
		OrderNo: order.No,
		UserID:  userID,
		GoodsID: goodsID,
		Reason:  reason,
		Status:  service.RefundPending,
	}
	if refund.ID, err = s.refunds.Create(ctx, refund); err != nil {
		return nil, err
	}
	return s.refunds.FindByNo(ctx, refund.No)
}

func (s *sRefund) GetMine(ctx context.Context, userID uint64, refundNo string) (*service.Refund, error) {
	refund, err := s.refunds.FindByNo(ctx, refundNo)
	if err != nil {
		return nil, err
	}
	if refund == nil || refund.UserID != userID {
		return nil, service.ErrNotFound
	}
	return refund, nil
}

func (s *sRefund) ListMine(ctx context.Context, userID uint64, page, size int) ([]*service.Refund, int, error) {
	if userID == 0 {
		return nil, 0, service.ErrInvalidArgument
	}
	return s.list(ctx, userID, "", page, size)
}

func (s *sRefund) List(ctx context.Context, status string, page, size int) ([]*service.Refund, int, error) {
	return s.list(ctx, 0, status, page, size)
}

func (s *sRefund) list(ctx context.Context, userID uint64, status string, page, size int) ([]*service.Refund, int, error) {
	if page < 1 || size < 1 || size > 50 {
		return nil, 0, service.ErrInvalidArgument
	}
	total, err := s.refunds.Count(ctx, userID, status)
	if err != nil {
		return nil, 0, err
	}
	refunds, err := s.refunds.List(ctx, userID, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	return refunds, total, nil
}

func newRefundNo(now time.Time) string {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("R%s%08d", now.UTC().Format("20060102150405"), n.Int64())
}
