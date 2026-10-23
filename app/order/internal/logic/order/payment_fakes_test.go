package order

import (
	"context"
	"sync"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

// fakePayments shares the orders' mutex: that is the "order row lock" which
// makes ConfirmPaid and Transition exclude each other, as in MySQL.
type fakePayments struct {
	orders   *fakeOrders
	payments map[string]*service.Payment
	refunds  map[string]*service.TechRefund // by payment id
}

func newFakePayments(orders *fakeOrders) *fakePayments {
	return &fakePayments{orders: orders, payments: map[string]*service.Payment{}, refunds: map[string]*service.TechRefund{}}
}

func (f *fakePayments) Create(_ context.Context, p *service.Payment) error {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	if _, ok := f.payments[p.ID]; !ok {
		c := *p
		f.payments[p.ID] = &c
	}
	return nil
}

func (f *fakePayments) Find(_ context.Context, id string) (*service.Payment, error) {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	if p, ok := f.payments[id]; ok {
		c := *p
		return &c, nil
	}
	return nil, nil
}

func (f *fakePayments) MarkClosed(_ context.Context, id string) error {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	if p, ok := f.payments[id]; ok && p.State == service.PaymentCreated {
		p.State = service.PaymentClosed
	}
	return nil
}

func (f *fakePayments) ConfirmPaid(_ context.Context, orderID uint64, p *service.Payment, paidAt time.Time) (string, error) {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	order, ok := f.orders.rows[orderID]
	if !ok {
		return "", service.ErrNotFound
	}
	stored, ok := f.payments[p.ID]
	if !ok {
		stored = &service.Payment{ID: p.ID, OrderID: orderID, AmountCents: p.AmountCents, State: service.PaymentCreated}
		f.payments[p.ID] = stored
	}
	if stored.State == service.PaymentCreated || stored.State == service.PaymentClosed {
		stored.State, stored.ProviderTxID = service.PaymentSucceeded, p.ProviderTxID
	}
	switch order.Status {
	case service.StatusPendingPayment:
		order.Status, order.PaidAt = service.StatusPaymentConfirming, &paidAt
		return service.PaidConfirming, nil
	case service.StatusPaymentConfirming, service.StatusPaid, service.StatusException:
		return service.PaidDuplicate, nil
	}
	if f.schedule(p.ID, orderID, p.AmountCents, service.RefundLatePayment) {
		return service.PaidLate, nil
	}
	return service.PaidDuplicate, nil
}

func (f *fakePayments) schedule(paymentID string, orderID, amount uint64, reason string) bool {
	if _, ok := f.refunds[paymentID]; ok {
		return false
	}
	f.refunds[paymentID] = &service.TechRefund{
		ID: service.TechRefundID(paymentID), PaymentID: paymentID, OrderID: orderID,
		AmountCents: amount, Reason: reason, State: service.TechRefundPending,
	}
	return true
}

func (f *fakePayments) Except(_ context.Context, orderID uint64, paymentID string) (bool, error) {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	order := f.orders.rows[orderID]
	if order.Status != service.StatusPaymentConfirming {
		return false, nil
	}
	order.Status, order.CloseReason = service.StatusException, service.ReasonInventoryConflict
	f.schedule(paymentID, orderID, f.payments[paymentID].AmountCents, service.RefundCommitFailed)
	return true, nil
}

func (f *fakePayments) DueRefunds(_ context.Context, _ int) ([]*service.TechRefund, error) {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	var out []*service.TechRefund
	for _, r := range f.refunds {
		if r.State == service.TechRefundPending {
			c := *r
			out = append(out, &c)
		}
	}
	return out, nil
}

func (f *fakePayments) Refunded(_ context.Context, refundID string) error {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	for _, r := range f.refunds {
		if r.ID != refundID || r.State != service.TechRefundPending {
			continue
		}
		r.State = service.TechRefundSucceeded
		f.payments[r.PaymentID].State = service.PaymentRefunded
		if order := f.orders.rows[r.OrderID]; order.Status == service.StatusException {
			order.Status = service.StatusClosing
		}
	}
	return nil
}

func (f *fakePayments) RefundAttempted(_ context.Context, refundID string) error {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	for _, r := range f.refunds {
		if r.ID == refundID {
			r.Attempts++
		}
	}
	return nil
}

func (f *fakePayments) refund(paymentID string) *service.TechRefund {
	f.orders.mu.Lock()
	defer f.orders.mu.Unlock()
	if r, ok := f.refunds[paymentID]; ok {
		c := *r
		return &c
	}
	return nil
}

// fakeProvider behaves like the simulator (one mutex = its row lock) and can
// be told to fail calls or to report a different amount.
type fakeProvider struct {
	mu       sync.Mutex
	now      func() time.Time
	payments map[string]*service.ProviderPayment
	refunds  map[string]string // refund id -> payment id
	fail     map[string]error  // method -> error for the next call
	calls    map[string]int
	tamper   uint64 // added to amounts in answers
}

func newFakeProvider(now func() time.Time) *fakeProvider {
	return &fakeProvider{now: now, payments: map[string]*service.ProviderPayment{}, refunds: map[string]string{},
		fail: map[string]error{}, calls: map[string]int{}}
}

func (f *fakeProvider) enter(method string) error {
	f.calls[method]++
	if err := f.fail[method]; err != nil {
		delete(f.fail, method)
		return err
	}
	return nil
}

func (f *fakeProvider) answer(p *service.ProviderPayment) *service.ProviderPayment {
	c := *p
	c.AmountCents += f.tamper
	return &c
}

func (f *fakeProvider) Create(_ context.Context, id, orderNo string, amount uint64, expiresAt time.Time) (*service.ProviderPayment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("create"); err != nil {
		return nil, err
	}
	p, ok := f.payments[id]
	if !ok {
		p = &service.ProviderPayment{ID: id, OrderNo: orderNo, AmountCents: amount, Currency: service.Currency,
			State: service.PaymentCreated, PayURL: "http://paysim/checkout/" + id, ExpiresAt: &expiresAt}
		f.payments[id] = p
	}
	return f.answer(p), nil
}

func (f *fakeProvider) Close(_ context.Context, id, orderNo string) (*service.ProviderPayment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("close"); err != nil {
		return nil, err
	}
	p, ok := f.payments[id]
	if !ok {
		p = &service.ProviderPayment{ID: id, OrderNo: orderNo, Currency: service.Currency, State: service.PaymentClosed}
		f.payments[id] = p
	}
	if p.State == service.PaymentCreated {
		p.State = service.PaymentClosed
	}
	return f.answer(p), nil
}

func (f *fakeProvider) Query(_ context.Context, id string) (*service.ProviderPayment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("query"); err != nil {
		return nil, err
	}
	p, ok := f.payments[id]
	if !ok {
		return nil, service.ErrNotFound
	}
	return f.answer(p), nil
}

func (f *fakeProvider) Refund(_ context.Context, refundID, id string, amount uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("refund"); err != nil {
		return err
	}
	if f.refunds[refundID] == id {
		return nil
	}
	p := f.payments[id]
	if p == nil || p.State != service.PaymentSucceeded || p.AmountCents != amount {
		return service.ErrFailedPrecondition
	}
	p.State = service.PaymentRefunded
	f.refunds[refundID] = id
	return nil
}

// pay is the payer paying on the checkout page. False if it was closed.
func (f *fakeProvider) pay(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.payments[id]
	if !ok || p.State != service.PaymentCreated {
		return p != nil && p.Paid()
	}
	now := f.now()
	p.State, p.TxID, p.PaidAt = service.PaymentSucceeded, "T-"+id, &now
	return true
}

func (f *fakeProvider) state(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.payments[id]; ok {
		return p.State
	}
	return ""
}

func (f *fakeProvider) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method]
}

// fakeNotices accepts the signature "good" and nothing else.
type fakeNotices struct{}

func (fakeNotices) Verify(_ []byte, signature string) bool { return signature == "good" }

// operations counts the distinct operation ids sent for kind: what the goods
// service would actually apply.
func (f *fakeInventory) operations(kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := map[string]bool{}
	for _, c := range f.calls {
		if c.kind == kind {
			seen[c.operation] = true
		}
	}
	return len(seen)
}
