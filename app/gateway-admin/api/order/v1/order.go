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
	Status      string      `json:"status"      v:"in:,RESERVING,PENDING_PAYMENT,PAYMENT_CONFIRMING,PAID,EXCEPTION,CLOSING,CLOSED" dc:"状态"`
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
	Status string `json:"status" v:"in:,PENDING,APPROVED,REJECTED" dc:"状态"`
	Page   uint32 `json:"page"   v:"min:1"  d:"1"  dc:"页码"`
	Size   uint32 `json:"size"   v:"max:50" d:"10" dc:"每页条数"`
}
