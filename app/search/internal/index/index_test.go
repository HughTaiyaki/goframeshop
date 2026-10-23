package index

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

func render(t *testing.T, q *service.Query) string {
	t.Helper()
	raw, err := json.Marshal(BuildSearch(q))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestBuildSearch(t *testing.T) {
	keyword := render(t, &service.Query{Keyword: "电视", Page: 3, Size: 20})
	for _, want := range []string{
		`"multi_match":{"fields":["name^3","tags"]`,
		`"query":"电视"`,
		`"sort":[{"_score":"desc"},{"goods_id":"desc"}]`,
		`"from":40`, `"size":20`,
		`"encoder":"html"`,
		`"track_total_hits":true`,
	} {
		if !strings.Contains(keyword, want) {
			t.Errorf("keyword query lacks %s\n%s", want, keyword)
		}
	}

	browse := render(t, &service.Query{Page: 1, Size: 10})
	if strings.Contains(browse, "multi_match") || !strings.Contains(browse, `"sort":[{"goods_id":"desc"}]`) {
		t.Errorf("no keyword: want newest first and no text query\n%s", browse)
	}

	filters := render(t, &service.Query{CategoryID: 7, Brand: "ACME", MinPriceCents: 100, MaxPriceCents: 900, Sort: service.SortPriceAsc, Page: 1, Size: 10})
	for _, want := range []string{
		`{"term":{"brand":"ACME"}}`,
		`{"term":{"level1_category_id":7}}`, `{"term":{"level2_category_id":7}}`, `{"term":{"level3_category_id":7}}`,
		`"minimum_should_match":1`,
		`{"range":{"price_cents":{"gte":100,"lte":900}}}`,
		`"sort":[{"price_cents":"asc"},{"goods_id":"desc"}]`,
	} {
		if !strings.Contains(filters, want) {
			t.Errorf("filtered query lacks %s\n%s", want, filters)
		}
	}
	// Filters must not affect scoring; they belong under filter, not must.
	if strings.Contains(filters, `"must"`) {
		t.Errorf("filters leaked into must:\n%s", filters)
	}

	if sales := render(t, &service.Query{Sort: service.SortSales, Page: 1, Size: 10}); !strings.Contains(sales, `"sort":[{"sale":"desc"},{"goods_id":"desc"}]`) {
		t.Errorf("sales sort:\n%s", sales)
	}
	if minOnly := render(t, &service.Query{MinPriceCents: 5, Page: 1, Size: 10}); strings.Contains(minOnly, `"lte"`) {
		t.Errorf("an unset max became a bound:\n%s", minOnly)
	}
}
