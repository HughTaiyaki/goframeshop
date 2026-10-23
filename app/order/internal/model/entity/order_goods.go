// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// OrderGoods is the golang structure for table order_goods.
type OrderGoods struct {
	Id             uint64      `json:"id"             orm:"id"               description:""` //
	OrderId        uint64      `json:"orderId"        orm:"order_id"         description:""` //
	GoodsId        uint64      `json:"goodsId"        orm:"goods_id"         description:""` //
	GoodsName      string      `json:"goodsName"      orm:"goods_name"       description:""` //
	CoverUrl       string      `json:"coverUrl"       orm:"cover_url"        description:""` //
	UnitPriceCents uint64      `json:"unitPriceCents" orm:"unit_price_cents" description:""` //
	Quantity       uint        `json:"quantity"       orm:"quantity"         description:""` //
	SubtotalCents  uint64      `json:"subtotalCents"  orm:"subtotal_cents"   description:""` //
	CreatedAt      *gtime.Time `json:"createdAt"      orm:"created_at"       description:""` //
}
