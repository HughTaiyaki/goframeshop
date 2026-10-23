// Package clients calls the file and goods services with the caller's own
// administrator token, forwarded from the incoming call.
package clients

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"google.golang.org/grpc/codes"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
	filev1 "github.com/hughtaiyaki/goframeshop/app/file/api/file/v1"
	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Files struct{ files filev1.FileServiceClient }

func NewFiles() *Files {
	conn := grpcx.Client.MustNewGrpcClientConn("file", grpcx.Client.ChainUnary(utility.GrpcClientTimeout, utility.GrpcClientForwardAuth))
	return &Files{files: filev1.NewFileServiceClient(conn)}
}

func (f *Files) URL(ctx context.Context, fileID uint64) (string, error) {
	res, err := f.files.GetFile(ctx, &filev1.GetFileRequest{FileId: fileID})
	if utility.RPCCode(err) == codes.NotFound {
		return "", service.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return res.GetFile().GetUrl(), nil
}

type Catalog struct{ goods goodsv1.GoodsServiceClient }

func NewCatalog() *Catalog {
	conn := grpcx.Client.MustNewGrpcClientConn("goods", grpcx.Client.ChainUnary(utility.GrpcClientTimeout, utility.GrpcClientForwardAuth))
	return &Catalog{goods: goodsv1.NewGoodsServiceClient(conn)}
}

func (c *Catalog) GoodsName(ctx context.Context, goodsID uint64) (string, error) {
	res, err := c.goods.GetGoods(ctx, &goodsv1.GetGoodsRequest{GoodsId: goodsID})
	if utility.RPCCode(err) == codes.NotFound {
		return "", service.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return res.GetGoods().GetName(), nil
}
