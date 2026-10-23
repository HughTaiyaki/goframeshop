// Package category is the gRPC transport layer for categories.
package category

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type Controller struct {
	v1.UnimplementedCategoryServiceServer
	categories service.ICategory
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterCategoryServiceServer(s.Server, &Controller{categories: logic.Category()})
}

func (c *Controller) ListCategories(ctx context.Context, req *v1.ListCategoriesRequest) (*v1.ListCategoriesResponse, error) {
	items, total, err := c.categories.List(ctx, req.ParentId, int(req.Page), int(req.Size))
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.ListCategoriesResponse{Items: toProtoList(items), Page: req.Page, Size: req.Size, Total: uint64(total)}, nil
}

func (c *Controller) ListAllCategories(ctx context.Context, _ *v1.ListAllCategoriesRequest) (*v1.ListAllCategoriesResponse, error) {
	items, err := c.categories.ListAll(ctx)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.ListAllCategoriesResponse{Items: toProtoList(items)}, nil
}

func (c *Controller) CreateCategory(ctx context.Context, req *v1.CreateCategoryRequest) (*v1.CreateCategoryResponse, error) {
	id, err := c.categories.Create(ctx, &service.CategoryInput{
		ParentID: req.ParentId, Name: req.Name, IconFileID: req.IconFileId, Sort: req.Sort,
	})
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.CreateCategoryResponse{CategoryId: id}, nil
}

func (c *Controller) UpdateCategory(ctx context.Context, req *v1.UpdateCategoryRequest) (*v1.UpdateCategoryResponse, error) {
	err := c.categories.Update(ctx, req.CategoryId, &service.CategoryInput{
		Name: req.Name, IconFileID: req.IconFileId, Sort: req.Sort,
	})
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.UpdateCategoryResponse{}, nil
}

func (c *Controller) DeleteCategory(ctx context.Context, req *v1.DeleteCategoryRequest) (*v1.DeleteCategoryResponse, error) {
	if err := c.categories.Delete(ctx, req.CategoryId); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.DeleteCategoryResponse{}, nil
}

func toProtoList(items []*service.Category) []*v1.Category {
	out := make([]*v1.Category, 0, len(items))
	for _, item := range items {
		out = append(out, &v1.Category{
			CategoryId: item.ID,
			ParentId:   item.ParentID,
			Level:      item.Level,
			Name:       item.Name,
			IconFileId: item.IconFileID,
			IconUrl:    item.IconURL,
			Sort:       item.Sort,
			CreatedAt:  controller.Timestamp(item.CreatedAt),
			UpdatedAt:  controller.Timestamp(item.UpdatedAt),
		})
	}
	return out
}
