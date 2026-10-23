package v1

import "github.com/gogf/gf/v2/frame/g"

// 前台订单接口，全部需要登录，只能操作自己的订单。用户 ID 来自令牌，不接受请求参数。
// 金额、商品名和收货地址都由服务端查询，请求里不能指定。

type OrderItemInput struct {
	GoodsId  uint64 `json:"goodsId"  v:"required|min:1"     dc:"商品 ID"`
	Quantity uint32 `json:"quantity" v:"required|between:1,99" dc:"数量"`
}

type OrderCreateReq struct {
	g.Meta      `path:"/order" method:"post" tags:"订单" summary:"下单（预留库存，15 分钟内未支付自动关单）"`
	RequestId   string            `json:"requestId"   v:"required|length:1,64" dc:"客户端生成的请求号，同一次提交重试时保持不变，避免重复下单"`
	Items       []*OrderItemInput `json:"items"       v:"required"             dc:"商品与数量，最多 20 种"`
	ConsigneeId uint64            `json:"consigneeId" v:"required|min:1"       dc:"自己的收货地址 ID"`
	Remark      string            `json:"remark"      v:"length:0,255"         dc:"备注"`
}

type OrderCreateRes struct {
	*OrderView
}

type OrderCancelReq struct {
	g.Meta  `path:"/order/cancel" method:"post" tags:"订单" summary:"取消订单（释放库存）"`
	OrderNo string `json:"orderNo" v:"required|length:1,32" dc:"订单号"`
}

type OrderCancelRes struct {
	*OrderView
}

type OrderListReq struct {
	g.Meta `path:"/order" method:"get" tags:"订单" summary:"我的订单"`
	Status string `json:"status" v:"in:,RESERVING,PENDING_PAYMENT,PAYMENT_CONFIRMING,PAID,EXCEPTION,CLOSING,CLOSED" dc:"按状态筛选，不填表示全部"`
	Page   uint32 `json:"page"   v:"min:1"  d:"1"  dc:"页码"`
	Size   uint32 `json:"size"   v:"max:50" d:"10" dc:"每页条数"`
}

type OrderPayReq struct {
	g.Meta  `path:"/order/pay" method:"post" tags:"订单" summary:"去支付：返回支付模拟器的收银台地址；已经付过款会直接确认"`
	OrderNo string `json:"orderNo" v:"required|length:1,32" dc:"订单号，只能是自己的待支付订单"`
}

type OrderPayRes struct {
	*PaymentView
}

// PaymentNotifyReq 是支付模拟器的回调，不需要登录：请求体的签名在订单服务里校验，
// 而且订单服务只把它当成"去查一下"的提示，真正的结果来自查询。
type PaymentNotifyReq struct {
	g.Meta `path:"/payment/notify" method:"post" tags:"支付" summary:"支付结果通知（支付模拟器调用，带签名）"`
}

type PaymentNotifyRes struct{}

type OrderDetailReq struct {
	g.Meta  `path:"/order/detail" method:"get" tags:"订单" summary:"我的订单详情"`
	OrderNo string `json:"orderNo" v:"required|length:1,32" dc:"订单号"`
}

type OrderDetailRes struct {
	*OrderView
}

type RefundCreateReq struct {
	g.Meta  `path:"/refund" method:"post" tags:"售后" summary:"申请售后（仅限已支付订单）"`
	OrderNo string `json:"orderNo" v:"required|length:1,32"  dc:"订单号"`
	GoodsId uint64 `json:"goodsId" v:"required|min:1"        dc:"订单里的商品 ID"`
	Reason  string `json:"reason"  v:"required|length:1,255" dc:"原因"`
}

type RefundCreateRes struct {
	*RefundView
}

type RefundListReq struct {
	g.Meta `path:"/refund" method:"get" tags:"售后" summary:"我的售后申请"`
	Page   uint32 `json:"page" v:"min:1"  d:"1"  dc:"页码"`
	Size   uint32 `json:"size" v:"max:50" d:"10" dc:"每页条数"`
}

type RefundDetailReq struct {
	g.Meta   `path:"/refund/detail" method:"get" tags:"售后" summary:"我的售后申请详情"`
	RefundNo string `json:"refundNo" v:"required|length:1,32" dc:"售后单号"`
}

type RefundDetailRes struct {
	*RefundView
}
