package goods

import (
	"context"

	categoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) CategoryList(ctx context.Context, req *v1.CategoryListReq) (res *v1.CategoryListRes, err error) {
	grpcRes, err := c.CategoriesClient.ListCategories(ctx, &categoryv1.ListCategoriesRequest{
		ParentId: req.ParentId,
		Page:     req.Page,
		Size:     req.Size,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CategoryListRes{Items: toCategoryItems(grpcRes.Items), Page: req.Page, Size: req.Size, Total: grpcRes.Total}, nil
}
