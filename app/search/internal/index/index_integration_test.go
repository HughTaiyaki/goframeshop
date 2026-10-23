package index

// Runs against the Compose Elasticsearch (with IK):
//
//	make infra-up
//	GOFRAMESHOP_IT=1 go test ./app/search/internal/index/ -run Integration -v
//
// Every test uses its own alias and deletes its indices afterwards.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

func integrationES(t *testing.T) (*ES, context.Context) {
	t.Helper()
	if os.Getenv("GOFRAMESHOP_IT") != "1" {
		t.Skip("set GOFRAMESHOP_IT=1 to run against the Compose Elasticsearch")
	}
	address := os.Getenv("ES_IT_ADDRESS")
	if address == "" {
		address = "http://127.0.0.1:9200"
	}
	alias := fmt.Sprintf("it_goods_%d", time.Now().UnixNano())
	es, err := New([]string{address}, alias)
	if err != nil {
		t.Fatal(err)
	}
	es.Refresh = "wait_for"
	ctx := context.Background()
	if err := es.Ping(ctx); err != nil {
		t.Fatalf("cannot reach Elasticsearch: %v", err)
	}
	// Elasticsearch 8 refuses wildcard deletes (action.destructive_requires_name),
	// so delete the test's physical indices by name.
	t.Cleanup(func() {
		targets, err := es.aliasTargets(ctx)
		if err != nil {
			t.Errorf("cleanup: %v", err)
		}
		for _, index := range targets {
			if _, err := es.do(ctx, http.MethodDelete, "/"+index, nil, nil); err != nil {
				t.Errorf("cleanup of %s: %v", index, err)
			}
		}
	})
	return es, ctx
}

func docs(list ...*service.GoodsDoc) func(emit func(*service.GoodsDoc) error) error {
	return func(emit func(*service.GoodsDoc) error) error {
		for _, d := range list {
			if err := emit(d); err != nil {
				return err
			}
		}
		return nil
	}
}

func search(t *testing.T, es *ES, ctx context.Context, q *service.Query) ([]service.Hit, int) {
	t.Helper()
	if q.Page == 0 {
		q.Page, q.Size = 1, 10
	}
	hits, total, err := es.Search(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	return hits, total
}

func TestIntegrationChineseSearchFiltersAndHighlight(t *testing.T) {
	es, ctx := integrationES(t)
	n, err := es.Rebuild(ctx, docs(
		&service.GoodsDoc{GoodsID: 1, Name: "全面屏智能电视机 65 英寸", Brand: "ACME", PriceCents: 399900, Level1CategoryID: 1, Level2CategoryID: 2, Sale: 5},
		&service.GoodsDoc{GoodsID: 2, Name: "智能手机", Brand: "Other", PriceCents: 199900, Level1CategoryID: 3, Sale: 50},
		&service.GoodsDoc{GoodsID: 3, Name: "<script>alert(1)</script> 电视支架", Tags: "配件", Brand: "ACME", PriceCents: 9900, Level1CategoryID: 1},
	))
	if err != nil || n != 3 {
		t.Fatalf("rebuild: n=%d err=%v", n, err)
	}

	// IK splits 电视 out of 电视机 / 电视支架; the standard analyzer would not
	// match a two-character word against these names.
	hits, total := search(t, es, ctx, &service.Query{Keyword: "电视"})
	if total != 2 {
		t.Fatalf("电视: total=%d hits=%+v", total, hits)
	}
	for _, h := range hits {
		if !strings.Contains(h.HighlightedName, "<em>") {
			t.Errorf("no highlight in %q", h.HighlightedName)
		}
		if strings.Contains(h.HighlightedName, "<script>") {
			t.Fatalf("highlight returned raw HTML from the name: %q", h.HighlightedName)
		}
	}

	if _, total := search(t, es, ctx, &service.Query{Keyword: "配件"}); total != 1 {
		t.Errorf("tags are not searched: total=%d", total)
	}
	if _, total := search(t, es, ctx, &service.Query{Brand: "ACME"}); total != 2 {
		t.Errorf("brand filter: total=%d", total)
	}
	if _, total := search(t, es, ctx, &service.Query{CategoryID: 2}); total != 1 {
		t.Errorf("level2 category filter: total=%d", total)
	}
	if _, total := search(t, es, ctx, &service.Query{MinPriceCents: 10000, MaxPriceCents: 200000}); total != 1 {
		t.Errorf("price range: total=%d", total)
	}
	if hits, _ := search(t, es, ctx, &service.Query{Sort: service.SortSales}); hits[0].GoodsID != 2 {
		t.Errorf("sales sort: first=%d", hits[0].GoodsID)
	}
	if hits, _ := search(t, es, ctx, &service.Query{Sort: service.SortPriceAsc}); hits[0].GoodsID != 3 {
		t.Errorf("price sort: first=%d", hits[0].GoodsID)
	}
	// A name without a match is still returned HTML-escaped.
	hits, _ = search(t, es, ctx, &service.Query{Keyword: "支架"})
	if len(hits) != 1 || strings.Contains(hits[0].HighlightedName, "<script>") {
		t.Fatalf("escaping: %+v", hits)
	}
}

func TestIntegrationUpsertDeleteAndRebuildSwap(t *testing.T) {
	es, ctx := integrationES(t)
	if _, err := es.Rebuild(ctx, docs(&service.GoodsDoc{GoodsID: 1, Name: "旧商品"})); err != nil {
		t.Fatal(err)
	}
	first, _ := es.aliasTargets(ctx)

	if err := es.Upsert(ctx, &service.GoodsDoc{GoodsID: 2, Name: "新商品"}); err != nil {
		t.Fatal(err)
	}
	if _, total := search(t, es, ctx, &service.Query{}); total != 2 {
		t.Fatalf("after upsert: total=%d", total)
	}
	if err := es.Delete(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if err := es.Delete(ctx, 2); err != nil {
		t.Fatalf("deleting a missing document must be a no-op: %v", err)
	}

	// A rebuild replaces the contents atomically and drops the old index.
	if _, err := es.Rebuild(ctx, docs(&service.GoodsDoc{GoodsID: 9, Name: "重建后"})); err != nil {
		t.Fatal(err)
	}
	hits, total := search(t, es, ctx, &service.Query{})
	if total != 1 || hits[0].GoodsID != 9 {
		t.Fatalf("after rebuild: %+v", hits)
	}
	second, _ := es.aliasTargets(ctx)
	if len(second) != 1 || second[0] == first[0] {
		t.Fatalf("alias targets: before=%v after=%v", first, second)
	}
	if _, err := es.do(ctx, http.MethodGet, "/"+first[0], nil, nil); !isStatus(err, http.StatusNotFound) {
		t.Fatalf("old index still exists: %v", err)
	}
}
