// Package order implements placing, paying, cancelling and expiring orders,
// and keeps each order's stock reservation and payment in step with its state.
//
//	Create ──▶ RESERVING ──Reserve ok──▶ PENDING_PAYMENT ──paid──▶ PAYMENT_CONFIRMING ──Commit ok──▶ PAID
//	               │                            │                         └ Commit refused ──▶ EXCEPTION ──refunded──┐
//	               │                            └ expires / cancel ──▶ CLOSING ◀─────────────────────────────────────┘
//	               │ refused (no stock, off shelf) ──────────────────▶ CLOSED     │ payment closed (or paid → refund),
//	               └ deadline passed / cancel ──▶ CLOSING                         │ then Release ok
//	                                                                              ▼
//	                                                                            CLOSED
//
// The payment half is in paymentflow.go.
//
// Every state change is conditional on the current state (see Change), so a
// shopper cancelling and the worker expiring the same order cannot both win.
// Every inventory call uses an operation id derived from the order number
// ("reserve-<no>", "release-<no>"), so retrying after a lost response is safe:
// the goods service returns the original outcome instead of acting twice.
// When an inventory call fails without an answer, the order simply stays
// where it is and the recovery worker tries again.
package order

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel/attribute"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

// Deps are everything the order logic talks to.
type Deps struct {
	Orders    service.OrderRepository
	Payments  service.PaymentRepository
	Catalog   service.Catalog
	Addresses service.Addresses
	Inventory service.Inventory
	Provider  service.Provider
	Notices   service.NoticeVerifier
	Timing    service.Timing
	Now       func() time.Time
}

type sOrder struct {
	orders    service.OrderRepository
	payments  service.PaymentRepository
	catalog   service.Catalog
	addresses service.Addresses
	inventory service.Inventory
	provider  service.Provider
	notices   service.NoticeVerifier
	timing    service.Timing
	now       func() time.Time
}

func New(d Deps) service.IOrder {
	return &sOrder{
		orders: d.Orders, payments: d.Payments, catalog: d.Catalog, addresses: d.Addresses,
		inventory: d.Inventory, provider: d.Provider, notices: d.Notices, timing: d.Timing, now: d.Now,
	}
}

func (s *sOrder) Create(ctx context.Context, userID uint64, in *service.CreateInput) (*service.Order, error) {
	items, err := normalize(in)
	if err != nil {
		return nil, err
	}
	remark := strings.TrimSpace(in.Remark)
	hash := requestHash(items, in.ConsigneeID, remark)

	// A retry of the same submit returns the order it created.
	if existing, err := s.orders.FindByRequest(ctx, userID, in.RequestID); err != nil {
		return nil, err
	} else if existing != nil {
		return sameRequest(existing, hash)
	}

	// Prices and names come from the goods service, never from the client.
	order := &service.Order{
		UserID:      userID,
		RequestID:   in.RequestID,
		RequestHash: hash,
		Status:      service.StatusReserving,
		Remark:      remark,
	}
	for _, item := range items {
		goods, err := s.catalog.Goods(ctx, item.GoodsID)
		if errors.Is(err, service.ErrNotFound) {
			return nil, service.ErrGoodsUnavailable
		}
		if err != nil {
			return nil, err
		}
		subtotal, ok := multiply(goods.PriceCents, item.Quantity)
		if !ok || order.TotalCents+subtotal < order.TotalCents {
			return nil, service.ErrInvalidArgument
		}
		order.TotalCents += subtotal
		order.Items = append(order.Items, service.OrderItem{
			GoodsID:        item.GoodsID,
			GoodsName:      goods.Name,
			CoverURL:       goods.CoverURL,
			UnitPriceCents: goods.PriceCents,
			Quantity:       item.Quantity,
			SubtotalCents:  subtotal,
		})
	}
	// So does the address: from the shopper's own address book, read with
	// the shopper's own token.
	address, err := s.addresses.Consignee(ctx, in.ConsigneeID)
	if errors.Is(err, service.ErrNotFound) {
		return nil, service.ErrInvalidArgument
	}
	if err != nil {
		return nil, err
	}
	order.ConsigneeName, order.ConsigneePhone, order.ConsigneeAddress = address.Name, address.Phone, address.Address

	now := s.now()
	order.ReserveDeadline = now.Add(s.timing.ReserveTimeout)
	order.No = newOrderNo(now)
	order.ID, err = s.orders.Create(ctx, order)
	if errors.Is(err, service.ErrAlreadyExists) {
		// The same submit raced us and won; answer with its order.
		existing, findErr := s.orders.FindByRequest(ctx, userID, in.RequestID)
		if findErr != nil || existing == nil {
			return nil, err
		}
		return sameRequest(existing, hash)
	}
	if err != nil {
		return nil, err
	}

	s.reserve(ctx, order)
	return s.orders.FindByNo(ctx, order.No)
}

func sameRequest(existing *service.Order, hash string) (*service.Order, error) {
	if existing.RequestHash != hash {
		return nil, service.ErrIdempotencyConflict
	}
	return existing, nil
}

// reserve asks for the stock and moves the order on according to the answer.
// With no answer the order stays RESERVING for the worker.
func (s *sOrder) reserve(ctx context.Context, order *service.Order) {
	outcome, err := s.inventory.Reserve(ctx, "reserve-"+order.No, order.No, inputs(order.Items))
	if err != nil {
		g.Log().Warningf(ctx, "order %s: reserve outcome unknown, the worker will retry: %v", order.No, err)
		return
	}
	now := s.now()
	reserving := []string{service.StatusReserving}
	switch outcome {
	case service.OutcomeSucceeded:
		expires := now.Add(s.timing.PaymentTimeout)
		// The expiry message is a wake-up call at expiresAt, written with the
		// change itself; the recovery scan still closes the order if it is lost.
		wake := expires.Add(service.ExpiryGrace)
		s.transition(ctx, order, service.Change{
			From: reserving, To: service.StatusPendingPayment, ExpiresAt: &expires,
			Events: []eventbus.Event{{
				Type:        service.EventPaymentPending,
				AggregateID: order.No,
				Payload:     service.PaymentPending{OrderNo: order.No, ExpiresAt: expires},
				DeliverAt:   &wake,
			}},
		})
	case service.OutcomeInsufficientStock, service.OutcomeGoodsUnavailable:
		// Nothing was reserved, so there is nothing to release.
		s.transition(ctx, order, service.Change{From: reserving, To: service.StatusClosed, CloseReason: outcome, ClosedAt: &now})
	default:
		// ALREADY_RELEASED / STATE_CONFLICT: the order was cancelled while
		// the reservation was in flight. Make sure it is released.
		if s.transition(ctx, order, service.Change{From: reserving, To: service.StatusClosing, CloseReason: service.ReasonReserveConflict}) {
			s.close(ctx, order)
		}
	}
}

// close finishes a CLOSING order: first the payment, then the stock. An
// order that was never payable (no expiresAt) has no payment to settle.
// Otherwise the provider closes the payment, or tells us it was paid after
// all; then the payment is recorded and refunded (design §6.4: "已支付则退款
// 后释放"). Either way the stock goes back. With no answer from the provider
// the order stays CLOSING and the worker asks again.
func (s *sOrder) close(ctx context.Context, order *service.Order) {
	if order.ExpiresAt != nil {
		paymentID := service.PaymentID(order.No)
		p, err := s.provider.Close(ctx, paymentID, order.No)
		if err != nil {
			g.Log().Warningf(ctx, "order %s: closing payment %s failed, the worker will retry: %v", order.No, paymentID, err)
			return
		}
		if p.Paid() {
			if err := s.confirm(ctx, order, p); err != nil {
				return // logged; leave CLOSING until the payment is sorted out
			}
		} else if err := s.payments.MarkClosed(ctx, paymentID); err != nil {
			g.Log().Warningf(ctx, "order %s: record payment closed: %v", order.No, err)
		}
	}
	s.release(ctx, order)
}

// release hands the order's stock back and closes it. Releasing stock that
// was never reserved is harmless: the goods service records a tombstone so a
// late reservation for this order is refused.
func (s *sOrder) release(ctx context.Context, order *service.Order) {
	outcome, err := s.inventory.Release(ctx, "release-"+order.No, order.No, inputs(order.Items))
	if err != nil {
		g.Log().Warningf(ctx, "order %s: release outcome unknown, the worker will retry: %v", order.No, err)
		return
	}
	if outcome != service.OutcomeSucceeded {
		// STATE_CONFLICT means the stock was committed (sold): that needs a
		// person, not a retry loop.
		g.Log().Errorf(ctx, "order %s: release refused with %s; left CLOSING for manual review", order.No, outcome)
		return
	}
	now := s.now()
	s.transition(ctx, order, service.Change{From: []string{service.StatusClosing}, To: service.StatusClosed, ClosedAt: &now})
}

// transition reports whether this call made the change. A false result means
// someone else moved the order first, which is fine: they own the next step.
func (s *sOrder) transition(ctx context.Context, order *service.Order, c service.Change) bool {
	changed, err := s.orders.Transition(ctx, order.ID, c)
	if err != nil {
		g.Log().Warningf(ctx, "order %s: %v -> %s failed, the worker will retry: %v", order.No, c.From, c.To, err)
		return false
	}
	return changed
}

func (s *sOrder) Cancel(ctx context.Context, userID uint64, orderNo string) (*service.Order, error) {
	order, err := s.GetMine(ctx, userID, orderNo)
	if err != nil {
		return nil, err
	}
	switch order.Status {
	case service.StatusClosing, service.StatusClosed:
		return order, nil // already cancelled or closed: nothing to do
	case service.StatusReserving, service.StatusPendingPayment:
	default:
		return nil, service.ErrFailedPrecondition
	}
	changed := s.transition(ctx, order, service.Change{
		From:        []string{service.StatusReserving, service.StatusPendingPayment},
		To:          service.StatusClosing,
		CloseReason: service.ReasonUserCancelled,
	})
	if changed {
		s.close(ctx, order)
	}
	return s.orders.FindByNo(ctx, orderNo)
}

func (s *sOrder) Recover(ctx context.Context) error {
	now := s.now()
	due, err := s.orders.Due(ctx, now, 100)
	if err != nil {
		return err
	}
	for _, order := range due {
		// One trace per order driven: the scan itself runs on a quiet
		// context and records nothing, so an idle pass costs no spans.
		octx, span := telemetry.Root(ctx, "order.recover",
			attribute.String("order.no", order.No), attribute.String("order.status", order.Status))
		s.drive(octx, order, now)
		span.End()
	}
	return s.refundPending(ctx)
}

func (s *sOrder) Settle(ctx context.Context, orderNo string) error {
	order, err := s.Get(ctx, orderNo)
	if err != nil {
		return err
	}
	now := s.now()
	if order.Status == service.StatusPendingPayment && (order.ExpiresAt == nil || now.Before(*order.ExpiresAt)) {
		return nil // still payable: nothing to drive
	}
	s.drive(ctx, order, now)
	return nil
}

// drive takes one step for an order that is waiting on something. Every step
// is conditional or idempotent, so driving the same order twice at once (the
// scan and the expiry consumer) is safe.
func (s *sOrder) drive(ctx context.Context, order *service.Order, now time.Time) {
	switch order.Status {
	case service.StatusReserving:
		if now.Before(order.ReserveDeadline) {
			s.reserve(ctx, order) // same operation id: safe to repeat
			return
		}
		if s.transition(ctx, order, service.Change{
			From: []string{service.StatusReserving}, To: service.StatusClosing, CloseReason: service.ReasonReserveTimeout,
		}) {
			s.close(ctx, order)
		}
	case service.StatusPendingPayment:
		// Only expired ones get here (Due and Settle check expiresAt).
		if s.transition(ctx, order, service.Change{
			From: []string{service.StatusPendingPayment}, To: service.StatusClosing, CloseReason: service.ReasonPaymentTimeout,
		}) {
			s.close(ctx, order)
		}
	case service.StatusPaymentConfirming:
		s.commit(ctx, order)
	case service.StatusClosing:
		s.close(ctx, order)
	}
}

func (s *sOrder) GetMine(ctx context.Context, userID uint64, orderNo string) (*service.Order, error) {
	order, err := s.Get(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	// Someone else's order is NotFound, not Forbidden: the response must not
	// confirm that the number exists.
	if order.UserID != userID {
		return nil, service.ErrNotFound
	}
	return order, nil
}

func (s *sOrder) ListMine(ctx context.Context, userID uint64, status string, page, size int) ([]*service.Order, int, error) {
	if userID == 0 {
		return nil, 0, service.ErrInvalidArgument
	}
	return s.List(ctx, &service.ListFilter{UserID: userID, Status: status, Page: page, Size: size})
}

func (s *sOrder) Get(ctx context.Context, orderNo string) (*service.Order, error) {
	order, err := s.orders.FindByNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, service.ErrNotFound
	}
	return order, nil
}

func (s *sOrder) List(ctx context.Context, f *service.ListFilter) ([]*service.Order, int, error) {
	if f.Page < 1 || f.Size < 1 || f.Size > 50 {
		return nil, 0, service.ErrInvalidArgument
	}
	total, err := s.orders.Count(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	orders, err := s.orders.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// normalize validates the request, merges repeated goods and sorts lines by
// goods id, so {A:1, A:1} and {A:2} are the same order and hash the same.
func normalize(in *service.CreateInput) ([]service.ItemInput, error) {
	if in.RequestID == "" || len(in.RequestID) > service.MaxRequestIDBytes || strings.TrimSpace(in.RequestID) != in.RequestID ||
		in.ConsigneeID == 0 || utf8.RuneCountInString(strings.TrimSpace(in.Remark)) > service.MaxRemarkRunes || len(in.Items) == 0 {
		return nil, service.ErrInvalidArgument
	}
	merged := map[uint64]uint32{}
	for _, item := range in.Items {
		if item.GoodsID == 0 || item.Quantity == 0 {
			return nil, service.ErrInvalidArgument
		}
		merged[item.GoodsID] += item.Quantity
		if merged[item.GoodsID] > service.MaxLineQuantity {
			return nil, service.ErrInvalidArgument
		}
	}
	if len(merged) > service.MaxOrderLines {
		return nil, service.ErrInvalidArgument
	}
	items := make([]service.ItemInput, 0, len(merged))
	for id, quantity := range merged {
		items = append(items, service.ItemInput{GoodsID: id, Quantity: quantity})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GoodsID < items[j].GoodsID })
	return items, nil
}

func requestHash(items []service.ItemInput, consigneeID uint64, remark string) string {
	var b strings.Builder
	for _, item := range items {
		fmt.Fprintf(&b, "%d:%d\n", item.GoodsID, item.Quantity)
	}
	fmt.Fprintf(&b, "consignee:%d\nremark:%s\n", consigneeID, remark)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func inputs(items []service.OrderItem) []service.ItemInput {
	out := make([]service.ItemInput, 0, len(items))
	for _, item := range items {
		out = append(out, service.ItemInput{GoodsID: item.GoodsID, Quantity: item.Quantity})
	}
	return out
}

func multiply(price uint64, quantity uint32) (uint64, bool) {
	if price != 0 && uint64(quantity) > ^uint64(0)/price {
		return 0, false
	}
	return price * uint64(quantity), true
}

// newOrderNo is "O" + UTC timestamp + 8 random digits: sortable by time,
// unguessable within a second, 23 characters. The unique key on order_no is
// the real guarantee; a collision fails the insert rather than merging orders.
func newOrderNo(now time.Time) string {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return fmt.Sprintf("O%s%08d", now.UTC().Format("20060102150405"), n.Int64())
}
