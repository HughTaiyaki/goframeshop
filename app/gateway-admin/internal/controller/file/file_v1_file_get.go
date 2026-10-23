package file

import (
	"context"

	filev1 "github.com/hughtaiyaki/goframeshop/app/file/api/file/v1"
	"github.com/hughtaiyaki/goframeshop/utility"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/file/v1"
)

func (c *ControllerV1) FileGet(ctx context.Context, req *v1.FileGetReq) (res *v1.FileGetRes, err error) {
	grpcRes, err := c.FilesClient.GetFile(ctx, &filev1.GetFileRequest{FileId: req.FileId})
	if err != nil {
		return nil, err
	}
	f := grpcRes.GetFile()
	return &v1.FileGetRes{
		FileId:      f.GetFileId(),
		UploaderId:  f.GetUploaderId(),
		Name:        f.GetName(),
		Url:         f.GetUrl(),
		Size:        f.GetSize(),
		ContentType: f.GetContentType(),
		CreatedAt:   utility.TimeFromTimestamp(f.GetCreatedAt()),
	}, nil
}
