// Package goods is the gRPC transport layer for products.
//
// Reads are public. Writes are admin-only, which the auth interceptor already
// enforces (deny by default, reads listed as public); there is no per-handler
// identity check to forget. On the reads, a verified administrator also sees
// goods off shelf and the reserved quantity.
package goods

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"

	v1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/controller"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

type Controller struct {
	v1.UnimplementedGoodsServiceServer
	goods service.IGoods
}

func Register(s *grpcx.GrpcServer) {
	v1.RegisterGoodsServiceServer(s.Server, &Controller{goods: logic.Goods()})
}

func (c *Controller) ListGoods(ctx context.Context, req *v1.ListGoodsRequest) (*v1.ListGoodsResponse, error) {
	shelf := service.ShelfOn
	if controller.IsAdmin(ctx) {
		shelf = service.ShelfFilter(req.Shelf)
	}
	filter := service.GoodsFilter{CategoryID: req.CategoryId, Shelf: shelf, HotOnly: req.HotOnly}
	items, total, err := c.goods.List(ctx, filter, int(req.Page), int(req.Size))
	if err != nil {
		return nil, controller.StatusError(err)
	}
	res := &v1.ListGoodsResponse{
		Items: make([]*v1.GoodsSummary, 0, len(items)),
		Page:  req.Page,
		Size:  req.Size,
		Total: uint64(total),
	}
	for _, item := range items {
		res.Items = append(res.Items, &v1.GoodsSummary{
			GoodsId:    item.ID,
			Name:       item.Name,
			PriceCents: item.PriceCents,
			Brand:      item.Brand,
			Stock:      item.Stock.Available,
			Sale:       item.Stock.Sold,
			Tags:       item.Tags,
			CoverUrl:   item.CoverURL,
			OnShelf:    item.OnShelf,
			Sort:       item.Sort,
		})
	}
	return res, nil
}

func (c *Controller) GetGoods(ctx context.Context, req *v1.GetGoodsRequest) (*v1.GetGoodsResponse, error) {
	admin := controller.IsAdmin(ctx)
	goods, err := c.goods.Get(ctx, req.GoodsId, admin)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	out := toProto(goods)
	if !admin {
		out.Reserved = 0
	}
	return &v1.GetGoodsResponse{Goods: out}, nil
}

func (c *Controller) CreateGoods(ctx context.Context, req *v1.CreateGoodsRequest) (*v1.CreateGoodsResponse, error) {
	id, err := c.goods.Create(ctx, toInput(req.Goods), req.InitialStock)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.CreateGoodsResponse{GoodsId: id}, nil
}

func (c *Controller) UpdateGoods(ctx context.Context, req *v1.UpdateGoodsRequest) (*v1.UpdateGoodsResponse, error) {
	if err := c.goods.Update(ctx, req.GoodsId, toInput(req.Goods)); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.UpdateGoodsResponse{}, nil
}

func (c *Controller) DeleteGoods(ctx context.Context, req *v1.DeleteGoodsRequest) (*v1.DeleteGoodsResponse, error) {
	if err := c.goods.Delete(ctx, req.GoodsId); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.DeleteGoodsResponse{}, nil
}

func (c *Controller) AddGoodsImage(ctx context.Context, req *v1.AddGoodsImageRequest) (*v1.AddGoodsImageResponse, error) {
	id, err := c.goods.AddImage(ctx, req.GoodsId, req.FileId, req.Sort)
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.AddGoodsImageResponse{ImageId: id}, nil
}

func (c *Controller) RemoveGoodsImage(ctx context.Context, req *v1.RemoveGoodsImageRequest) (*v1.RemoveGoodsImageResponse, error) {
	if err := c.goods.RemoveImage(ctx, req.ImageId); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.RemoveGoodsImageResponse{}, nil
}

func (c *Controller) SetGoodsShelf(ctx context.Context, req *v1.SetGoodsShelfRequest) (*v1.SetGoodsShelfResponse, error) {
	if err := c.goods.SetShelf(ctx, req.GoodsId, req.OnShelf); err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.SetGoodsShelfResponse{}, nil
}

func (c *Controller) AdjustStock(ctx context.Context, req *v1.AdjustStockRequest) (*v1.AdjustStockResponse, error) {
	stock, err := c.goods.AdjustStock(ctx, req.GoodsId, int64(req.Delta))
	if err != nil {
		return nil, controller.StatusError(err)
	}
	return &v1.AdjustStockResponse{Available: stock.Available, Reserved: stock.Reserved, Sold: stock.Sold}, nil
}

func toInput(in *v1.GoodsInput) *service.GoodsInput {
	return &service.GoodsInput{
		Name:             in.GetName(),
		PriceCents:       in.GetPriceCents(),
		Level1CategoryID: in.GetLevel1CategoryId(),
		Level2CategoryID: in.GetLevel2CategoryId(),
		Level3CategoryID: in.GetLevel3CategoryId(),
		Brand:            in.GetBrand(),
		Tags:             in.GetTags(),
		DetailInfo:       in.GetDetailInfo(),
		Sort:             in.GetSort(),
	}
}

func toProto(goods *service.Goods) *v1.Goods {
	out := &v1.Goods{
		GoodsId:          goods.ID,
		Name:             goods.Name,
		PriceCents:       goods.PriceCents,
		Level1CategoryId: goods.Level1CategoryID,
		Level2CategoryId: goods.Level2CategoryID,
		Level3CategoryId: goods.Level3CategoryID,
		Brand:            goods.Brand,
		Stock:            goods.Stock.Available,
		Sale:             goods.Stock.Sold,
		Reserved:         goods.Stock.Reserved,
		OnShelf:          goods.OnShelf,
		Sort:             goods.Sort,
		Tags:             goods.Tags,
		DetailInfo:       goods.DetailInfo,
		Images:           make([]*v1.GoodsImage, 0, len(goods.Images)),
		CreatedAt:        controller.Timestamp(goods.CreatedAt),
		UpdatedAt:        controller.Timestamp(goods.UpdatedAt),
	}
	for _, image := range goods.Images {
		out.Images = append(out.Images, &v1.GoodsImage{
			ImageId: image.ID, FileId: image.FileID, Url: image.URL, Sort: image.Sort,
		})
	}
	return out
}
