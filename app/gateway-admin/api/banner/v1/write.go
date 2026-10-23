package v1

import "github.com/gogf/gf/v2/frame/g"

// 写接口只在后台。图片先用 /backend/upload/image 上传拿到 fileId；
// 链接只接受 http/https 的绝对地址（javascript:、data: 等会被拒绝）。

type RotationCreateReq struct {
	g.Meta `path:"/rotation" method:"post" tags:"首页运营" summary:"新增轮播图"`
	FileId uint64 `json:"fileId" v:"required|min:1"  dc:"图片文件 ID"`
	Link   string `json:"link"   v:"length:0,512"    dc:"跳转链接，可为空"`
	Sort   uint32 `json:"sort"   v:"between:0,999"   dc:"排序，小的在前"`
}

type RotationCreateRes struct {
	RotationId uint64 `json:"rotationId" dc:"新轮播图 ID"`
}

type RotationUpdateReq struct {
	g.Meta     `path:"/rotation" method:"put" tags:"首页运营" summary:"修改轮播图（整体替换）"`
	RotationId uint64 `json:"rotationId" v:"required|min:1" dc:"轮播图 ID"`
	FileId     uint64 `json:"fileId"     v:"required|min:1" dc:"图片文件 ID"`
	Link       string `json:"link"       v:"length:0,512"   dc:"跳转链接，可为空"`
	Sort       uint32 `json:"sort"       v:"between:0,999"  dc:"排序"`
}

type RotationUpdateRes struct{}

type RotationDeleteReq struct {
	g.Meta     `path:"/rotation" method:"delete" tags:"首页运营" summary:"删除轮播图"`
	RotationId uint64 `json:"rotationId" v:"required|min:1" dc:"轮播图 ID"`
}

type RotationDeleteRes struct{}

type PositionCreateReq struct {
	g.Meta  `path:"/position" method:"post" tags:"首页运营" summary:"新增手工位（商品名自动读取）"`
	FileId  uint64 `json:"fileId"  v:"required|min:1" dc:"图片文件 ID"`
	GoodsId uint64 `json:"goodsId" v:"required|min:1" dc:"商品 ID"`
	Link    string `json:"link"    v:"length:0,512"   dc:"跳转链接；为空表示打开商品详情"`
	Sort    uint32 `json:"sort"    v:"between:0,999"  dc:"排序，小的在前"`
}

type PositionCreateRes struct {
	PositionId uint64 `json:"positionId" dc:"新手工位 ID"`
}

type PositionUpdateReq struct {
	g.Meta     `path:"/position" method:"put" tags:"首页运营" summary:"修改手工位（整体替换）"`
	PositionId uint64 `json:"positionId" v:"required|min:1" dc:"手工位 ID"`
	FileId     uint64 `json:"fileId"     v:"required|min:1" dc:"图片文件 ID"`
	GoodsId    uint64 `json:"goodsId"    v:"required|min:1" dc:"商品 ID"`
	Link       string `json:"link"       v:"length:0,512"   dc:"跳转链接"`
	Sort       uint32 `json:"sort"       v:"between:0,999"  dc:"排序"`
}

type PositionUpdateRes struct{}

type PositionDeleteReq struct {
	g.Meta     `path:"/position" method:"delete" tags:"首页运营" summary:"删除手工位"`
	PositionId uint64 `json:"positionId" v:"required|min:1" dc:"手工位 ID"`
}

type PositionDeleteRes struct{}
