package testfake

import (
	"context"
	"sort"
	"sync"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

// Ledger is an in-memory service.Ledger. One mutex around every transaction
// makes them serialisable; a transaction that returns an error is rolled back
// by restoring a snapshot. The real MySQL ledger's concurrency is covered by
// the integration test in repository.
type Ledger struct {
	mu         sync.Mutex
	Stocks     map[uint64]service.Stock
	Sellable   map[uint64]bool
	Operations map[string]service.Operation
	Rows       []service.Reservation
	nextID     uint64
	// RaceOn, when set, makes the next InsertOperation of that id fail as if
	// another transaction had recorded it first with RaceOutcome and
	// committed, to exercise the "lost the race" path.
	RaceOn      string
	RaceOutcome string
	// winner is the competitor's committed record; it survives our rollback.
	winner *service.Operation
}

func NewLedger() *Ledger {
	return &Ledger{Stocks: map[uint64]service.Stock{}, Sellable: map[uint64]bool{}, Operations: map[string]service.Operation{}}
}

// AddGoods registers sellable goods with some available stock.
func (l *Ledger) AddGoods(goodsID uint64, available uint32) {
	l.Stocks[goodsID] = service.Stock{Available: available}
	l.Sellable[goodsID] = true
}

func (l *Ledger) InTx(ctx context.Context, fn func(ctx context.Context, tx service.LedgerTx) error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	snapshot := l.copyState()
	err := fn(ctx, &ledgerTx{l})
	if err != nil {
		l.restore(snapshot)
	}
	if l.winner != nil {
		l.Operations[l.winner.ID] = *l.winner
		l.winner = nil
	}
	return err
}

func (l *Ledger) FindOperation(_ context.Context, id string) (*service.Operation, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if op, ok := l.Operations[id]; ok {
		return &op, nil
	}
	return nil, nil
}

func (l *Ledger) Reservations(_ context.Context, orderID string) ([]service.Reservation, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.byOrder(orderID), nil
}

func (l *Ledger) byOrder(orderID string) []service.Reservation {
	out := []service.Reservation{}
	for _, r := range l.Rows {
		if r.OrderID == orderID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GoodsID < out[j].GoodsID })
	return out
}

type state struct {
	stocks       map[uint64]service.Stock
	operations   map[string]service.Operation
	reservations []service.Reservation
	nextID       uint64
}

func (l *Ledger) copyState() state {
	s := state{stocks: map[uint64]service.Stock{}, operations: map[string]service.Operation{}, nextID: l.nextID}
	for k, v := range l.Stocks {
		s.stocks[k] = v
	}
	for k, v := range l.Operations {
		s.operations[k] = v
	}
	s.reservations = append([]service.Reservation(nil), l.Rows...)
	return s
}

func (l *Ledger) restore(s state) {
	l.Stocks, l.Operations, l.Rows, l.nextID = s.stocks, s.operations, s.reservations, s.nextID
}

type ledgerTx struct{ l *Ledger }

func (t *ledgerTx) FindOperation(id string) (*service.Operation, error) {
	if op, ok := t.l.Operations[id]; ok {
		return &op, nil
	}
	return nil, nil
}

func (t *ledgerTx) InsertOperation(op *service.Operation) error {
	if t.l.RaceOn == op.ID {
		// Pretend a concurrent transaction recorded this id and committed.
		winner := *op
		winner.Outcome = t.l.RaceOutcome
		t.l.RaceOn = ""
		t.l.winner = &winner
		return service.ErrOperationExists
	}
	if _, ok := t.l.Operations[op.ID]; ok {
		return service.ErrOperationExists
	}
	t.l.Operations[op.ID] = *op
	return nil
}

func (t *ledgerTx) SetOperationOutcome(id, outcome string) error {
	op := t.l.Operations[id]
	op.Outcome = outcome
	t.l.Operations[id] = op
	return nil
}

func (t *ledgerTx) LockReservation(orderID string, goodsID uint64) (*service.Reservation, error) {
	for _, r := range t.l.Rows {
		if r.OrderID == orderID && r.GoodsID == goodsID {
			copied := r
			return &copied, nil
		}
	}
	return nil, nil
}

func (t *ledgerTx) LockReservations(orderID string) ([]service.Reservation, error) {
	return t.l.byOrder(orderID), nil
}

func (t *ledgerTx) ReservedGoods(orderID string) ([]uint64, error) {
	ids := []uint64{}
	for _, r := range t.l.byOrder(orderID) {
		ids = append(ids, r.GoodsID)
	}
	return ids, nil
}

func (t *ledgerTx) InsertReservation(r *service.Reservation) error {
	t.l.nextID++
	stored := *r
	stored.ID = t.l.nextID
	t.l.Rows = append(t.l.Rows, stored)
	return nil
}

func (t *ledgerTx) SetReservationState(id uint64, state string) error {
	for i := range t.l.Rows {
		if t.l.Rows[i].ID == id {
			t.l.Rows[i].State = state
		}
	}
	return nil
}

func (t *ledgerTx) LockStock(goodsID uint64) (service.Stock, bool, error) {
	stock, ok := t.l.Stocks[goodsID]
	return stock, ok, nil
}

func (t *ledgerTx) MoveStock(goodsID uint64, available, reserved, sold int64) error {
	s := t.l.Stocks[goodsID]
	next := [3]int64{int64(s.Available) + available, int64(s.Reserved) + reserved, int64(s.Sold) + sold}
	for _, n := range next {
		if n < 0 {
			panic("stock went negative: the logic let an impossible move through")
		}
	}
	t.l.Stocks[goodsID] = service.Stock{Available: uint32(next[0]), Reserved: uint32(next[1]), Sold: uint32(next[2])}
	return nil
}

func (t *ledgerTx) Sellable(goodsID uint64) (bool, error) {
	return t.l.Sellable[goodsID], nil
}
