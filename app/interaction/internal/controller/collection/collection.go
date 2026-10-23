// Package collection is the gRPC transport layer for collections.
package collection

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/collection/v1"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

type Controller struct {
	v1.UnimplementedCollectionServiceServer
	collections service.IMark
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterCollectionServiceServer(s.Server, &Controller{collections: logic.Collection()})
}

func (c *Controller) ListCollections(ctx context.Context, req *v1.ListCollectionsRequest) (*v1.ListCollectionsResponse, error) {
	userID, err := controller.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := c.collections.List(ctx, userID, req.ObjectType, int(req.Page), int(req.Size))
	if err != nil {
		return nil, controller.StatusError(err)
	}
	response := &v1.ListCollectionsResponse{
		Items: make([]*v1.Collection, 0, len(items)),
		Page:  req.Page,
		Size:  req.Size,
		Total: uint64(total),
	}
	for _, item := range items {
		response.Items = append(response.Items, &v1.Collection{
			CollectionId: item.ID,
			ObjectType:   item.ObjectType,
			ObjectId:     item.ObjectID,
			CreatedAt:    utility.SafeConvertTime(item.CreatedAt),
		})
	}
	return response, nil
}

func (c *Controller) CreateCollection(ctx context.Context, req *v1.CreateCollectionRequest) (*v1.CreateCollectionResponse, error) {
	userID, err := controller.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := c.collections.Create(ctx, userID, req.ObjectType, req.ObjectId)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.CreateCollectionResponse{CollectionId: id}, nil
}

func (c *Controller) DeleteCollection(ctx context.Context, req *v1.DeleteCollectionRequest) (*v1.DeleteCollectionResponse, error) {
	userID, err := controller.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.collections.Delete(ctx, userID, req.CollectionId); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.DeleteCollectionResponse{}, nil
}
