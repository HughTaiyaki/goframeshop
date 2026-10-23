package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// 读接口：后台能看到下架商品和被预留的库存。

type GoodsListReq struct {
	g.Meta     `path:"/goods" method:"get" tags:"商品" summary:"商品列表"`
	CategoryId uint64 `json:"categoryId"                     dc:"按分类筛选（任意层级），0 表示全部"`
	Shelf      uint32 `json:"shelf"      v:"in:0,1,2"       dc:"0 全部，1 只看上架，2 只看下架"`
	IsHot      uint32 `json:"isHot"      v:"in:0,1"  d:"0"  dc:"1 只看热门（sort > 0，按 sort 从高到低），0 全部"`
	Page       uint32 `json:"page"       v:"min:1"   d:"1"  dc:"页码，从 1 开始"`
	Size       uint32 `json:"size"       v:"max:100" d:"10" dc:"每页条数"`
}

type GoodsDetailReq struct {
	g.Meta  `path:"/goods/detail" method:"get" tags:"商品" summary:"商品详情（走 Redis 缓存）"`
	GoodsId uint64 `json:"goodsId" v:"required|min:1" dc:"商品 ID"`
}

type CategoryListReq struct {
	g.Meta   `path:"/category" method:"get" tags:"商品分类" summary:"某个分类的子分类"`
	ParentId uint64 `json:"parentId"                     dc:"父分类 ID，0 表示一级分类"`
	Page     uint32 `json:"page"     v:"min:1"   d:"1"  dc:"页码，从 1 开始"`
	Size     uint32 `json:"size"     v:"max:100" d:"10" dc:"每页条数"`
}

type CategoryAllReq struct {
	g.Meta `path:"/category/all" method:"get" tags:"商品分类" summary:"全部分类（走 Redis 缓存）"`
}

// GoodsItem 是列表里的一行。价格单位是分（priceCents），不使用浮点数。
type GoodsItem struct {
	GoodsId    uint64 `json:"goodsId"    dc:"商品 ID"`
	Name       string `json:"name"       dc:"商品名称"`
	PriceCents uint64 `json:"priceCents" dc:"价格（分）"`
	Brand      string `json:"brand"      dc:"品牌"`
	Stock      uint32 `json:"stock"      dc:"可用库存"`
	Sale       uint32 `json:"sale"       dc:"销量（已提交的订单数量）"`
	Tags       string `json:"tags"       dc:"标签"`
	CoverUrl   string `json:"coverUrl"   dc:"封面图（排序最靠前的图片），没有图片时为空"`
	OnShelf    bool   `json:"onShelf"    dc:"是否上架"`
	Sort       uint32 `json:"sort"       dc:"推荐权重：0 不是热门，越大越靠前"`
}

type GoodsListRes struct {
	Items []*GoodsItem `json:"items" dc:"商品列表，新的在前；isHot=1 时按 sort 从高到低"`
	Page  uint32       `json:"page"  dc:"当前页码"`
	Size  uint32       `json:"size"  dc:"每页条数"`
	Total uint64       `json:"total" dc:"总数"`
}

type GoodsImageItem struct {
	ImageId uint64 `json:"imageId" dc:"商品图片 ID"`
	FileId  uint64 `json:"fileId"  dc:"文件 ID"`
	Url     string `json:"url"     dc:"图片地址"`
	Sort    uint32 `json:"sort"    dc:"排序，小的在前"`
}

type GoodsDetailRes struct {
	GoodsId          uint64            `json:"goodsId"          dc:"商品 ID"`
	Name             string            `json:"name"             dc:"商品名称"`
	PriceCents       uint64            `json:"priceCents"       dc:"价格（分）"`
	Level1CategoryId uint64            `json:"level1CategoryId" dc:"一级分类 ID"`
	Level2CategoryId uint64            `json:"level2CategoryId" dc:"二级分类 ID，0 表示无"`
	Level3CategoryId uint64            `json:"level3CategoryId" dc:"三级分类 ID，0 表示无"`
	Brand            string            `json:"brand"            dc:"品牌"`
	OnShelf          bool              `json:"onShelf"          dc:"是否上架"`
	Sort             uint32            `json:"sort"             dc:"推荐权重：0 不是热门，越大越靠前"`
	Stock            uint32            `json:"stock"            dc:"可用库存"`
	Reserved         uint32            `json:"reserved"         dc:"已被未支付订单预留的数量"`
	Sale             uint32            `json:"sale"             dc:"销量（已提交的订单数量）"`
	Tags             string            `json:"tags"             dc:"标签"`
	DetailInfo       string            `json:"detailInfo"       dc:"商品详情"`
	Images           []*GoodsImageItem `json:"images"           dc:"商品图片，按 sort 排序"`
	CreatedAt        *gtime.Time       `json:"createdAt"        dc:"创建时间"`
	UpdatedAt        *gtime.Time       `json:"updatedAt"        dc:"更新时间"`
}

type CategoryItem struct {
	CategoryId uint64      `json:"categoryId" dc:"分类 ID"`
	ParentId   uint64      `json:"parentId"   dc:"父分类 ID，0 表示一级分类"`
	Level      uint32      `json:"level"      dc:"层级 1-3，由父分类决定"`
	Name       string      `json:"name"       dc:"名称"`
	IconFileId uint64      `json:"iconFileId" dc:"图标文件 ID，0 表示无"`
	IconUrl    string      `json:"iconUrl"    dc:"图标地址"`
	Sort       uint32      `json:"sort"       dc:"排序，小的在前"`
	CreatedAt  *gtime.Time `json:"createdAt"  dc:"创建时间"`
	UpdatedAt  *gtime.Time `json:"updatedAt"  dc:"更新时间"`
}

type CategoryListRes struct {
	Items []*CategoryItem `json:"items" dc:"分类列表"`
	Page  uint32          `json:"page"  dc:"当前页码"`
	Size  uint32          `json:"size"  dc:"每页条数"`
	Total uint64          `json:"total" dc:"总数"`
}

// CategoryAllRes 是整棵分类树的扁平列表，按层级、sort、ID 排序；客户端用 parentId 拼树。
type CategoryAllRes struct {
	Items []*CategoryItem `json:"items" dc:"全部分类"`
}
