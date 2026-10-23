// Package inventory implements stock reservation for the order flow.
//
// Every call follows the same shape, inside one transaction:
//
//  1. Record the operation id first. A concurrent call with the same id
//     queues on that row; a later retry finds it and returns the stored
//     outcome without acting again.
//  2. Lock the inventory rows of every goods involved, in goods_id order,
//     then the order's reservation rows. All work on one goods is therefore
//     serialised, and every transaction takes locks in the same order.
//  3. Check everything before changing anything: a call either applies to
//     all items or to none.
//  4. Store the outcome with the stock change and commit.
//
// State rules (docs/GoFrame微服务商城详细设计.md §6.3):
//
//	current     Reserve (same order)   Commit              Release
//	none        reserve                NOT_READY           write RELEASED tombstone
//	RESERVED    no change              -> COMMITTED, sold  -> RELEASED, available
//	COMMITTED   no change              no change           STATE_CONFLICT
//	RELEASED    ALREADY_RELEASED       STATE_CONFLICT      no change
package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type sInventory struct {
	ledger service.Ledger
}

func New(ledger service.Ledger) service.IInventory {
	return &sInventory{ledger: ledger}
}

// errNotReady rolls back a Commit that found nothing to commit, so that no
// operation record is stored and the same id can be retried later.
var errNotReady = errors.New("nothing to commit yet")

func (s *sInventory) Reserve(ctx context.Context, operationID, orderID string, items []service.InventoryItem) (*service.OperationResult, error) {
	items, err := normalize(operationID, orderID, items)
	if err != nil {
		return nil, err
	}
	op := &service.Operation{ID: operationID, Kind: service.KindReserve, OrderID: orderID, RequestHash: hash(service.KindReserve, orderID, items)}
	return s.run(ctx, op, func(tx service.LedgerTx) (string, error) {
		// Lock order: inventory rows (goods_id order), then reservation rows.
		stocks, err := lockStocks(tx, goodsIDs(items))
		if err != nil {
			return "", err
		}
		// Pass 1: decide, without changing anything.
		toReserve := make([]service.InventoryItem, 0, len(items))
		for _, item := range items {
			existing, err := tx.LockReservation(orderID, item.GoodsID)
			if err != nil {
				return "", err
			}
			if existing != nil {
				switch {
				case existing.State == service.StateReleased:
					return service.OutcomeAlreadyReleased, nil
				case existing.Quantity != item.Quantity:
					return service.OutcomeStateConflict, nil
				default:
					continue // already held for this order: nothing to do
				}
			}
			toReserve = append(toReserve, item)
		}
		for _, item := range toReserve {
			stock, ok := stocks[item.GoodsID]
			if !ok {
				return service.OutcomeGoodsUnavailable, nil
			}
			sellable, err := tx.Sellable(item.GoodsID)
			if err != nil {
				return "", err
			}
			if !sellable {
				return service.OutcomeGoodsUnavailable, nil
			}
			if stock.Available < item.Quantity {
				return service.OutcomeInsufficientStock, nil
			}
		}
		// Pass 2: apply.
		for _, item := range toReserve {
			q := int64(item.Quantity)
			if err := tx.MoveStock(item.GoodsID, -q, q, 0); err != nil {
				return "", err
			}
			if err := tx.InsertReservation(&service.Reservation{
				OrderID: orderID, GoodsID: item.GoodsID, Quantity: item.Quantity, State: service.StateReserved,
			}); err != nil {
				return "", err
			}
		}
		return service.OutcomeSucceeded, nil
	})
}

func (s *sInventory) Commit(ctx context.Context, operationID, orderID string) (*service.OperationResult, error) {
	if !validID(operationID) || !validID(orderID) {
		return nil, service.ErrInvalidArgument
	}
	op := &service.Operation{ID: operationID, Kind: service.KindCommit, OrderID: orderID, RequestHash: hash(service.KindCommit, orderID, nil)}
	result, err := s.run(ctx, op, func(tx service.LedgerTx) (string, error) {
		ids, err := tx.ReservedGoods(orderID)
		if err != nil {
			return "", err
		}
		if _, err := lockStocks(tx, ids); err != nil {
			return "", err
		}
		reservations, err := tx.LockReservations(orderID)
		if err != nil {
			return "", err
		}
		if len(reservations) == 0 {
			return "", errNotReady
		}
		for _, r := range reservations {
			if r.State == service.StateReleased {
				return service.OutcomeStateConflict, nil
			}
		}
		for _, r := range reservations {
			if r.State != service.StateReserved {
				continue // already committed
			}
			q := int64(r.Quantity)
			if err := tx.MoveStock(r.GoodsID, 0, -q, q); err != nil {
				return "", err
			}
			if err := tx.SetReservationState(r.ID, service.StateCommitted); err != nil {
				return "", err
			}
		}
		return service.OutcomeSucceeded, nil
	})
	if errors.Is(err, errNotReady) {
		return &service.OperationResult{OperationID: operationID, Outcome: service.OutcomeNotReady}, nil
	}
	return result, err
}

func (s *sInventory) Release(ctx context.Context, operationID, orderID string, items []service.InventoryItem) (*service.OperationResult, error) {
	items, err := normalize(operationID, orderID, items)
	if err != nil {
		return nil, err
	}
	op := &service.Operation{ID: operationID, Kind: service.KindRelease, OrderID: orderID, RequestHash: hash(service.KindRelease, orderID, items)}
	return s.run(ctx, op, func(tx service.LedgerTx) (string, error) {
		if _, err := lockStocks(tx, goodsIDs(items)); err != nil {
			return "", err
		}
		existing := make([]*service.Reservation, len(items))
		for i, item := range items {
			r, err := tx.LockReservation(orderID, item.GoodsID)
			if err != nil {
				return "", err
			}
			// Sold stock is never handed back by a Release.
			if r != nil && r.State == service.StateCommitted {
				return service.OutcomeStateConflict, nil
			}
			existing[i] = r
		}
		for i, item := range items {
			r := existing[i]
			switch {
			case r == nil:
				// The Reserve has not arrived (or never will). Leave a
				// tombstone so that if it arrives late it is refused.
				if err := tx.InsertReservation(&service.Reservation{
					OrderID: orderID, GoodsID: item.GoodsID, Quantity: item.Quantity, State: service.StateReleased,
				}); err != nil {
					return "", err
				}
			case r.State == service.StateReserved:
				q := int64(r.Quantity)
				if err := tx.MoveStock(r.GoodsID, q, -q, 0); err != nil {
					return "", err
				}
				if err := tx.SetReservationState(r.ID, service.StateReleased); err != nil {
					return "", err
				}
			}
			// RELEASED already: nothing to do.
		}
		return service.OutcomeSucceeded, nil
	})
}

func (s *sInventory) Query(ctx context.Context, operationID string) (*service.OperationResult, error) {
	op, err := s.ledger.FindOperation(ctx, operationID)
	if err != nil || op == nil {
		return nil, err
	}
	reservations, err := s.ledger.Reservations(ctx, op.OrderID)
	if err != nil {
		return nil, err
	}
	return &service.OperationResult{OperationID: op.ID, Outcome: op.Outcome, Reservations: reservations}, nil
}

// run executes decide inside a transaction behind the idempotency record.
//
// If the operation id is already recorded, the stored outcome is returned
// (or ErrIdempotencyConflict when the parameters differ) and decide never
// runs. If another transaction records the same id while this one is in
// flight, this one rolls back and the stored result is read on the next pass.
func (s *sInventory) run(ctx context.Context, op *service.Operation,
	decide func(tx service.LedgerTx) (string, error)) (*service.OperationResult, error) {
	for range 2 {
		var result *service.OperationResult
		err := s.ledger.InTx(ctx, func(ctx context.Context, tx service.LedgerTx) error {
			stored, err := tx.FindOperation(op.ID)
			if err != nil {
				return err
			}
			if stored == nil {
				if err := tx.InsertOperation(op); err != nil {
					return err
				}
				outcome, err := decide(tx)
				if err != nil {
					return err
				}
				if err := tx.SetOperationOutcome(op.ID, outcome); err != nil {
					return err
				}
				stored = &service.Operation{ID: op.ID, OrderID: op.OrderID, Kind: op.Kind, RequestHash: op.RequestHash, Outcome: outcome}
			}
			if stored.Kind != op.Kind || stored.RequestHash != op.RequestHash {
				return service.ErrIdempotencyConflict
			}
			reservations, err := tx.LockReservations(op.OrderID)
			if err != nil {
				return err
			}
			result = &service.OperationResult{OperationID: op.ID, Outcome: stored.Outcome, Reservations: reservations}
			return nil
		})
		if errors.Is(err, service.ErrOperationExists) {
			continue // lost the race on the operation id: read the winner's result
		}
		return result, err
	}
	return nil, fmt.Errorf("operation %s: concurrent duplicate did not settle", op.ID)
}

// lockStocks locks the inventory rows of ids in ascending order and returns
// the ones that exist. Every operation calls it before touching reservations.
func lockStocks(tx service.LedgerTx, ids []uint64) (map[uint64]service.Stock, error) {
	sorted := append([]uint64(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	stocks := make(map[uint64]service.Stock, len(sorted))
	for _, id := range sorted {
		stock, ok, err := tx.LockStock(id)
		if err != nil {
			return nil, err
		}
		if ok {
			stocks[id] = stock
		}
	}
	return stocks, nil
}

func goodsIDs(items []service.InventoryItem) []uint64 {
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.GoodsID)
	}
	return ids
}

// normalize validates ids and a non-empty item list, merges duplicate goods and sorts by
// goods_id. The sort is what makes every transaction lock rows in the same
// order; the merge makes {A:1, A:1} and {A:2} the same request.
func normalize(operationID, orderID string, items []service.InventoryItem) ([]service.InventoryItem, error) {
	if !validID(operationID) || !validID(orderID) {
		return nil, service.ErrInvalidArgument
	}
	if len(items) == 0 || len(items) > service.MaxInventoryItems {
		return nil, service.ErrInvalidArgument
	}
	merged := map[uint64]uint32{}
	for _, item := range items {
		if item.GoodsID == 0 || item.Quantity == 0 || item.Quantity > service.MaxInventoryQuantity {
			return nil, service.ErrInvalidArgument
		}
		merged[item.GoodsID] += item.Quantity
		if merged[item.GoodsID] > service.MaxInventoryQuantity {
			return nil, service.ErrInvalidArgument
		}
	}
	out := make([]service.InventoryItem, 0, len(merged))
	for goodsID, quantity := range merged {
		out = append(out, service.InventoryItem{GoodsID: goodsID, Quantity: quantity})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GoodsID < out[j].GoodsID })
	return out, nil
}

func validID(id string) bool {
	return id != "" && len(id) <= service.MaxIDLength && strings.TrimSpace(id) == id
}

// hash is the canonical fingerprint of a request: kind, order and the
// normalized items. Same id + same hash = a retry; same id + different hash =
// a caller bug.
func hash(kind, orderID string, items []service.InventoryItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n", kind, orderID)
	for _, item := range items {
		fmt.Fprintf(&b, "%d:%d\n", item.GoodsID, item.Quantity)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
