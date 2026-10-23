package search

import (
	"context"

	searchv1 "github.com/hughtaiyaki/goframeshop/app/search/api/search/v1"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/api/search/v1"
)

var sorts = map[string]searchv1.SortBy{
	"":           searchv1.SortBy_SORT_BY_RELEVANCE,
	"relevance":  searchv1.SortBy_SORT_BY_RELEVANCE,
	"price_asc":  searchv1.SortBy_SORT_BY_PRICE_ASC,
	"price_desc": searchv1.SortBy_SORT_BY_PRICE_DESC,
	"sales":      searchv1.SortBy_SORT_BY_SALES,
	"newest":     searchv1.SortBy_SORT_BY_NEWEST,
}

func (c *ControllerV1) SearchGoods(ctx context.Context, req *v1.SearchGoodsReq) (res *v1.SearchGoodsRes, err error) {
	grpcRes, err := c.SearchClient.SearchGoods(ctx, &searchv1.SearchGoodsRequest{
		Keyword:       req.Keyword,
		CategoryId:    req.CategoryId,
		Brand:         req.Brand,
		MinPriceCents: req.MinPriceCents,
		MaxPriceCents: req.MaxPriceCents,
		Sort:          sorts[req.Sort],
		Page:          req.Page,
		Size:          req.Size,
	})
	if err != nil {
		return nil, err
	}
	res = &v1.SearchGoodsRes{Items: make([]*v1.SearchHit, 0, len(grpcRes.Items)), Page: req.Page, Size: req.Size, Total: grpcRes.Total}
	for _, h := range grpcRes.Items {
		res.Items = append(res.Items, &v1.SearchHit{
			GoodsId:         h.GoodsId,
			Name:            h.Name,
			HighlightedName: h.HighlightedName,
			PriceCents:      h.PriceCents,
			Brand:           h.Brand,
			Stock:           h.Stock,
			Sale:            h.Sale,
			CoverUrl:        h.CoverUrl,
		})
	}
	return res, nil
}
