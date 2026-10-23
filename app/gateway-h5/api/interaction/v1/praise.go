package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Praise 接口只操作当前登录用户自己的点赞，用户 ID 来自令牌，不接受请求参数。

type PraiseCreateReq struct {
	g.Meta     `path:"/praise" method:"post" tags:"点赞" summary:"点赞"`
	ObjectType uint32 `json:"objectType" v:"required|in:1,2" dc:"对象类型：1 商品 2 文章"`
	ObjectId   uint64 `json:"objectId"   v:"required|min:1"  dc:"对象 ID"`
}

type PraiseCreateRes struct {
	PraiseId uint64 `json:"praiseId" dc:"新建的点赞 ID"`
}

type PraiseDeleteReq struct {
	g.Meta   `path:"/praise" method:"delete" tags:"点赞" summary:"取消点赞"`
	PraiseId uint64 `json:"praiseId" v:"required|min:1" dc:"点赞 ID"`
}

type PraiseDeleteRes struct{}

type PraiseListReq struct {
	g.Meta     `path:"/praise" method:"get" tags:"点赞" summary:"我的点赞列表"`
	ObjectType uint32 `json:"objectType" v:"required|in:1,2" dc:"对象类型：1 商品 2 文章"`
	Page       uint32 `json:"page"       v:"min:1"   d:"1"  dc:"页码，从 1 开始"`
	Size       uint32 `json:"size"       v:"max:100" d:"10" dc:"每页条数"`
}

type PraiseListRes struct {
	Items []*PraiseItem `json:"items" dc:"点赞列表"`
	Page  uint32        `json:"page"  dc:"当前页码"`
	Size  uint32        `json:"size"  dc:"每页条数"`
	Total uint64        `json:"total" dc:"总数"`
}

type PraiseItem struct {
	PraiseId   uint64      `json:"praiseId"     dc:"点赞 ID"`
	ObjectType uint32      `json:"objectType" dc:"对象类型：1 商品 2 文章"`
	ObjectId   uint64      `json:"objectId"   dc:"对象 ID"`
	CreatedAt  *gtime.Time `json:"createdAt"  dc:"创建时间"`
}
