// Package source reads goods from the goods database for indexing.
//
// The search index is a read model of data owned by the goods service, so
// this package reads goods tables directly, through a read-only account
// (search_sync). That is a deliberate coupling: when the goods schema
// changes, update docQuery below. The query names its columns, so a removed or
// renamed column fails loudly instead of shifting values into the wrong
// fields, which is what positional binlog parsing would do.
package source

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
)

// docQuery selects exactly the goods that belong in the index: on shelf
// (status 1) and not deleted. Stock and sale come from the inventory ledger,
// the cover from the first image by sort order.
const docQuery = `
SELECT g.id, g.name, g.tags, g.brand, g.price_cents,
       g.level1_category_id, g.level2_category_id, g.level3_category_id, g.created_at,
       COALESCE(i.available, 0) AS stock, COALESCE(i.sold, 0) AS sale,
       COALESCE((SELECT gi.url FROM goods_images gi WHERE gi.goods_id = g.id
                 ORDER BY gi.sort, gi.id LIMIT 1), '') AS cover_url
FROM goods_info g
LEFT JOIN inventory i ON i.goods_id = g.id
WHERE g.status = 1 AND g.deleted_at IS NULL`

const pageSize = 500

// MySQL reads through a GoFrame database group (database.goods_source).
type MySQL struct {
	db gdb.DB
}

func New(db gdb.DB) *MySQL {
	return &MySQL{db: db}
}

func (m *MySQL) Load(ctx context.Context, goodsID uint64) (*service.GoodsDoc, error) {
	records, err := m.db.GetAll(ctx, docQuery+" AND g.id = ?", goodsID)
	if err != nil {
		return nil, err
	}
	if records.IsEmpty() {
		return nil, nil
	}
	return toDoc(records[0])
}

// LoadAll pages by primary key (keyset pagination): unlike OFFSET, every page
// costs the same however deep it is.
func (m *MySQL) LoadAll(ctx context.Context, emit func(*service.GoodsDoc) error) error {
	var after uint64
	for {
		records, err := m.db.GetAll(ctx, docQuery+" AND g.id > ? ORDER BY g.id LIMIT ?", after, pageSize)
		if err != nil {
			return err
		}
		for _, record := range records {
			doc, err := toDoc(record)
			if err != nil {
				return err
			}
			if err := emit(doc); err != nil {
				return err
			}
			after = doc.GoodsID
		}
		if len(records) < pageSize {
			return nil
		}
	}
}

func toDoc(record gdb.Record) (*service.GoodsDoc, error) {
	doc := &service.GoodsDoc{
		GoodsID:          record["id"].Uint64(),
		Name:             record["name"].String(),
		Tags:             record["tags"].String(),
		Brand:            record["brand"].String(),
		PriceCents:       record["price_cents"].Uint64(),
		Level1CategoryID: record["level1_category_id"].Uint64(),
		Level2CategoryID: record["level2_category_id"].Uint64(),
		Level3CategoryID: record["level3_category_id"].Uint64(),
		Stock:            record["stock"].Uint32(),
		Sale:             record["sale"].Uint32(),
		CoverURL:         record["cover_url"].String(),
	}
	if t := record["created_at"].GTime(); t != nil {
		doc.CreatedAt = t.Time
	}
	return doc, nil
}
