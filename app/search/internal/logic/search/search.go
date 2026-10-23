// Package search validates search requests before they reach Elasticsearch.
package search

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

type sSearch struct {
	index service.Index
}

func New(index service.Index) service.ISearch {
	return &sSearch{index: index}
}

func (s *sSearch) Search(ctx context.Context, q *service.Query) ([]service.Hit, int, error) {
	q.Keyword = strings.TrimSpace(q.Keyword)
	q.Brand = strings.TrimSpace(q.Brand)
	if utf8.RuneCountInString(q.Keyword) > service.MaxKeywordRunes ||
		utf8.RuneCountInString(q.Brand) > service.MaxBrandRunes ||
		q.Page < 1 || q.Size < 1 || q.Size > service.MaxPageSize ||
		// Elasticsearch refuses from+size past its result window; say so
		// here with a clear error instead of passing on its 400.
		q.Page*q.Size > service.MaxResultWindow ||
		(q.MaxPriceCents > 0 && q.MinPriceCents > q.MaxPriceCents) ||
		q.Sort < service.SortRelevance || q.Sort > service.SortNewest {
		return nil, 0, service.ErrInvalidArgument
	}
	return s.index.Search(ctx, q)
}
