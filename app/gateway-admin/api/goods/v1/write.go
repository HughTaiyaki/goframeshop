package v1

import "github.com/gogf/gf/v2/frame/g"

// 写接口只在后台网关上存在，需要管理员令牌。

// GoodsFields 是可编辑字段。库存和销量都不在其中：库存用 /goods/stock 调整，销量由订单产生。
type GoodsFields struct {
	Name             string `json:"name"             v:"required|length:1,200" dc:"商品名称"`
	PriceCents       uint64 `json:"priceCents"       v:"required|min:1"        dc:"价格（分），例如 19.90 元填 1990"`
	Level1CategoryId uint64 `json:"level1CategoryId" v:"required|min:1"        dc:"一级分类 ID"`
	Level2CategoryId uint64 `json:"level2CategoryId"                           dc:"二级分类 ID，必须属于一级分类；0 表示无"`
	Level3CategoryId uint64 `json:"level3CategoryId"                           dc:"三级分类 ID，必须属于二级分类；0 表示无"`
	Brand            string `json:"brand"            v:"length:0,30"           dc:"品牌"`
	Tags             string `json:"tags"             v:"length:0,255"          dc:"标签"`
	DetailInfo       string `json:"detailInfo"       v:"length:0,16000"        dc:"商品详情"`
	Sort             uint32 `json:"sort"             v:"between:0,9999"        dc:"推荐权重：0（默认）不是热门；大于 0 进入前台热门推荐，越大越靠前。修改商品时不填等于 0"`
}

type GoodsCreateReq struct {
	g.Meta `path:"/goods" method:"post" tags:"商品" summary:"创建商品（创建后是下架状态）"`
	GoodsFields
	InitialStock uint32 `json:"initialStock" v:"between:0,1000000" dc:"初始可用库存"`
}

type GoodsCreateRes struct {
	GoodsId uint64 `json:"goodsId" dc:"新商品 ID"`
}

type GoodsUpdateReq struct {
	g.Meta  `path:"/goods" method:"put" tags:"商品" summary:"修改商品（整体替换可编辑字段）"`
	GoodsId uint64 `json:"goodsId" v:"required|min:1" dc:"商品 ID"`
	GoodsFields
}

type GoodsUpdateRes struct{}

type GoodsDeleteReq struct {
	g.Meta  `path:"/goods" method:"delete" tags:"商品" summary:"删除商品"`
	GoodsId uint64 `json:"goodsId" v:"required|min:1" dc:"商品 ID"`
}

type GoodsDeleteRes struct{}

type GoodsImageAddReq struct {
	g.Meta  `path:"/goods/image" method:"post" tags:"商品" summary:"给商品添加图片（先用 /backend/upload/image 上传拿到 fileId）"`
	GoodsId uint64 `json:"goodsId" v:"required|min:1"     dc:"商品 ID"`
	FileId  uint64 `json:"fileId"  v:"required|min:1"     dc:"文件 ID"`
	Sort    uint32 `json:"sort"    v:"between:0,999"      dc:"排序，小的在前"`
}

type GoodsImageAddRes struct {
	ImageId uint64 `json:"imageId" dc:"商品图片 ID"`
}

type GoodsImageRemoveReq struct {
	g.Meta  `path:"/goods/image" method:"delete" tags:"商品" summary:"移除商品图片"`
	ImageId uint64 `json:"imageId" v:"required|min:1" dc:"商品图片 ID"`
}

type GoodsImageRemoveRes struct{}

type CategoryCreateReq struct {
	g.Meta     `path:"/category" method:"post" tags:"商品分类" summary:"创建分类（层级由父分类决定，最多三级）"`
	ParentId   uint64 `json:"parentId"                             dc:"父分类 ID，0 表示一级分类"`
	Name       string `json:"name"       v:"required|length:1,30"  dc:"名称"`
	IconFileId uint64 `json:"iconFileId"                           dc:"图标文件 ID，0 表示无"`
	Sort       uint32 `json:"sort"       v:"between:0,999"         dc:"排序，小的在前"`
}

type CategoryCreateRes struct {
	CategoryId uint64 `json:"categoryId" dc:"新分类 ID"`
}

// CategoryUpdateReq 不能修改父分类：移动分类会让已有商品的分类链失效。
type CategoryUpdateReq struct {
	g.Meta     `path:"/category" method:"put" tags:"商品分类" summary:"修改分类名称、图标和排序"`
	CategoryId uint64 `json:"categoryId" v:"required|min:1"        dc:"分类 ID"`
	Name       string `json:"name"       v:"required|length:1,30"  dc:"名称"`
	IconFileId uint64 `json:"iconFileId"                           dc:"图标文件 ID，0 表示无"`
	Sort       uint32 `json:"sort"       v:"between:0,999"         dc:"排序，小的在前"`
}

type CategoryUpdateRes struct{}

type CategoryDeleteReq struct {
	g.Meta     `path:"/category" method:"delete" tags:"商品分类" summary:"删除分类（还有子分类或商品时会被拒绝）"`
	CategoryId uint64 `json:"categoryId" v:"required|min:1" dc:"分类 ID"`
}

type CategoryDeleteRes struct{}

type GoodsShelfReq struct {
	g.Meta  `path:"/goods/shelf" method:"put" tags:"商品" summary:"上架或下架"`
	GoodsId uint64 `json:"goodsId" v:"required|min:1" dc:"商品 ID"`
	OnShelf bool   `json:"onShelf"                    dc:"true 上架，false 下架；下架后前台看不到，也不能再被预留"`
}

type GoodsShelfRes struct{}

// GoodsStockReq 只改可用库存，不影响已预留和已售出的数量；会让可用库存变成负数时拒绝。
type GoodsStockReq struct {
	g.Meta  `path:"/goods/stock" method:"post" tags:"商品" summary:"增减可用库存"`
	GoodsId uint64 `json:"goodsId" v:"required|min:1"                      dc:"商品 ID"`
	Delta   int32  `json:"delta"   v:"required|between:-1000000,1000000" dc:"正数入库，负数出库，不能为 0"`
}

type GoodsStockRes struct {
	Available uint32 `json:"available" dc:"可用库存"`
	Reserved  uint32 `json:"reserved"  dc:"已预留"`
	Sold      uint32 `json:"sold"      dc:"已售出"`
}
