package fault

import (
	"context"
	"fmt"
	"sort"

	"github.com/gogf/gf/v2/database/gdb"
)

// Stock is one inventory row.
type Stock struct {
	Available, Reserved, Sold int64
}

func (s Stock) Total() int64 { return s.Available + s.Reserved + s.Sold }

func (s *Shop) Stock(ctx context.Context, goodsID uint64) (Stock, error) {
	row, err := s.Goods.GetOne(ctx, "SELECT available, reserved, sold FROM inventory WHERE goods_id = ?", goodsID)
	if err != nil {
		return Stock{}, err
	}
	return Stock{row["available"].Int64(), row["reserved"].Int64(), row["sold"].Int64()}, nil
}

// settledStates are where an order may rest once the system has recovered;
// anything else is work still in flight or stuck.
var settledStates = map[string]bool{
	"PENDING_PAYMENT": true, "PAID": true, "SHIPPED": true, "COMPLETED": true, "CLOSED": true,
}

// reservationFor is what a reservation may be, given its order's state
// once settled (design §6.3).
var reservationFor = map[string]string{
	"PENDING_PAYMENT": "RESERVED",
	"PAID":            "COMMITTED", "SHIPPED": "COMMITTED", "COMPLETED": "COMMITTED",
	"CLOSED": "RELEASED",
}

// Invariants checks every goods in initial (goods id -> stock it started
// with) and every order that bought them, and returns the violations.
func (s *Shop) Invariants(ctx context.Context, initial map[uint64]int64) ([]string, error) {
	var bad []string
	ids := make([]uint64, 0, len(initial))
	for id := range initial {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	for _, id := range ids {
		st, err := s.Stock(ctx, id)
		if err != nil {
			return nil, err
		}
		if st.Total() != initial[id] {
			bad = append(bad, fmt.Sprintf("goods %d: available+reserved+sold = %d+%d+%d = %d, started with %d",
				id, st.Available, st.Reserved, st.Sold, st.Total(), initial[id]))
		}
		// The ledger agrees with its own reservations.
		sums, err := s.Goods.GetOne(ctx, "SELECT "+
			"COALESCE(SUM(CASE WHEN state='RESERVED' THEN quantity END),0) AS reserved, "+
			"COALESCE(SUM(CASE WHEN state='COMMITTED' THEN quantity END),0) AS sold "+
			"FROM reservation WHERE goods_id = ?", id)
		if err != nil {
			return nil, err
		}
		if sums["reserved"].Int64() != st.Reserved || sums["sold"].Int64() != st.Sold {
			bad = append(bad, fmt.Sprintf("goods %d: inventory says reserved %d sold %d, reservations say %d and %d",
				id, st.Reserved, st.Sold, sums["reserved"].Int64(), sums["sold"].Int64()))
		}
	}

	// Every order that bought these goods: settled, and its reservation
	// matches its state.
	rows, err := s.OrdersDB.GetAll(ctx, "SELECT DISTINCT o.order_no, o.status FROM order_info o "+
		"JOIN order_goods g ON g.order_id = o.id WHERE g.goods_id IN (?)", ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		no, status := row["order_no"].String(), row["status"].String()
		if !settledStates[status] {
			bad = append(bad, fmt.Sprintf("order %s: still %s", no, status))
			continue
		}
		states, err := s.Goods.GetArray(ctx, "SELECT DISTINCT state FROM reservation WHERE order_id = ?", no)
		if err != nil {
			return nil, err
		}
		want := reservationFor[status]
		for _, st := range states {
			if st.String() != want {
				bad = append(bad, fmt.Sprintf("order %s is %s but has a %s reservation", no, status, st.String()))
			}
		}
		if status != "CLOSED" && len(states) == 0 {
			bad = append(bad, fmt.Sprintf("order %s is %s without a reservation", no, status))
		}
	}

	// Money: at most one technical refund per payment, and never more
	// refunded than paid.
	over, err := s.Paysim.GetAll(ctx, "SELECT p.payment_id, p.amount_cents, SUM(r.amount_cents) AS refunded, COUNT(*) AS n "+
		"FROM payment p JOIN refund r ON r.payment_id = p.payment_id WHERE p.order_no IN (?) "+
		"GROUP BY p.payment_id, p.amount_cents HAVING refunded > p.amount_cents OR SUM(r.refund_id LIKE 'TR%') > 1",
		orderNos(rows))
	if err != nil {
		return nil, err
	}
	for _, row := range over {
		bad = append(bad, fmt.Sprintf("payment %s: %d refunds totalling %d of %d paid",
			row["payment_id"].String(), row["n"].Int(), row["refunded"].Int64(), row["amount_cents"].Int64()))
	}
	return bad, nil
}

func orderNos(rows gdb.Result) []string {
	out := []string{""} // IN () needs one value
	for _, r := range rows {
		out = append(out, r["order_no"].String())
	}
	return out
}
