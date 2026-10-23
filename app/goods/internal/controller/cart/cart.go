// Package cart is the gRPC transport layer for the shopper's cart.
//
// The auth policy admits shop user tokens only (cmd.cartMethods), and the
// caller is always taken from that token: no request carries a user id.
package cart

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Controller struct {
	v1.UnimplementedCartServiceServer
	cart service.ICart
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterCartServiceServer(s.Server, &Controller{cart: logic.Cart()})
}

func (c *Controller) ListCartItems(ctx context.Context, req *v1.ListCartItemsRequest) (*v1.ListCartItemsResponse, error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	lines, total, err := c.cart.List(ctx, userID, int(req.Page), int(req.Size))
	if err != nil {
		return nil, controller.StatusError(err)
	}
	res := &v1.ListCartItemsResponse{
		Items: make([]*v1.CartItem, 0, len(lines)),
		Page:  req.Page,
		Size:  req.Size,
		Total: uint64(total),
	}
	for _, line := range lines {
		res.Items = append(res.Items, toProto(line))
	}
	return res, nil
}

func (c *Controller) AddCartItem(ctx context.Context, req *v1.AddCartItemRequest) (*v1.CartItemResponse, error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	line, err := c.cart.Add(ctx, userID, req.GoodsId, req.Quantity)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.CartItemResponse{Item: toProto(line)}, nil
}

func (c *Controller) UpdateCartItem(ctx context.Context, req *v1.UpdateCartItemRequest) (*v1.CartItemResponse, error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	line, err := c.cart.Update(ctx, userID, req.CartItemId, req.Quantity)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.CartItemResponse{Item: toProto(line)}, nil
}

func (c *Controller) DeleteCartItems(ctx context.Context, req *v1.DeleteCartItemsRequest) (*v1.DeleteCartItemsResponse, error) {
	userID, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	n, err := c.cart.Delete(ctx, userID, req.CartItemIds)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.DeleteCartItemsResponse{Deleted: uint32(n)}, nil
}

// caller is the shopper the verified token names. A handler reached without
// one is a wiring bug, and it fails closed.
func caller(ctx context.Context) (uint64, error) {
	userID, ok := utility.UserIDFromContext(ctx)
	if !ok || userID == 0 {
		return 0, controller.StatusError(service.ErrNoIdentity)
	}
	return userID, nil
}

func toProto(line *service.CartLine) *v1.CartItem {
	return &v1.CartItem{
		CartItemId: line.ID,
		GoodsId:    line.GoodsID,
		Quantity:   line.Quantity,
		GoodsName:  line.GoodsName,
		PriceCents: line.PriceCents,
		CoverUrl:   line.CoverURL,
		Available:  line.Available,
		Stock:      line.Stock,
		CreatedAt:  controller.Timestamp(line.CreatedAt),
		UpdatedAt:  controller.Timestamp(line.UpdatedAt),
	}
}
