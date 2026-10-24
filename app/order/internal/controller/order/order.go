// Package order is the gRPC transport layer for orders.
package order

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
)

type Controller struct {
	v1.UnimplementedOrderServiceServer
	orders service.IOrder
}

func Register(s *grpcx.GrpcServer, orders service.IOrder) {
	v1.RegisterOrderServiceServer(s.Server, &Controller{orders: orders})
}

func (c *Controller) CreateOrder(ctx context.Context, req *v1.CreateOrderRequest) (*v1.OrderResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	in := &service.CreateInput{RequestID: req.RequestId, ConsigneeID: req.ConsigneeId, Remark: req.Remark, FromCart: req.FromCart}
	for _, item := range req.Items {
		in.Items = append(in.Items, service.ItemInput{GoodsID: item.GetGoodsId(), Quantity: item.GetQuantity()})
	}
	order, err := c.orders.Create(ctx, userID, in)
	return respond(ctx, order, err)
}

func (c *Controller) CancelOrder(ctx context.Context, req *v1.CancelOrderRequest) (*v1.OrderResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	order, err := c.orders.Cancel(ctx, userID, req.OrderNo)
	return respond(ctx, order, err)
}

func (c *Controller) GetMyOrder(ctx context.Context, req *v1.GetMyOrderRequest) (*v1.OrderResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	order, err := c.orders.GetMine(ctx, userID, req.OrderNo)
	return respond(ctx, order, err)
}

func (c *Controller) ListMyOrders(ctx context.Context, req *v1.ListMyOrdersRequest) (*v1.ListOrdersResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	orders, total, err := c.orders.ListMine(ctx, userID, statusName(req.Status), int(req.Page), int(req.Size))
	return respondList(ctx, orders, total, req.Page, req.Size, err)
}

func (c *Controller) GetOrder(ctx context.Context, req *v1.GetOrderRequest) (*v1.OrderResponse, error) {
	order, err := c.orders.Get(ctx, req.OrderNo)
	return respond(ctx, order, err)
}

func (c *Controller) ListOrders(ctx context.Context, req *v1.ListOrdersRequest) (*v1.ListOrdersResponse, error) {
	f := &service.ListFilter{
		UserID: req.UserId, OrderNo: req.OrderNo, Status: statusName(req.Status),
		Page: int(req.Page), Size: int(req.Size),
	}
	if req.CreatedFrom != nil {
		t := req.CreatedFrom.AsTime()
		f.CreatedFrom = &t
	}
	if req.CreatedTo != nil {
		t := req.CreatedTo.AsTime()
		f.CreatedTo = &t
	}
	orders, total, err := c.orders.List(ctx, f)
	return respondList(ctx, orders, total, req.Page, req.Size, err)
}

func statusName(s v1.OrderStatus) string {
	if s == v1.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		return ""
	}
	return s.String()
}

func respond(ctx context.Context, order *service.Order, err error) (*v1.OrderResponse, error) {
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.OrderResponse{Order: ToProto(order)}, nil
}

func respondList(ctx context.Context, orders []*service.Order, total int, page, size uint32, err error) (*v1.ListOrdersResponse, error) {
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	res := &v1.ListOrdersResponse{Items: make([]*v1.Order, 0, len(orders)), Page: page, Size: size, Total: uint64(total)}
	for _, o := range orders {
		res.Items = append(res.Items, ToProto(o))
	}
	return res, nil
}

// ToProto is shared with the payment controller.
func ToProto(o *service.Order) *v1.Order {
	out := &v1.Order{
		OrderNo:          o.No,
		UserId:           o.UserID,
		Status:           v1.OrderStatus(v1.OrderStatus_value[o.Status]),
		CloseReason:      o.CloseReason,
		TotalCents:       o.TotalCents,
		ConsigneeName:    o.ConsigneeName,
		ConsigneePhone:   o.ConsigneePhone,
		ConsigneeAddress: o.ConsigneeAddress,
		Remark:           o.Remark,
		Items:            make([]*v1.OrderItem, 0, len(o.Items)),
		CreatedAt:        controller.Timestamp(&o.CreatedAt),
		ExpiresAt:        controller.Timestamp(o.ExpiresAt),
		ClosedAt:         controller.Timestamp(o.ClosedAt),
		PaidAt:           controller.Timestamp(o.PaidAt),
		Carrier:          o.Carrier,
		TrackingNo:       o.TrackingNo,
		ShippedAt:        controller.Timestamp(o.ShippedAt),
		ConfirmDeadline:  controller.Timestamp(o.ConfirmDeadline),
		CompletedAt:      controller.Timestamp(o.CompletedAt),
		CompleteReason:   o.CompleteReason,
	}
	for _, item := range o.Items {
		out.Items = append(out.Items, &v1.OrderItem{
			GoodsId:        item.GoodsID,
			GoodsName:      item.GoodsName,
			CoverUrl:       item.CoverURL,
			UnitPriceCents: item.UnitPriceCents,
			Quantity:       item.Quantity,
			SubtotalCents:  item.SubtotalCents,
		})
	}
	return out
}

func (c *Controller) ShipOrder(ctx context.Context, req *v1.ShipOrderRequest) (*v1.OrderResponse, error) {
	order, err := c.orders.Ship(ctx, req.OrderNo, service.Shipment{Carrier: req.Carrier, TrackingNo: req.TrackingNo})
	return respond(ctx, order, err)
}

func (c *Controller) ConfirmReceipt(ctx context.Context, req *v1.ConfirmReceiptRequest) (*v1.OrderResponse, error) {
	userID, err := controller.CallerID(ctx)
	if err != nil {
		return nil, err
	}
	order, err := c.orders.ConfirmReceipt(ctx, userID, req.OrderNo)
	return respond(ctx, order, err)
}
