package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// 后台订单接口：查看所有人的订单和售后申请，需要管理员令牌。

type OrderListReq struct {
	g.Meta      `path:"/order" method:"get" tags:"订单" summary:"订单列表"`
	OrderNo     string      `json:"orderNo"     v:"length:0,32" dc:"订单号"`
	UserId      uint64      `json:"userId"                      dc:"用户 ID，0 表示全部"`
	Status      string      `json:"status"      v:"in:,RESERVING,PENDING_PAYMENT,PAYMENT_CONFIRMING,PAID,EXCEPTION,CLOSING,CLOSED,SHIPPED,COMPLETED" dc:"状态"`
	CreatedFrom *gtime.Time `json:"createdFrom"                 dc:"下单时间起"`
	CreatedTo   *gtime.Time `json:"createdTo"                   dc:"下单时间止"`
	Page        uint32      `json:"page"        v:"min:1"  d:"1"  dc:"页码"`
	Size        uint32      `json:"size"        v:"max:50" d:"10" dc:"每页条数"`
}

type OrderDetailReq struct {
	g.Meta  `path:"/order/detail" method:"get" tags:"订单" summary:"订单详情"`
	OrderNo string `json:"orderNo" v:"required|length:1,32" dc:"订单号"`
}

type OrderDetailRes struct {
	*OrderView
}

type RefundListReq struct {
	g.Meta `path:"/refund" method:"get" tags:"售后" summary:"售后申请列表"`
	Status string `json:"status" v:"in:,PENDING,APPROVED,REJECTED,REFUNDED" dc:"状态"`
	Page   uint32 `json:"page"   v:"min:1"  d:"1"  dc:"页码"`
	Size   uint32 `json:"size"   v:"max:50" d:"10" dc:"每页条数"`
}

type OrderShipReq struct {
	g.Meta     `path:"/order/ship" method:"post" tags:"订单" summary:"发货（只能发已支付的订单；同一运单号重复提交直接返回）"`
	OrderNo    string `json:"orderNo"    v:"required|length:1,32"               dc:"订单号"`
	Carrier    string `json:"carrier"    v:"required|length:1,32"               dc:"物流公司，例如 顺丰"`
	TrackingNo string `json:"trackingNo" v:"required|regex:^[A-Za-z0-9-]{1,64}$" dc:"运单号：字母、数字和 -，最长 64 位"`
}

type OrderShipRes struct {
	*OrderView
}

type RefundApproveReq struct {
	g.Meta   `path:"/refund/approve" method:"post" tags:"售后" summary:"同意售后并退款（退这件商品的小计；已同意或已退款时直接返回）"`
	RefundNo string `json:"refundNo" v:"required|length:1,32" dc:"售后单号"`
	Note     string `json:"note"     v:"length:0,255"         dc:"审批意见，可以不填"`
}

type RefundApproveRes struct {
	*RefundView
}

type RefundRejectReq struct {
	g.Meta   `path:"/refund/reject" method:"post" tags:"售后" summary:"拒绝售后（必须写原因，顾客能看到）"`
	RefundNo string `json:"refundNo" v:"required|length:1,32"  dc:"售后单号"`
	Note     string `json:"note"     v:"required|length:1,255" dc:"拒绝原因"`
}

type RefundRejectRes struct {
	*RefundView
}
