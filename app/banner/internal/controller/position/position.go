// Package position is the gRPC transport layer for hand-picked goods slots.
package position

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/banner/api/position/v1"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
)

type Controller struct {
	v1.UnimplementedPositionServiceServer
	positions service.IPosition
}

func Register(s *grpcx.GrpcServer, positions service.IPosition) {
	v1.RegisterPositionServiceServer(s.Server, &Controller{positions: positions})
}

func (c *Controller) ListPositions(ctx context.Context, req *v1.ListPositionsRequest) (*v1.ListPositionsResponse, error) {
	items, total, err := c.positions.List(ctx, int(req.Page), int(req.Size), req.AvailableOnly)
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	res := &v1.ListPositionsResponse{Items: make([]*v1.Position, 0, len(items)), Page: req.Page, Size: req.Size, Total: uint64(total)}
	for _, x := range items {
		res.Items = append(res.Items, &v1.Position{
			PositionId: x.ID, FileId: x.FileID, PicUrl: x.PicURL, GoodsId: x.GoodsID, GoodsName: x.GoodsName,
			Link: x.Link, Sort: x.Sort, CreatedAt: controller.Timestamp(x.CreatedAt), UpdatedAt: controller.Timestamp(x.UpdatedAt),
			GoodsOnShelf: x.GoodsOnShelf,
		})
	}
	return res, nil
}

func (c *Controller) CreatePosition(ctx context.Context, req *v1.CreatePositionRequest) (*v1.CreatePositionResponse, error) {
	id, err := c.positions.Create(ctx, &service.PositionInput{FileID: req.FileId, GoodsID: req.GoodsId, Link: req.Link, Sort: req.Sort})
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.CreatePositionResponse{PositionId: id}, nil
}

func (c *Controller) UpdatePosition(ctx context.Context, req *v1.UpdatePositionRequest) (*v1.UpdatePositionResponse, error) {
	in := &service.PositionInput{FileID: req.FileId, GoodsID: req.GoodsId, Link: req.Link, Sort: req.Sort}
	if err := c.positions.Update(ctx, req.PositionId, in); err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.UpdatePositionResponse{}, nil
}

func (c *Controller) DeletePosition(ctx context.Context, req *v1.DeletePositionRequest) (*v1.DeletePositionResponse, error) {
	if err := c.positions.Delete(ctx, req.PositionId); err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.DeletePositionResponse{}, nil
}
