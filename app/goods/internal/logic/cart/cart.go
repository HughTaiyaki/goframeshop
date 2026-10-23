// Package cart implements the shopper's cart.
//
// The cart stores only (goods, quantity). Name, price, cover and stock are
// read when the cart is shown, so they are always the current ones; the
// order service prices the order again from the goods service anyway.
package cart

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type sCart struct {
	cart   service.CartRepository
	goods  service.GoodsRepository
	images service.ImageRepository
	stocks service.StockRepository
}

func New(cart service.CartRepository, goods service.GoodsRepository, images service.ImageRepository,
	stocks service.StockRepository) service.ICart {
	return &sCart{cart: cart, goods: goods, images: images, stocks: stocks}
}

func (s *sCart) List(ctx context.Context, userID uint64, page, size int) ([]*service.CartLine, int, error) {
	if userID == 0 {
		return nil, 0, service.ErrNoIdentity
	}
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, service.ErrInvalidArgument
	}
	total, err := s.cart.Count(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.cart.List(ctx, userID, page, size)
	if err != nil {
		return nil, 0, err
	}
	lines, err := s.lines(ctx, items)
	return lines, total, err
}

// Add accepts goods on shelf only: anything else could not be ordered. A
// new row needs room in the cart; adding to an existing row does not. The
// line count is checked before the insert, so two adds racing at the limit
// may both pass: it is a soft cap against abuse, not an invariant.
func (s *sCart) Add(ctx context.Context, userID, goodsID uint64, quantity uint32) (*service.CartLine, error) {
	if userID == 0 {
		return nil, service.ErrNoIdentity
	}
	if goodsID == 0 || quantity < 1 || quantity > service.MaxCartQuantity {
		return nil, service.ErrInvalidArgument
	}
	goods, err := s.goods.FindByID(ctx, goodsID)
	if err != nil {
		return nil, err
	}
	if goods == nil || goods.Status != service.StatusOnShelf {
		return nil, service.ErrNotFound
	}
	existing, err := s.cart.FindByGoods(ctx, userID, goodsID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		count, err := s.cart.Count(ctx, userID)
		if err != nil {
			return nil, err
		}
		if count >= service.MaxCartLines {
			return nil, service.ErrCartLimit
		}
	}
	item, err := s.cart.Add(ctx, userID, goodsID, quantity, service.MaxCartQuantity)
	if err != nil {
		return nil, err
	}
	return s.line(ctx, item)
}

func (s *sCart) Update(ctx context.Context, userID, itemID uint64, quantity uint32) (*service.CartLine, error) {
	if userID == 0 {
		return nil, service.ErrNoIdentity
	}
	if itemID == 0 || quantity < 1 || quantity > service.MaxCartQuantity {
		return nil, service.ErrInvalidArgument
	}
	ok, err := s.cart.SetQuantity(ctx, userID, itemID, quantity)
	if err != nil {
		return nil, err
	}
	if !ok {
		// Missing and someone else's look the same.
		return nil, service.ErrNotFound
	}
	item, err := s.cart.Find(ctx, userID, itemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, service.ErrNotFound // deleted in between
	}
	return s.line(ctx, item)
}

func (s *sCart) Delete(ctx context.Context, userID uint64, itemIDs []uint64) (int, error) {
	if userID == 0 {
		return 0, service.ErrNoIdentity
	}
	if len(itemIDs) == 0 || len(itemIDs) > service.MaxCartLines {
		return 0, service.ErrInvalidArgument
	}
	return s.cart.Delete(ctx, userID, itemIDs)
}

func (s *sCart) line(ctx context.Context, item *service.CartItem) (*service.CartLine, error) {
	lines, err := s.lines(ctx, []*service.CartItem{item})
	if err != nil {
		return nil, err
	}
	return lines[0], nil
}

// lines joins rows with their goods in three batched reads, whatever the
// page size: goods, covers and stock.
func (s *sCart) lines(ctx context.Context, items []*service.CartItem) ([]*service.CartLine, error) {
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.GoodsID)
	}
	goods, err := s.goods.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	covers, err := s.images.Covers(ctx, ids)
	if err != nil {
		return nil, err
	}
	stocks, err := s.stocks.ByGoods(ctx, ids)
	if err != nil {
		return nil, err
	}
	lines := make([]*service.CartLine, 0, len(items))
	for _, item := range items {
		line := &service.CartLine{CartItem: *item}
		if g, ok := goods[item.GoodsID]; ok {
			line.GoodsName = g.Name
			line.PriceCents = g.PriceCents
			line.CoverURL = covers[item.GoodsID]
			line.Available = g.Status == service.StatusOnShelf
			line.Stock = stocks[item.GoodsID].Available
		}
		lines = append(lines, line)
	}
	return lines, nil
}
