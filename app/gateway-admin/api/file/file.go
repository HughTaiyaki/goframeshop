// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package file

import (
	"context"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/file/v1"
)

type IFileV1 interface {
	UploadImage(ctx context.Context, req *v1.UploadImageReq) (res *v1.UploadImageRes, err error)
	FileGet(ctx context.Context, req *v1.FileGetReq) (res *v1.FileGetRes, err error)
}
