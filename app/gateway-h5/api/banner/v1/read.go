package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// 列表在 gateway-h5 上公开，在 gateway-admin 上需要管理员令牌。

type RotationListReq struct {
	g.Meta `path:"/rotation" method:"get" tags:"首页运营" summary:"轮播图列表（按 sort 升序）"`
	Page   uint32 `json:"page" v:"min:1"  d:"1"  dc:"页码"`
	Size   uint32 `json:"size" v:"max:50" d:"10" dc:"每页条数"`
}

type RotationListRes struct {
	Items []*RotationItem `json:"items" dc:"轮播图"`
	Page  uint32          `json:"page"  dc:"当前页码"`
	Size  uint32          `json:"size"  dc:"每页条数"`
	Total uint64          `json:"total" dc:"总数"`
}

type RotationItem struct {
	RotationId uint64      `json:"rotationId" dc:"轮播图 ID"`
	FileId     uint64      `json:"fileId"     dc:"图片文件 ID"`
	PicUrl     string      `json:"picUrl"     dc:"图片地址"`
	Link       string      `json:"link"       dc:"跳转链接（http/https），为空表示不跳转"`
	Sort       uint32      `json:"sort"       dc:"排序，小的在前"`
	CreatedAt  *gtime.Time `json:"createdAt"  dc:"创建时间"`
	UpdatedAt  *gtime.Time `json:"updatedAt"  dc:"更新时间"`
}

type PositionListReq struct {
	g.Meta `path:"/position" method:"get" tags:"首页运营" summary:"手工位列表（按 sort 升序）"`
	Page   uint32 `json:"page" v:"min:1"  d:"1"  dc:"页码"`
	Size   uint32 `json:"size" v:"max:50" d:"10" dc:"每页条数"`
}

type PositionListRes struct {
	Items []*PositionItem `json:"items" dc:"手工位"`
	Page  uint32          `json:"page"  dc:"当前页码"`
	Size  uint32          `json:"size"  dc:"每页条数"`
	Total uint64          `json:"total" dc:"总数"`
}

type PositionItem struct {
	PositionId uint64      `json:"positionId" dc:"手工位 ID"`
	FileId     uint64      `json:"fileId"     dc:"图片文件 ID"`
	PicUrl     string      `json:"picUrl"     dc:"图片地址"`
	GoodsId    uint64      `json:"goodsId"    dc:"商品 ID"`
	GoodsName  string      `json:"goodsName"  dc:"商品名（保存时从商品服务读取，之后随商品改名自动更新）"`
	Link       string      `json:"link"       dc:"跳转链接；为空表示打开商品详情"`
	Sort       uint32      `json:"sort"       dc:"排序，小的在前"`
	CreatedAt  *gtime.Time `json:"createdAt"  dc:"创建时间"`
	UpdatedAt  *gtime.Time `json:"updatedAt"  dc:"更新时间"`
}
