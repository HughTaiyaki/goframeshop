package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/gogf/gf/v2/database/gdb"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/do"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// mysqlDeadlock is ER_LOCK_DEADLOCK: InnoDB rolled this transaction back to
// break a lock cycle. The whole transaction can safely be run again.
const mysqlDeadlock = 1213

const deadlockRetries = 3

type ledger struct{}

// NewLedger returns the MySQL-backed reservation ledger.
func NewLedger() service.Ledger {
	return &ledger{}
}

// InTx runs fn at READ COMMITTED. At the default REPEATABLE READ, a locking
// read of a reservation that does not exist yet takes a gap lock, and two
// transactions inserting into neighbouring gaps deadlock each other. The
// callers' "inventory rows first, in goods_id order" discipline is what keeps
// READ COMMITTED correct here.
func (l *ledger) InTx(ctx context.Context, fn func(ctx context.Context, tx service.LedgerTx) error) error {
	var err error
	for attempt := 0; attempt < deadlockRetries; attempt++ {
		err = dao.Inventory.DB().TransactionWithOptions(ctx,
			gdb.TxOptions{Propagation: gdb.PropagationRequired, Isolation: sql.LevelReadCommitted},
			func(ctx context.Context, tx gdb.TX) error {
				return fn(ctx, &ledgerTx{ctx: ctx, tx: tx})
			})
		if !isDeadlock(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return err
}

func (l *ledger) FindOperation(ctx context.Context, operationID string) (*service.Operation, error) {
	return findOperation(dao.InventoryOperation.Ctx(ctx), operationID)
}

func (l *ledger) Reservations(ctx context.Context, orderID string) ([]service.Reservation, error) {
	return reservations(dao.Reservation.Ctx(ctx), orderID)
}

type ledgerTx struct {
	ctx context.Context
	tx  gdb.TX
}

func (t *ledgerTx) FindOperation(operationID string) (*service.Operation, error) {
	return findOperation(dao.InventoryOperation.Ctx(t.ctx).TX(t.tx), operationID)
}

func (t *ledgerTx) InsertOperation(op *service.Operation) error {
	_, err := dao.InventoryOperation.Ctx(t.ctx).TX(t.tx).Data(do.InventoryOperation{
		OperationId: op.ID,
		Kind:        op.Kind,
		OrderId:     op.OrderID,
		RequestHash: op.RequestHash,
		Outcome:     "PENDING", // overwritten before commit; never visible outside
	}).Insert()
	if utility.IsDuplicateKey(err) {
		return service.ErrOperationExists
	}
	return err
}

func (t *ledgerTx) SetOperationOutcome(operationID, outcome string) error {
	_, err := dao.InventoryOperation.Ctx(t.ctx).TX(t.tx).
		Data(do.InventoryOperation{Outcome: outcome}).
		Where(dao.InventoryOperation.Columns().OperationId, operationID).
		Update()
	return err
}

func (t *ledgerTx) LockReservation(orderID string, goodsID uint64) (*service.Reservation, error) {
	columns := dao.Reservation.Columns()
	one, err := dao.Reservation.Ctx(t.ctx).TX(t.tx).
		Where(columns.OrderId, orderID).
		Where(columns.GoodsId, goodsID).
		LockUpdate().
		One()
	if err != nil || one.IsEmpty() {
		return nil, err
	}
	var row entity.Reservation
	if err := one.Struct(&row); err != nil {
		return nil, err
	}
	r := toReservation(&row)
	return &r, nil
}

func (t *ledgerTx) LockReservations(orderID string) ([]service.Reservation, error) {
	return reservations(dao.Reservation.Ctx(t.ctx).TX(t.tx).LockUpdate(), orderID)
}

func (t *ledgerTx) ReservedGoods(orderID string) ([]uint64, error) {
	columns := dao.Reservation.Columns()
	values, err := dao.Reservation.Ctx(t.ctx).TX(t.tx).
		Fields(columns.GoodsId).
		Where(columns.OrderId, orderID).
		Array()
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(values))
	for _, v := range values {
		ids = append(ids, v.Uint64())
	}
	return ids, nil
}

func (t *ledgerTx) InsertReservation(r *service.Reservation) error {
	_, err := dao.Reservation.Ctx(t.ctx).TX(t.tx).Data(do.Reservation{
		OrderId:  r.OrderID,
		GoodsId:  r.GoodsID,
		Quantity: r.Quantity,
		State:    r.State,
	}).Insert()
	return err
}

func (t *ledgerTx) SetReservationState(id uint64, state string) error {
	_, err := dao.Reservation.Ctx(t.ctx).TX(t.tx).
		Data(do.Reservation{State: state}).
		Where(dao.Reservation.Columns().Id, id).
		Update()
	return err
}

func (t *ledgerTx) LockStock(goodsID uint64) (service.Stock, bool, error) {
	one, err := dao.Inventory.Ctx(t.ctx).TX(t.tx).
		Where(dao.Inventory.Columns().GoodsId, goodsID).
		LockUpdate().
		One()
	if err != nil || one.IsEmpty() {
		return service.Stock{}, false, err
	}
	var row entity.Inventory
	if err := one.Struct(&row); err != nil {
		return service.Stock{}, false, err
	}
	return toStock(&row), true, nil
}

// MoveStock applies signed deltas in one statement. The deltas are integers
// formatted by this code, never caller text. The columns are UNSIGNED, so a
// bug that would drive one negative fails the statement instead of wrapping.
func (t *ledgerTx) MoveStock(goodsID uint64, available, reserved, sold int64) error {
	columns := dao.Inventory.Columns()
	_, err := dao.Inventory.Ctx(t.ctx).TX(t.tx).
		Data(gdb.Map{
			columns.Available: gdb.Raw(fmt.Sprintf("`%s` + (%d)", columns.Available, available)),
			columns.Reserved:  gdb.Raw(fmt.Sprintf("`%s` + (%d)", columns.Reserved, reserved)),
			columns.Sold:      gdb.Raw(fmt.Sprintf("`%s` + (%d)", columns.Sold, sold)),
		}).
		Where(columns.GoodsId, goodsID).
		Update()
	return err
}

// Sellable checks the product inside the transaction. goods_info has a
// deleted_at column, so GoFrame excludes deleted rows automatically.
func (t *ledgerTx) Sellable(goodsID uint64) (bool, error) {
	columns := dao.GoodsInfo.Columns()
	count, err := dao.GoodsInfo.Ctx(t.ctx).TX(t.tx).
		Where(columns.Id, goodsID).
		Where(columns.Status, service.StatusOnShelf).
		Count()
	return count > 0, err
}

func findOperation(model *gdb.Model, operationID string) (*service.Operation, error) {
	one, err := model.Where(dao.InventoryOperation.Columns().OperationId, operationID).One()
	if err != nil || one.IsEmpty() {
		return nil, err
	}
	var row entity.InventoryOperation
	if err := one.Struct(&row); err != nil {
		return nil, err
	}
	return &service.Operation{
		ID: row.OperationId, Kind: row.Kind, OrderID: row.OrderId,
		RequestHash: row.RequestHash, Outcome: row.Outcome,
	}, nil
}

func reservations(model *gdb.Model, orderID string) ([]service.Reservation, error) {
	columns := dao.Reservation.Columns()
	all, err := model.Where(columns.OrderId, orderID).OrderAsc(columns.GoodsId).All()
	if err != nil {
		return nil, err
	}
	var rows []*entity.Reservation
	if err := all.Structs(&rows); err != nil {
		return nil, err
	}
	out := make([]service.Reservation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toReservation(row))
	}
	return out, nil
}

func toReservation(row *entity.Reservation) service.Reservation {
	return service.Reservation{
		ID: row.Id, OrderID: row.OrderId, GoodsID: row.GoodsId,
		Quantity: uint32(row.Quantity), State: row.State,
	}
}

func toStock(row *entity.Inventory) service.Stock {
	return service.Stock{Available: uint32(row.Available), Reserved: uint32(row.Reserved), Sold: uint32(row.Sold)}
}

func isDeadlock(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDeadlock
}
