// Package rotation is the gRPC transport layer for the carousel.
package rotation

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/banner/api/rotation/v1"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
)

type Controller struct {
	v1.UnimplementedRotationServiceServer
	rotations service.IRotation
}

func Register(s *grpcx.GrpcServer, rotations service.IRotation) {
	v1.RegisterRotationServiceServer(s.Server, &Controller{rotations: rotations})
}

func (c *Controller) ListRotations(ctx context.Context, req *v1.ListRotationsRequest) (*v1.ListRotationsResponse, error) {
	items, total, err := c.rotations.List(ctx, int(req.Page), int(req.Size))
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	res := &v1.ListRotationsResponse{Items: make([]*v1.Rotation, 0, len(items)), Page: req.Page, Size: req.Size, Total: uint64(total)}
	for _, x := range items {
		res.Items = append(res.Items, &v1.Rotation{
			RotationId: x.ID, FileId: x.FileID, PicUrl: x.PicURL, Link: x.Link, Sort: x.Sort,
			CreatedAt: controller.Timestamp(x.CreatedAt), UpdatedAt: controller.Timestamp(x.UpdatedAt),
		})
	}
	return res, nil
}

func (c *Controller) CreateRotation(ctx context.Context, req *v1.CreateRotationRequest) (*v1.CreateRotationResponse, error) {
	id, err := c.rotations.Create(ctx, &service.RotationInput{FileID: req.FileId, Link: req.Link, Sort: req.Sort})
	if err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.CreateRotationResponse{RotationId: id}, nil
}

func (c *Controller) UpdateRotation(ctx context.Context, req *v1.UpdateRotationRequest) (*v1.UpdateRotationResponse, error) {
	if err := c.rotations.Update(ctx, req.RotationId, &service.RotationInput{FileID: req.FileId, Link: req.Link, Sort: req.Sort}); err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.UpdateRotationResponse{}, nil
}

func (c *Controller) DeleteRotation(ctx context.Context, req *v1.DeleteRotationRequest) (*v1.DeleteRotationResponse, error) {
	if err := c.rotations.Delete(ctx, req.RotationId); err != nil {
		return nil, controller.StatusError(ctx, err)
	}
	return &v1.DeleteRotationResponse{}, nil
}
