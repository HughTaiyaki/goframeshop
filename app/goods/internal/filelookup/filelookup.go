// Package filelookup resolves file ids through the file service.
package filelookup

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"google.golang.org/grpc/codes"

	filev1 "github.com/hughtaiyaki/goframeshop/app/file/api/file/v1"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// Client calls FileService.GetFile. The administrator's token is forwarded
// from the incoming call, so the file service checks the same caller again.
type Client struct {
	files filev1.FileServiceClient
}

func New() *Client {
	conn := grpcx.Client.MustNewGrpcClientConn("file", grpcx.Client.ChainUnary(
		utility.GrpcClientTimeout,
		utility.GrpcClientForwardAuth,
	))
	return &Client{files: filev1.NewFileServiceClient(conn)}
}

func (c *Client) URL(ctx context.Context, fileID uint64) (string, error) {
	res, err := c.files.GetFile(ctx, &filev1.GetFileRequest{FileId: fileID})
	if utility.RPCCode(err) == codes.NotFound {
		return "", service.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return res.GetFile().GetUrl(), nil
}
