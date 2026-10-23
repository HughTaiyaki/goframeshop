package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// 评论列表公开可读；发表和删除需要登录，作者 ID 来自令牌。

type CommentListReq struct {
	g.Meta     `path:"/comment" method:"get" tags:"评论" summary:"对象的评论列表（公开）"`
	ObjectType uint32 `json:"objectType" v:"required|in:1,2" dc:"对象类型：1 商品 2 文章"`
	ObjectId   uint64 `json:"objectId"   v:"required|min:1"  dc:"对象 ID"`
	Page       uint32 `json:"page"       v:"min:1"   d:"1"  dc:"页码，从 1 开始"`
	Size       uint32 `json:"size"       v:"max:100" d:"10" dc:"每页条数"`
}

// CommentListRes 是一个扁平列表，按 ID 倒序；客户端用 parentId 自己拼回复树。
type CommentListRes struct {
	Items []*CommentItem `json:"items" dc:"评论列表"`
	Page  uint32         `json:"page"  dc:"当前页码"`
	Size  uint32         `json:"size"  dc:"每页条数"`
	Total uint64         `json:"total" dc:"总数"`
}

type CommentItem struct {
	CommentId  uint64      `json:"commentId"  dc:"评论 ID"`
	ParentId   uint64      `json:"parentId"   dc:"父评论 ID，0 表示顶层评论"`
	UserId     uint64      `json:"userId"     dc:"作者 ID"`
	ObjectType uint32      `json:"objectType" dc:"对象类型：1 商品 2 文章"`
	ObjectId   uint64      `json:"objectId"   dc:"对象 ID"`
	Content    string      `json:"content"    dc:"评论内容"`
	CreatedAt  *gtime.Time `json:"createdAt"  dc:"发表时间"`
}

type CommentCreateReq struct {
	g.Meta     `path:"/comment" method:"post" tags:"评论" summary:"发表评论或回复"`
	ObjectType uint32 `json:"objectType" v:"required|in:1,2"       dc:"对象类型：1 商品 2 文章"`
	ObjectId   uint64 `json:"objectId"   v:"required|min:1"        dc:"对象 ID"`
	ParentId   uint64 `json:"parentId"                             dc:"父评论 ID，不填或 0 表示顶层评论"`
	Content    string `json:"content"    v:"required|length:1,255" dc:"评论内容"`
}

type CommentCreateRes struct {
	CommentId uint64 `json:"commentId" dc:"新评论 ID"`
}

type CommentDeleteReq struct {
	g.Meta    `path:"/comment" method:"delete" tags:"评论" summary:"删除自己的评论"`
	CommentId uint64 `json:"commentId" v:"required|min:1" dc:"评论 ID"`
}

type CommentDeleteRes struct{}
