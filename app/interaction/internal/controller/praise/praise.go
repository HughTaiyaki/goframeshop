// Package praise is the gRPC transport layer for praises.
package praise

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/praise/v1"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Controller struct {
	v1.UnimplementedPraiseServiceServer
	praises service.IMark
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterPraiseServiceServer(s.Server, &Controller{praises: logic.Praise()})
}

func (c *Controller) ListPraises(ctx context.Context, req *v1.ListPraisesRequest) (*v1.ListPraisesResponse, error) {
	userID, err := controller.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := c.praises.List(ctx, userID, req.ObjectType, int(req.Page), int(req.Size))
	if err != nil {
		return nil, controller.StatusError(err)
	}
	response := &v1.ListPraisesResponse{
		Items: make([]*v1.Praise, 0, len(items)),
		Page:  req.Page,
		Size:  req.Size,
		Total: uint64(total),
	}
	for _, item := range items {
		response.Items = append(response.Items, &v1.Praise{
			PraiseId:   item.ID,
			ObjectType: item.ObjectType,
			ObjectId:   item.ObjectID,
			CreatedAt:  utility.SafeConvertTime(item.CreatedAt),
		})
	}
	return response, nil
}

func (c *Controller) CreatePraise(ctx context.Context, req *v1.CreatePraiseRequest) (*v1.CreatePraiseResponse, error) {
	userID, err := controller.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := c.praises.Create(ctx, userID, req.ObjectType, req.ObjectId)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.CreatePraiseResponse{PraiseId: id}, nil
}

func (c *Controller) DeletePraise(ctx context.Context, req *v1.DeletePraiseRequest) (*v1.DeletePraiseResponse, error) {
	userID, err := controller.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.praises.Delete(ctx, userID, req.PraiseId); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.DeletePraiseResponse{}, nil
}
