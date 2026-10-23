package goods

import (
	"context"

	categoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/api/goods/v1"
)

func (c *ControllerV1) CategoryUpdate(ctx context.Context, req *v1.CategoryUpdateReq) (res *v1.CategoryUpdateRes, err error) {
	_, err = c.CategoriesClient.UpdateCategory(ctx, &categoryv1.UpdateCategoryRequest{
		CategoryId: req.CategoryId,
		Name:       req.Name,
		IconFileId: req.IconFileId,
		Sort:       req.Sort,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CategoryUpdateRes{}, nil
}
