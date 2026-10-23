// Package search is the gRPC transport layer for goods search.
package search

import (
	"context"
	"errors"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/frame/g"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "github.com/hughtaiyaki/goframeshop/app/search/api/search/v1"
	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

type Controller struct {
	v1.UnimplementedSearchServiceServer
	search service.ISearch
}

func Register(s *grpcx.GrpcServer, search service.ISearch) {
	v1.RegisterSearchServiceServer(s.Server, &Controller{search: search})
}

func (c *Controller) SearchGoods(ctx context.Context, req *v1.SearchGoodsRequest) (*v1.SearchGoodsResponse, error) {
	hits, total, err := c.search.Search(ctx, &service.Query{
		Keyword:       req.Keyword,
		CategoryID:    req.CategoryId,
		Brand:         req.Brand,
		MinPriceCents: req.MinPriceCents,
		MaxPriceCents: req.MaxPriceCents,
		Sort:          service.SortBy(req.Sort),
		Page:          int(req.Page),
		Size:          int(req.Size),
	})
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		return nil, status.Error(codes.InvalidArgument, "invalid argument")
	case err != nil:
		// The cause (an Elasticsearch error) stays in the log.
		g.Log().Errorf(ctx, "search failed: %v", err)
		return nil, status.Error(codes.Unavailable, "search is temporarily unavailable")
	}
	res := &v1.SearchGoodsResponse{Items: make([]*v1.SearchHit, 0, len(hits)), Page: req.Page, Size: req.Size, Total: uint64(total)}
	for _, h := range hits {
		res.Items = append(res.Items, &v1.SearchHit{
			GoodsId:          h.GoodsID,
			Name:             h.Name,
			HighlightedName:  h.HighlightedName,
			PriceCents:       h.PriceCents,
			Brand:            h.Brand,
			Stock:            h.Stock,
			Sale:             h.Sale,
			CoverUrl:         h.CoverURL,
			Level1CategoryId: h.Level1CategoryID,
			Level2CategoryId: h.Level2CategoryID,
			Level3CategoryId: h.Level3CategoryID,
		})
	}
	return res, nil
}
