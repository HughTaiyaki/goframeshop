package order

import (
	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"
	refundv1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/order/v1"
)

func toOrderView(o *orderv1.Order) *v1.OrderView {
	if o == nil {
		return nil
	}
	view := &v1.OrderView{
		OrderNo:          o.OrderNo,
		UserId:           o.UserId,
		Status:           o.Status.String(),
		CloseReason:      o.CloseReason,
		TotalCents:       o.TotalCents,
		ConsigneeName:    o.ConsigneeName,
		ConsigneePhone:   o.ConsigneePhone,
		ConsigneeAddress: o.ConsigneeAddress,
		Remark:           o.Remark,
		Items:            make([]*v1.OrderItemView, 0, len(o.Items)),
		CreatedAt:        utility.TimeFromTimestamp(o.CreatedAt),
		ExpiresAt:        utility.TimeFromTimestamp(o.ExpiresAt),
		PaidAt:           utility.TimeFromTimestamp(o.PaidAt),
		ClosedAt:         utility.TimeFromTimestamp(o.ClosedAt),
		Carrier:          o.Carrier,
		TrackingNo:       o.TrackingNo,
		ShippedAt:        utility.TimeFromTimestamp(o.ShippedAt),
		ConfirmDeadline:  utility.TimeFromTimestamp(o.ConfirmDeadline),
		CompletedAt:      utility.TimeFromTimestamp(o.CompletedAt),
		CompleteReason:   o.CompleteReason,
	}
	for _, item := range o.Items {
		view.Items = append(view.Items, &v1.OrderItemView{
			GoodsId:        item.GoodsId,
			GoodsName:      item.GoodsName,
			CoverUrl:       item.CoverUrl,
			UnitPriceCents: item.UnitPriceCents,
			Quantity:       item.Quantity,
			SubtotalCents:  item.SubtotalCents,
		})
	}
	return view
}

func toOrderList(res *orderv1.ListOrdersResponse) *v1.OrderListRes {
	out := &v1.OrderListRes{Items: make([]*v1.OrderView, 0, len(res.Items)), Page: res.Page, Size: res.Size, Total: res.Total}
	for _, o := range res.Items {
		out.Items = append(out.Items, toOrderView(o))
	}
	return out
}

func toRefundView(r *refundv1.Refund) *v1.RefundView {
	if r == nil {
		return nil
	}
	return &v1.RefundView{
		RefundNo:  r.RefundNo,
		OrderNo:   r.OrderNo,
		UserId:    r.UserId,
		GoodsId:   r.GoodsId,
		Reason:    r.Reason,
		Status:    r.Status.String(),
		CreatedAt: utility.TimeFromTimestamp(r.CreatedAt),

		AmountCents: r.AmountCents,
		GoodsName:   r.GoodsName,
		ReviewNote:  r.ReviewNote,
		ReviewedAt:  utility.TimeFromTimestamp(r.ReviewedAt),
		RefundedAt:  utility.TimeFromTimestamp(r.RefundedAt),
	}
}

func toRefundList(res *refundv1.ListRefundsResponse) *v1.RefundListRes {
	out := &v1.RefundListRes{Items: make([]*v1.RefundView, 0, len(res.Items)), Page: res.Page, Size: res.Size, Total: res.Total}
	for _, r := range res.Items {
		out.Items = append(out.Items, toRefundView(r))
	}
	return out
}
