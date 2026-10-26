package order

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

var trackingNoPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// Ship hands a PAID order to the carrier (task 0017). Repeating the same
// shipment returns the order as it is, so a double click or a retry after a
// lost response is harmless; a different shipment for an order already
// shipped is refused rather than silently replacing the waybill.
func (s *sOrder) Ship(ctx context.Context, orderNo string, in service.Shipment) (*service.Order, error) {
	in.Carrier = strings.TrimSpace(in.Carrier)
	in.TrackingNo = strings.TrimSpace(in.TrackingNo)
	if in.Carrier == "" || utf8.RuneCountInString(in.Carrier) > service.MaxCarrierRunes ||
		len(in.TrackingNo) > service.MaxTrackingNoBytes || !trackingNoPattern.MatchString(in.TrackingNo) {
		return nil, service.ErrInvalidArgument
	}
	order, err := s.Get(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order.Status == service.StatusPaid {
		if err := s.shippable(ctx, order); err != nil {
			return nil, err
		}
		now := s.now()
		deadline := now.Add(s.timing.AutoConfirm)
		changed, err := s.orders.Transition(ctx, order.ID, service.Change{
			From: []string{service.StatusPaid}, To: service.StatusShipped,
			Shipment: &in, ShippedAt: &now, ConfirmDeadline: &deadline,
		})
		if err != nil {
			return nil, err
		}
		if changed {
			logTransition(ctx, order.No, service.StatusPaid, service.StatusShipped, "")
		}
		// Changed or not, someone moved it on: answer from the row as it is.
		if order, err = s.Get(ctx, orderNo); err != nil {
			return nil, err
		}
	}
	shipped := order.Status == service.StatusShipped || order.Status == service.StatusCompleted
	if !shipped || order.Carrier != in.Carrier || order.TrackingNo != in.TrackingNo {
		return nil, service.ErrFailedPrecondition
	}
	return order, nil
}

// shippable refuses to ship while an after-sale request waits for review
// (the shopper may not want the goods any more: decide first), or when every
// goods line has been approved for refund (nothing left to send). Task 0018.
func (s *sOrder) shippable(ctx context.Context, order *service.Order) error {
	refunds, err := s.refunds.ByOrder(ctx, order.ID)
	if err != nil {
		return err
	}
	refunded := map[uint64]bool{}
	for _, r := range refunds {
		switch r.Status {
		case service.RefundPending:
			return service.ErrFailedPrecondition
		case service.RefundApproved, service.RefundRefunded:
			refunded[r.GoodsID] = true
		}
	}
	for _, item := range order.Items {
		if !refunded[item.GoodsID] {
			return nil
		}
	}
	return service.ErrFailedPrecondition
}

// ConfirmReceipt lets the shopper complete their own shipped order. Racing
// the worker's automatic confirmation is fine: whoever loses finds the order
// COMPLETED and returns it.
func (s *sOrder) ConfirmReceipt(ctx context.Context, userID uint64, orderNo string) (*service.Order, error) {
	order, err := s.GetMine(ctx, userID, orderNo)
	if err != nil {
		return nil, err
	}
	if order.Status == service.StatusShipped {
		now := s.now()
		changed, err := s.orders.Transition(ctx, order.ID, service.Change{
			From: []string{service.StatusShipped}, To: service.StatusCompleted,
			CompletedAt: &now, CompleteReason: service.CompleteUserConfirmed,
		})
		if err != nil {
			return nil, err
		}
		if changed {
			logTransition(ctx, order.No, service.StatusShipped, service.StatusCompleted, service.CompleteUserConfirmed)
		}
		if order, err = s.Get(ctx, orderNo); err != nil {
			return nil, err
		}
	}
	if order.Status != service.StatusCompleted {
		return nil, service.ErrFailedPrecondition
	}
	return order, nil
}
