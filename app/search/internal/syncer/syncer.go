// Package syncer keeps the search index in step with the goods database.
//
// Start does three things, in this order:
//
//  1. Note the current binlog position.
//  2. Rebuild the whole index from MySQL (Index.Rebuild, alias swap).
//  3. Follow the binlog from the noted position.
//
// Every change made while step 2 ran is therefore replayed in step 3. That is
// safe because a binlog event is used only as a signal, "goods N changed":
// the syncer reloads goods N's current state from MySQL and writes that, so
// replaying, reordering or duplicating events all converge on the same
// document. Nothing about the row's column layout is parsed from the event
// beyond finding the id column by name.
//
// Nothing is persisted between runs: every start rebuilds the index. That is
// cheap at this catalogue's size; for a large catalogue, store the binlog
// position after each processed event instead and rebuild only on demand.
package syncer

import (
	"context"
	"time"

	"github.com/go-mysql-org/go-mysql/canal"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

// idColumns maps each watched table to the column that names the goods.
// Any change in one of them can change a search document: the product itself,
// its stock and sales, or its cover image.
var idColumns = map[string]string{
	"goods_info":   "id",
	"inventory":    "goods_id",
	"goods_images": "goods_id",
}

// Config is where to read the binlog.
type Config struct {
	Addr     string
	User     string
	Password string
	// ServerID must be unique among everything replicating from this MySQL,
	// including real replicas.
	ServerID uint32
	Database string
}

type Syncer struct {
	index  service.Index
	source service.Source
	config Config
	// backoff bounds the wait between attempts to apply one change.
	minBackoff, maxBackoff time.Duration
}

func New(index service.Index, source service.Source, config Config) *Syncer {
	return &Syncer{index: index, source: source, config: config, minBackoff: 100 * time.Millisecond, maxBackoff: 30 * time.Second}
}

// Start rebuilds the index and then follows the binlog in the background
// until ctx ends. It returns once the rebuild is done, so the service only
// starts answering searches with a complete index.
func (s *Syncer) Start(ctx context.Context) error {
	cfg := canal.NewDefaultConfig()
	cfg.Addr = s.config.Addr
	cfg.User = s.config.User
	cfg.Password = s.config.Password
	cfg.ServerID = s.config.ServerID
	cfg.Flavor = "mysql"
	cfg.Dump.ExecutionPath = "" // no mysqldump: Rebuild loads the snapshot
	cfg.IncludeTableRegex = make([]string, 0, len(idColumns))
	for table := range idColumns {
		cfg.IncludeTableRegex = append(cfg.IncludeTableRegex, s.config.Database+`\.`+table+`$`)
	}
	c, err := canal.NewCanal(cfg)
	if err != nil {
		return gerror.Wrap(err, "connect to the binlog")
	}

	pos, err := c.GetMasterPos()
	if err != nil {
		c.Close()
		return gerror.Wrap(err, "read the binlog position")
	}
	count, err := s.index.Rebuild(ctx, func(emit func(*service.GoodsDoc) error) error {
		return s.source.LoadAll(ctx, emit)
	})
	if err != nil {
		c.Close()
		return gerror.Wrap(err, "rebuild the search index")
	}
	g.Log().Infof(ctx, "search index rebuilt with %d goods; following the binlog from %s", count, pos)

	c.SetEventHandler(&handler{ctx: ctx, syncer: s})
	go func() {
		<-ctx.Done()
		c.Close()
	}()
	go func() {
		if err := c.RunFrom(pos); err != nil && ctx.Err() == nil {
			// The index stops following MySQL until the service restarts,
			// which rebuilds it. Loud, because searches now go stale.
			g.Log().Errorf(ctx, "binlog sync stopped; the search index is no longer updated: %v", err)
		}
	}()
	return nil
}

// Apply brings one product's document in line with MySQL: upsert it if it
// belongs in the index, delete it otherwise. It retries with backoff until it
// succeeds or ctx ends, so an Elasticsearch outage pauses the sync instead of
// dropping changes.
func (s *Syncer) Apply(ctx context.Context, goodsID uint64) {
	wait := s.minBackoff
	for attempt := 1; ; attempt++ {
		err := s.applyOnce(ctx, goodsID)
		if err == nil {
			return
		}
		g.Log().Warningf(ctx, "search sync of goods %d failed (attempt %d), retrying in %s: %v", goodsID, attempt, wait, err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, s.maxBackoff)
	}
}

func (s *Syncer) applyOnce(ctx context.Context, goodsID uint64) error {
	doc, err := s.source.Load(ctx, goodsID)
	if err != nil {
		return err
	}
	if doc == nil {
		return s.index.Delete(ctx, goodsID)
	}
	return s.index.Upsert(ctx, doc)
}

// GoodsIDs returns the distinct goods ids a row event touches, in first-seen
// order. An UPDATE event carries before/after pairs; both are read, so a row
// whose goods id itself changed updates both documents.
func GoodsIDs(e *canal.RowsEvent) []uint64 {
	column, ok := idColumns[e.Table.Name]
	if !ok {
		return nil
	}
	index := e.Table.FindColumn(column)
	if index < 0 {
		return nil
	}
	seen := map[uint64]bool{}
	var ids []uint64
	for _, row := range e.Rows {
		if index >= len(row) {
			continue
		}
		id := gconv.Uint64(row[index])
		if id != 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

type handler struct {
	canal.DummyEventHandler
	ctx    context.Context
	syncer *Syncer
}

// OnRow never returns an error: canal stops the whole stream on one. Failures
// are retried inside Apply instead.
func (h *handler) OnRow(e *canal.RowsEvent) error {
	for _, id := range GoodsIDs(e) {
		h.syncer.Apply(h.ctx, id)
	}
	return nil
}

func (h *handler) String() string { return "search-syncer" }
