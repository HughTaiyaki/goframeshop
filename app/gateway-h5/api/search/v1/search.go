package v1

import "github.com/gogf/gf/v2/frame/g"

// 商品搜索，公开。只能搜到上架且未删除的商品。

type SearchGoodsReq struct {
	g.Meta        `path:"/search/goods" method:"get" tags:"搜索" summary:"商品搜索（Elasticsearch，IK 中文分词）"`
	Keyword       string `json:"keyword"       v:"length:0,100"      dc:"关键词，匹配商品名称和标签；为空时只按筛选条件列出"`
	CategoryId    uint64 `json:"categoryId"                          dc:"按分类筛选（任意层级），0 表示全部"`
	Brand         string `json:"brand"         v:"length:0,30"       dc:"品牌（精确匹配）"`
	MinPriceCents uint64 `json:"minPriceCents"                       dc:"最低价（分），0 表示不限"`
	MaxPriceCents uint64 `json:"maxPriceCents"                       dc:"最高价（分），0 表示不限"`
	Sort          string `json:"sort"          v:"in:,relevance,price_asc,price_desc,sales,newest" dc:"relevance（默认）、price_asc、price_desc、sales、newest"`
	Page          uint32 `json:"page"          v:"min:1"   d:"1"     dc:"页码；page×size 不能超过 10000"`
	Size          uint32 `json:"size"          v:"max:100" d:"10"    dc:"每页条数"`
}

type SearchGoodsRes struct {
	Items []*SearchHit `json:"items" dc:"搜索结果"`
	Page  uint32       `json:"page"  dc:"当前页码"`
	Size  uint32       `json:"size"  dc:"每页条数"`
	Total uint64       `json:"total" dc:"总数"`
}

type SearchHit struct {
	GoodsId         uint64 `json:"goodsId"         dc:"商品 ID"`
	Name            string `json:"name"            dc:"商品名称（纯文本）"`
	HighlightedName string `json:"highlightedName" dc:"命中词用 <em> 包裹，其余内容已做 HTML 转义，可直接作为 HTML 插入"`
	PriceCents      uint64 `json:"priceCents"      dc:"价格（分）"`
	Brand           string `json:"brand"           dc:"品牌"`
	Stock           uint32 `json:"stock"           dc:"可用库存（搜索结果可能有约 1 秒延迟）"`
	Sale            uint32 `json:"sale"            dc:"销量"`
	CoverUrl        string `json:"coverUrl"        dc:"封面图"`
}
