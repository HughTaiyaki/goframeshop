package goods

import (
	"context"

	categoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) CategoryCreate(ctx context.Context, req *v1.CategoryCreateReq) (res *v1.CategoryCreateRes, err error) {
	grpcRes, err := c.CategoriesClient.CreateCategory(ctx, &categoryv1.CreateCategoryRequest{
		ParentId:   req.ParentId,
		Name:       req.Name,
		IconFileId: req.IconFileId,
		Sort:       req.Sort,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CategoryCreateRes{CategoryId: grpcRes.CategoryId}, nil
}
