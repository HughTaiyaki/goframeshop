package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/model/entity"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
)

type cartRepository struct {
	now func() time.Time
}

// NewCart returns the MySQL-backed cart repository.
func NewCart() service.CartRepository {
	return &cartRepository{now: time.Now}
}

// stamp is the time written to created_at and updated_at. It is cut to the
// column's millisecond precision here, so MySQL never rounds it up past the
// real instant: order.placed compares updated_at with the order time.
func stamp(now func() time.Time) time.Time {
	return now().Truncate(time.Millisecond)
}

func (r *cartRepository) List(ctx context.Context, userID uint64, page, size int) ([]*service.CartItem, error) {
	var rows []*entity.CartItem
	err := dao.CartItem.Ctx(ctx).
		Where(dao.CartItem.Columns().UserId, userID).
		OrderDesc(dao.CartItem.Columns().Id).
		Page(page, size).
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	items := make([]*service.CartItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toCartItem(row))
	}
	return items, nil
}

func (r *cartRepository) Count(ctx context.Context, userID uint64) (int, error) {
	return dao.CartItem.Ctx(ctx).Where(dao.CartItem.Columns().UserId, userID).Count()
}

func (r *cartRepository) Find(ctx context.Context, userID, itemID uint64) (*service.CartItem, error) {
	columns := dao.CartItem.Columns()
	return r.one(dao.CartItem.Ctx(ctx).Where(columns.UserId, userID).Where(columns.Id, itemID))
}

func (r *cartRepository) FindByGoods(ctx context.Context, userID, goodsID uint64) (*service.CartItem, error) {
	columns := dao.CartItem.Columns()
	return r.one(dao.CartItem.Ctx(ctx).Where(columns.UserId, userID).Where(columns.GoodsId, goodsID))
}

func (r *cartRepository) one(model *gdb.Model) (*service.CartItem, error) {
	var row *entity.CartItem
	if err := model.Scan(&row); err != nil || row == nil {
		return nil, ignoreNoRows(err)
	}
	return toCartItem(row), nil
}

// addSQL inserts the row or adds to it in one statement, so two adds of the
// same goods at once still end in one row with the sum. When the sum would
// pass the limit the row is left exactly as it was (updated_at included,
// which is why it is assigned first: MySQL applies the assignments left to
// right, so quantity must still be the old value when updated_at is decided),
// and the statement affects 0 rows.
const addSQL = "INSERT INTO `cart_item` (`user_id`, `goods_id`, `quantity`, `created_at`, `updated_at`) " +
	"VALUES (?, ?, ?, ?, ?) AS `new` ON DUPLICATE KEY UPDATE " +
	"`updated_at` = IF(`cart_item`.`quantity` + `new`.`quantity` <= ?, `new`.`updated_at`, `cart_item`.`updated_at`), " +
	"`quantity` = IF(`cart_item`.`quantity` + `new`.`quantity` <= ?, `cart_item`.`quantity` + `new`.`quantity`, `cart_item`.`quantity`)"

func (r *cartRepository) Add(ctx context.Context, userID, goodsID uint64, quantity, max uint32) (*service.CartItem, error) {
	if quantity > max {
		return nil, service.ErrCartLimit
	}
	now := stamp(r.now)
	result, err := dao.CartItem.DB().Exec(ctx, addSQL, userID, goodsID, quantity, now, now, max, max)
	if err != nil {
		return nil, err
	}
	// 1: inserted, 2: added to the existing row, 0: left unchanged.
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, service.ErrCartLimit
	}
	return r.FindByGoods(ctx, userID, goodsID)
}

func (r *cartRepository) SetQuantity(ctx context.Context, userID, itemID uint64, quantity uint32) (bool, error) {
	columns := dao.CartItem.Columns()
	result, err := dao.CartItem.Ctx(ctx).
		Data(g.Map{columns.Quantity: quantity, columns.UpdatedAt: stamp(r.now)}).
		Where(columns.UserId, userID).
		Where(columns.Id, itemID).
		Update()
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *cartRepository) Delete(ctx context.Context, userID uint64, itemIDs []uint64) (int, error) {
	if len(itemIDs) == 0 {
		return 0, nil
	}
	columns := dao.CartItem.Columns()
	result, err := dao.CartItem.Ctx(ctx).
		Where(columns.UserId, userID).
		WhereIn(columns.Id, itemIDs).
		Delete()
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

// Statements of RemoveOrderedTx. Both only touch rows last changed at or
// before the order: a row the shopper changed afterwards is a new intent.
// The decrement sets updated_at to itself on purpose: left out, MySQL's ON
// UPDATE would move it to now, and a second order placed before now would
// then skip the row.
const (
	cartDecrementSQL = "UPDATE `cart_item` SET `quantity` = `quantity` - ?, `updated_at` = `updated_at` " +
		"WHERE `user_id` = ? AND `goods_id` = ? AND `updated_at` <= ? AND `quantity` > ?"
	cartRemoveSQL = "DELETE FROM `cart_item` " +
		"WHERE `user_id` = ? AND `goods_id` = ? AND `updated_at` <= ? AND `quantity` <= ?"
)

// RemoveOrderedTx takes the goods of an order checked out from the cart out
// of the shopper's cart: each row loses the bought quantity, and goes when
// nothing is left. It runs in the inbox transaction of order.placed, and
// reports how many rows it changed or removed.
func RemoveOrderedTx(ctx context.Context, tx gdb.TX, placed eventbus.OrderPlaced) (int, error) {
	at := placed.PlacedAt
	changed := 0
	for _, item := range placed.Items {
		// Remove before decrementing: the other way round, a row left with
		// no more than the bought quantity after the decrement (2 in the
		// cart, 1 bought) would match the delete as well and vanish. Each
		// row matches exactly one of the two.
		statements := []struct {
			sql  string
			args []any
		}{
			{cartRemoveSQL, []any{placed.UserID, item.GoodsID, at, item.Quantity}},
			{cartDecrementSQL, []any{item.Quantity, placed.UserID, item.GoodsID, at, item.Quantity}},
		}
		for _, st := range statements {
			result, err := tx.Exec(st.sql, st.args...)
			if err != nil {
				return 0, err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return 0, err
			}
			changed += int(n)
		}
	}
	return changed, nil
}

// ignoreNoRows is for Scan into a nil pointer, which reports sql.ErrNoRows
// in some GoFrame versions and nothing in others.
func ignoreNoRows(err error) error {
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func toCartItem(row *entity.CartItem) *service.CartItem {
	item := &service.CartItem{
		ID:       row.Id,
		UserID:   row.UserId,
		GoodsID:  row.GoodsId,
		Quantity: uint32(row.Quantity),
	}
	if row.CreatedAt != nil {
		item.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt != nil {
		item.UpdatedAt = row.UpdatedAt.Time
	}
	return item
}
