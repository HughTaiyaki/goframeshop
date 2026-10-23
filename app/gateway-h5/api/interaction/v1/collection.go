package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Collection 接口只操作当前登录用户自己的收藏，用户 ID 来自令牌，不接受请求参数。

type CollectionCreateReq struct {
	g.Meta     `path:"/collection" method:"post" tags:"收藏" summary:"收藏"`
	ObjectType uint32 `json:"objectType" v:"required|in:1,2" dc:"对象类型：1 商品 2 文章"`
	ObjectId   uint64 `json:"objectId"   v:"required|min:1"  dc:"对象 ID"`
}

type CollectionCreateRes struct {
	CollectionId uint64 `json:"collectionId" dc:"新建的收藏 ID"`
}

type CollectionDeleteReq struct {
	g.Meta       `path:"/collection" method:"delete" tags:"收藏" summary:"取消收藏"`
	CollectionId uint64 `json:"collectionId" v:"required|min:1" dc:"收藏 ID"`
}

type CollectionDeleteRes struct{}

type CollectionListReq struct {
	g.Meta     `path:"/collection" method:"get" tags:"收藏" summary:"我的收藏列表"`
	ObjectType uint32 `json:"objectType" v:"required|in:1,2" dc:"对象类型：1 商品 2 文章"`
	Page       uint32 `json:"page"       v:"min:1"   d:"1"  dc:"页码，从 1 开始"`
	Size       uint32 `json:"size"       v:"max:100" d:"10" dc:"每页条数"`
}

type CollectionListRes struct {
	Items []*CollectionItem `json:"items" dc:"收藏列表"`
	Page  uint32            `json:"page"  dc:"当前页码"`
	Size  uint32            `json:"size"  dc:"每页条数"`
	Total uint64            `json:"total" dc:"总数"`
}

type CollectionItem struct {
	CollectionId uint64      `json:"collectionId"     dc:"收藏 ID"`
	ObjectType   uint32      `json:"objectType" dc:"对象类型：1 商品 2 文章"`
	ObjectId     uint64      `json:"objectId"   dc:"对象 ID"`
	CreatedAt    *gtime.Time `json:"createdAt"  dc:"创建时间"`
}
