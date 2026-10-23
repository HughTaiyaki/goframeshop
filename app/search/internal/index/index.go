// Package index implements service.Index with Elasticsearch.
//
// Searches and single-document writes go to an alias ("goods" by default),
// never to a physical index. Rebuild writes a new physical index
// ("goods_<timestamp>") and then moves the alias to it in one atomic request,
// so a search never sees a half-built index and a mapping change is just a
// rebuild.
package index

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

// Mapping is strict: a field the code does not know about is an error, not a
// silently guessed type. name and tags are analysed with IK, indexing every
// plausible word (ik_max_word) and splitting queries coarsely (ik_smart).
const Mapping = `{
  "settings": {"number_of_shards": 1, "number_of_replicas": 0},
  "mappings": {
    "dynamic": "strict",
    "properties": {
      "goods_id":           {"type": "long"},
      "name":               {"type": "text", "analyzer": "ik_max_word", "search_analyzer": "ik_smart"},
      "tags":               {"type": "text", "analyzer": "ik_max_word", "search_analyzer": "ik_smart"},
      "brand":              {"type": "keyword"},
      "price_cents":        {"type": "long"},
      "level1_category_id": {"type": "long"},
      "level2_category_id": {"type": "long"},
      "level3_category_id": {"type": "long"},
      "stock":              {"type": "integer"},
      "sale":               {"type": "integer"},
      "cover_url":          {"type": "keyword", "index": false},
      "created_at":         {"type": "date"}
    }
  }
}`

const bulkBatch = 500

// ES is the Elasticsearch-backed index.
type ES struct {
	client *elasticsearch.Client
	alias  string
	// Refresh is passed on writes. Empty in production (near real time, about
	// a second); tests set "wait_for" so a write is searchable on return.
	Refresh string
}

func New(addresses []string, alias string) (*ES, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: addresses})
	if err != nil {
		return nil, gerror.Wrap(err, "create elasticsearch client")
	}
	return &ES{client: client, alias: alias}, nil
}

// Ping fails when Elasticsearch is not reachable.
func (e *ES) Ping(ctx context.Context) error {
	_, err := e.do(ctx, http.MethodGet, "/", nil, nil)
	return err
}

func (e *ES) Search(ctx context.Context, q *service.Query) ([]service.Hit, int, error) {
	body, err := json.Marshal(BuildSearch(q))
	if err != nil {
		return nil, 0, err
	}
	raw, err := e.do(ctx, http.MethodPost, "/"+e.alias+"/_search", nil, body)
	if err != nil {
		return nil, 0, err
	}
	var res struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source    service.GoodsDoc    `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, 0, gerror.Wrap(err, "decode search response")
	}
	hits := make([]service.Hit, 0, len(res.Hits.Hits))
	for _, h := range res.Hits.Hits {
		hit := service.Hit{GoodsDoc: h.Source, HighlightedName: html.EscapeString(h.Source.Name)}
		if fragments := h.Highlight["name"]; len(fragments) > 0 {
			hit.HighlightedName = fragments[0]
		}
		hits = append(hits, hit)
	}
	return hits, res.Hits.Total.Value, nil
}

// BuildSearch turns a query into an Elasticsearch request body. It is pure,
// so the query shape is unit-tested without a cluster.
func BuildSearch(q *service.Query) map[string]any {
	var must, filter []any
	if q.Keyword != "" {
		must = append(must, map[string]any{"multi_match": map[string]any{
			"query":  q.Keyword,
			"fields": []string{"name^3", "tags"},
			// Up to two words must all match; longer queries need 75% of
			// them, so a long phrase still finds near matches.
			"minimum_should_match": "2<75%",
		}})
	}
	if q.Brand != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"brand": q.Brand}})
	}
	if q.CategoryID != 0 {
		filter = append(filter, map[string]any{"bool": map[string]any{
			"minimum_should_match": 1,
			"should": []any{
				map[string]any{"term": map[string]any{"level1_category_id": q.CategoryID}},
				map[string]any{"term": map[string]any{"level2_category_id": q.CategoryID}},
				map[string]any{"term": map[string]any{"level3_category_id": q.CategoryID}},
			},
		}})
	}
	if q.MinPriceCents > 0 || q.MaxPriceCents > 0 {
		price := map[string]any{}
		if q.MinPriceCents > 0 {
			price["gte"] = q.MinPriceCents
		}
		if q.MaxPriceCents > 0 {
			price["lte"] = q.MaxPriceCents
		}
		filter = append(filter, map[string]any{"range": map[string]any{"price_cents": price}})
	}
	boolQuery := map[string]any{}
	if len(must) > 0 {
		boolQuery["must"] = must
	}
	if len(filter) > 0 {
		boolQuery["filter"] = filter
	}

	// goods_id breaks every tie, so paging is deterministic.
	newest := map[string]any{"goods_id": "desc"}
	var sort []any
	switch q.Sort {
	case service.SortPriceAsc:
		sort = []any{map[string]any{"price_cents": "asc"}, newest}
	case service.SortPriceDesc:
		sort = []any{map[string]any{"price_cents": "desc"}, newest}
	case service.SortSales:
		sort = []any{map[string]any{"sale": "desc"}, newest}
	case service.SortNewest:
		sort = []any{newest}
	default:
		if q.Keyword != "" {
			sort = []any{map[string]any{"_score": "desc"}, newest}
		} else {
			sort = []any{newest}
		}
	}

	return map[string]any{
		"query":            map[string]any{"bool": boolQuery},
		"sort":             sort,
		"from":             (q.Page - 1) * q.Size,
		"size":             q.Size,
		"track_total_hits": true,
		"highlight": map[string]any{
			// encoder html escapes the stored text, so a product named
			// "<script>" comes back as "&lt;script&gt;" and only the <em>
			// markers are raw HTML.
			"encoder":   "html",
			"pre_tags":  []string{"<em>"},
			"post_tags": []string{"</em>"},
			"fields":    map[string]any{"name": map[string]any{"number_of_fragments": 0}},
		},
	}
}

func (e *ES) Upsert(ctx context.Context, doc *service.GoodsDoc) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = e.do(ctx, http.MethodPut, "/"+e.alias+"/_doc/"+strconv.FormatUint(doc.GoodsID, 10), e.refresh(), body)
	return err
}

// Delete of a document that is not there is not an error: the outcome the
// caller wanted, "not in the index", holds.
func (e *ES) Delete(ctx context.Context, goodsID uint64) error {
	_, err := e.do(ctx, http.MethodDelete, "/"+e.alias+"/_doc/"+strconv.FormatUint(goodsID, 10), e.refresh(), nil)
	if isStatus(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (e *ES) Rebuild(ctx context.Context, load func(emit func(*service.GoodsDoc) error) error) (int, error) {
	physical := fmt.Sprintf("%s_%d", e.alias, time.Now().UnixNano())
	if _, err := e.do(ctx, http.MethodPut, "/"+physical, nil, []byte(Mapping)); err != nil {
		return 0, gerror.Wrap(err, "create index")
	}
	count := 0
	var batch bytes.Buffer
	flush := func() error {
		if batch.Len() == 0 {
			return nil
		}
		raw, err := e.do(ctx, http.MethodPost, "/"+physical+"/_bulk", nil, batch.Bytes())
		batch.Reset()
		if err != nil {
			return err
		}
		var res struct {
			Errors bool `json:"errors"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		if res.Errors {
			return gerror.New("bulk load reported item errors")
		}
		return nil
	}
	err := load(func(doc *service.GoodsDoc) error {
		line, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		fmt.Fprintf(&batch, `{"index":{"_id":"%d"}}`+"\n%s\n", doc.GoodsID, line)
		count++
		if count%bulkBatch == 0 {
			return flush()
		}
		return nil
	})
	if err == nil {
		err = flush()
	}
	if err == nil {
		_, err = e.do(ctx, http.MethodPost, "/"+physical+"/_refresh", nil, nil)
	}
	if err != nil {
		_, _ = e.do(ctx, http.MethodDelete, "/"+physical, nil, nil)
		return 0, gerror.Wrap(err, "load index")
	}

	old, err := e.aliasTargets(ctx)
	if err != nil {
		return 0, err
	}
	actions := []any{map[string]any{"add": map[string]any{"index": physical, "alias": e.alias}}}
	for _, index := range old {
		actions = append(actions, map[string]any{"remove": map[string]any{"index": index, "alias": e.alias}})
	}
	body, _ := json.Marshal(map[string]any{"actions": actions})
	if _, err := e.do(ctx, http.MethodPost, "/_aliases", nil, body); err != nil {
		_, _ = e.do(ctx, http.MethodDelete, "/"+physical, nil, nil)
		return 0, gerror.Wrap(err, "switch alias")
	}
	for _, index := range old {
		_, _ = e.do(ctx, http.MethodDelete, "/"+index, nil, nil)
	}
	return count, nil
}

// aliasTargets lists the physical indices the alias points at now.
func (e *ES) aliasTargets(ctx context.Context) ([]string, error) {
	raw, err := e.do(ctx, http.MethodGet, "/_alias/"+e.alias, nil, nil)
	if isStatus(err, http.StatusNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var targets map[string]any
	if err := json.Unmarshal(raw, &targets); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(targets))
	for index := range targets {
		out = append(out, index)
	}
	return out, nil
}

func (e *ES) refresh() map[string]string {
	if e.Refresh == "" {
		return nil
	}
	return map[string]string{"refresh": e.Refresh}
}

type statusError struct {
	status int
	body   string
}

func (s *statusError) Error() string {
	return fmt.Sprintf("elasticsearch: HTTP %d: %s", s.status, s.body)
}

func isStatus(err error, status int) bool {
	se, ok := err.(*statusError)
	return ok && se.status == status
}

// do sends one request. Any non-2xx answer becomes a *statusError carrying a
// short excerpt of the body for the logs; a transport failure is
// service.ErrUnavailable.
func (e *ES) do(ctx context.Context, method, path string, query map[string]string, body []byte) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, reader)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		values := req.URL.Query()
		for k, v := range query {
			values.Set(k, v)
		}
		req.URL.RawQuery = values.Encode()
	}
	if body != nil {
		contentType := "application/json"
		if strings.HasSuffix(path, "/_bulk") {
			contentType = "application/x-ndjson"
		}
		req.Header.Set("Content-Type", contentType)
	}
	res, err := e.client.Perform(req)
	if err != nil {
		return nil, gerror.Wrapf(service.ErrUnavailable, "%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		excerpt := string(raw)
		if len(excerpt) > 300 {
			excerpt = excerpt[:300]
		}
		return nil, &statusError{status: res.StatusCode, body: excerpt}
	}
	return raw, nil
}
