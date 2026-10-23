package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gtime"
)

// UploadImageReq 通过 multipart/form-data 上传，表单字段名为 file。
// 文件类型由文件内容判断，不看文件名和客户端声明的 Content-Type。
type UploadImageReq struct {
	g.Meta `path:"/upload/image" method:"post" mime:"multipart/form-data" tags:"文件" summary:"上传图片（JPEG/PNG/GIF/WebP）"`
	File   *ghttp.UploadFile `json:"file" type:"file" v:"required" dc:"图片文件"`
}

// UploadImageRes 返回 fileId：其他表应保存 fileId，而不是复制 URL。
type UploadImageRes struct {
	FileId uint64 `json:"fileId" dc:"文件 ID"`
	Url    string `json:"url"    dc:"访问地址"`
}

type FileGetReq struct {
	g.Meta `path:"/file" method:"get" tags:"文件" summary:"查询文件信息"`
	FileId uint64 `json:"fileId" v:"required|min:1" dc:"文件 ID"`
}

type FileGetRes struct {
	FileId      uint64      `json:"fileId"      dc:"文件 ID"`
	UploaderId  uint64      `json:"uploaderId"  dc:"上传者（管理员）ID"`
	Name        string      `json:"name"        dc:"原始文件名，仅用于展示"`
	Url         string      `json:"url"         dc:"访问地址"`
	Size        uint64      `json:"size"        dc:"字节数"`
	ContentType string      `json:"contentType" dc:"文件类型"`
	CreatedAt   *gtime.Time `json:"createdAt"   dc:"上传时间"`
}
