package v1

import "github.com/gogf/gf/v2/os/gtime"

// OrderView 是订单的 HTTP 视图。金额单位都是分；价格、商品名、地址都是下单那一刻的快照。
type OrderView struct {
	OrderNo          string           `json:"orderNo"          dc:"订单号"`
	UserId           uint64           `json:"userId"           dc:"下单用户 ID"`
	Status           string           `json:"status"           dc:"RESERVING 预留中 / PENDING_PAYMENT 待支付 / PAYMENT_CONFIRMING 已付款，确认中 / PAID 已支付 / EXCEPTION 异常，退款中 / CLOSING 关闭中 / CLOSED 已关闭 / SHIPPED 已发货 / COMPLETED 已完成"`
	CloseReason      string           `json:"closeReason"      dc:"关闭原因：USER_CANCELLED、PAYMENT_TIMEOUT、RESERVE_TIMEOUT、INSUFFICIENT_STOCK、GOODS_UNAVAILABLE、RESERVE_CONFLICT、INVENTORY_CONFLICT"`
	TotalCents       uint64           `json:"totalCents"       dc:"订单总额（分），由服务端按商品价格计算"`
	ConsigneeName    string           `json:"consigneeName"    dc:"收货人"`
	ConsigneePhone   string           `json:"consigneePhone"   dc:"收货电话"`
	ConsigneeAddress string           `json:"consigneeAddress" dc:"收货地址"`
	Remark           string           `json:"remark"           dc:"备注"`
	Items            []*OrderItemView `json:"items"            dc:"订单商品"`
	CreatedAt        *gtime.Time      `json:"createdAt"        dc:"下单时间"`
	ExpiresAt        *gtime.Time      `json:"expiresAt"        dc:"支付截止时间，过时自动关单并释放库存"`
	PaidAt           *gtime.Time      `json:"paidAt"           dc:"付款时间"`
	ClosedAt         *gtime.Time      `json:"closedAt"         dc:"关闭时间"`
	Carrier          string           `json:"carrier"          dc:"物流公司（发货后）"`
	TrackingNo       string           `json:"trackingNo"       dc:"运单号（发货后）"`
	ShippedAt        *gtime.Time      `json:"shippedAt"        dc:"发货时间"`
	ConfirmDeadline  *gtime.Time      `json:"confirmDeadline"  dc:"自动确认收货时间：到时还没确认，订单自动完成"`
	CompletedAt      *gtime.Time      `json:"completedAt"      dc:"完成时间"`
	CompleteReason   string           `json:"completeReason"   dc:"USER_CONFIRMED 顾客确认 / AUTO_CONFIRMED 到期自动确认"`
}

type OrderItemView struct {
	GoodsId        uint64 `json:"goodsId"        dc:"商品 ID"`
	GoodsName      string `json:"goodsName"      dc:"商品名（下单时）"`
	CoverUrl       string `json:"coverUrl"       dc:"封面图（下单时）"`
	UnitPriceCents uint64 `json:"unitPriceCents" dc:"单价（分，下单时）"`
	Quantity       uint32 `json:"quantity"       dc:"数量"`
	SubtotalCents  uint64 `json:"subtotalCents"  dc:"小计（分）"`
}

type OrderListRes struct {
	Items []*OrderView `json:"items" dc:"订单列表，新的在前"`
	Page  uint32       `json:"page"  dc:"当前页码"`
	Size  uint32       `json:"size"  dc:"每页条数"`
	Total uint64       `json:"total" dc:"总数"`
}

type RefundView struct {
	RefundNo  string      `json:"refundNo"  dc:"售后单号"`
	OrderNo   string      `json:"orderNo"   dc:"订单号"`
	UserId    uint64      `json:"userId"    dc:"申请用户 ID"`
	GoodsId   uint64      `json:"goodsId"   dc:"售后商品 ID"`
	Reason    string      `json:"reason"    dc:"原因"`
	Status    string      `json:"status"    dc:"PENDING 待处理 / APPROVED 同意 / REJECTED 拒绝"`
	CreatedAt *gtime.Time `json:"createdAt" dc:"申请时间"`
}

type RefundListRes struct {
	Items []*RefundView `json:"items" dc:"售后列表，新的在前"`
	Page  uint32        `json:"page"  dc:"当前页码"`
	Size  uint32        `json:"size"  dc:"每页条数"`
	Total uint64        `json:"total" dc:"总数"`
}
