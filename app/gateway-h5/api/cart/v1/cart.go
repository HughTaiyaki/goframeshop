package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// 购物车接口全部需要登录，只能操作自己的购物车：用户 ID 来自令牌，请求里没有。
// 价格是商品的当前价格；真正的金额以下单时服务端计算的为准。

type CartListReq struct {
	g.Meta `path:"/cart" method:"get" tags:"购物车" summary:"我的购物车（最新加入的在前）"`
	Page   uint32 `json:"page" v:"min:1"   d:"1"  dc:"页码，从 1 开始"`
	Size   uint32 `json:"size" v:"max:100" d:"20" dc:"每页条数"`
}

type CartListRes struct {
	Items []*CartItem `json:"items" dc:"购物车项"`
	Page  uint32      `json:"page"  dc:"当前页码"`
	Size  uint32      `json:"size"  dc:"每页条数"`
	Total uint64      `json:"total" dc:"总行数"`
}

type CartAddReq struct {
	g.Meta   `path:"/cart" method:"post" tags:"购物车" summary:"加入购物车（已有时数量累加，只能加购上架商品）"`
	GoodsId  uint64 `json:"goodsId"  v:"required|min:1"       dc:"商品 ID"`
	Quantity uint32 `json:"quantity" v:"required|between:1,99" dc:"加购数量；累加后不能超过 99"`
}

type CartAddRes struct {
	*CartItem
}

type CartUpdateReq struct {
	g.Meta     `path:"/cart" method:"put" tags:"购物车" summary:"修改数量"`
	CartItemId uint64 `json:"cartItemId" v:"required|min:1"        dc:"购物车项 ID"`
	Quantity   uint32 `json:"quantity"   v:"required|between:1,99" dc:"新的数量"`
}

type CartUpdateRes struct {
	*CartItem
}

type CartDeleteReq struct {
	g.Meta      `path:"/cart" method:"delete" tags:"购物车" summary:"删除购物车项（可以一次删多个）"`
	CartItemIds []uint64 `json:"cartItemIds" v:"required" dc:"购物车项 ID 列表，最多 100 个；不是自己的会被忽略"`
}

type CartDeleteRes struct {
	Deleted uint32 `json:"deleted" dc:"实际删除的行数"`
}

// CartItem 是购物车的一行，带着商品现在的信息。商品下架或删除后仍然显示，available 为 false。
type CartItem struct {
	CartItemId uint64      `json:"cartItemId" dc:"购物车项 ID"`
	GoodsId    uint64      `json:"goodsId"    dc:"商品 ID"`
	Quantity   uint32      `json:"quantity"   dc:"数量"`
	GoodsName  string      `json:"goodsName"  dc:"商品名称（商品已删除时为空）"`
	PriceCents uint64      `json:"priceCents" dc:"当前单价（分）"`
	CoverUrl   string      `json:"coverUrl"   dc:"封面图"`
	Available  bool        `json:"available"  dc:"现在能否购买（上架且未删除）"`
	Stock      uint32      `json:"stock"      dc:"当前可用库存，下单数量超过它会失败"`
	CreatedAt  *gtime.Time `json:"createdAt"  dc:"加入时间"`
	UpdatedAt  *gtime.Time `json:"updatedAt"  dc:"最后修改时间"`
}
