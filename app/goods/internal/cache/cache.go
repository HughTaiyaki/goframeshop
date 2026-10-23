// Package cache implements service.Cache with Redis.
//
// Keys carry a version so a change to the cached shape can be rolled out by
// bumping it, instead of decoding old entries into a new struct. The goods
// detail is at v2 since it gained sort (task 0015); v1 entries simply expire.
package cache

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

const (
	goodsKeyPrefix = "goods:v2:detail:"
	categoriesKey  = "goods:v1:category:all"
	// absentMarker is stored for an id that does not exist, so repeated
	// lookups of missing ids (or a scan of random ids) stop at Redis.
	absentMarker = "__absent__"
)

// Settings are the cache lifetimes. The jitter spreads expiry of entries that
// were filled at the same moment, so they do not all fall back to MySQL in the
// same second.
type Settings struct {
	GoodsTTL      time.Duration
	AbsentTTL     time.Duration
	CategoriesTTL time.Duration
	Jitter        float64 // fraction of the TTL, e.g. 0.1 for ±10%
	OpTimeout     time.Duration
}

// DefaultSettings: product pages tolerate an hour of staleness only if an
// invalidation was lost; normally every write deletes the entry at once.
var DefaultSettings = Settings{
	GoodsTTL:      time.Hour,
	AbsentTTL:     time.Minute,
	CategoriesTTL: 24 * time.Hour,
	Jitter:        0.1,
	OpTimeout:     100 * time.Millisecond,
}

// Redis is the Redis-backed cache.
type Redis struct {
	redis    *gredis.Redis
	settings Settings
}

func NewRedis(redis *gredis.Redis, settings Settings) *Redis {
	return &Redis{redis: redis, settings: settings}
}

// FromConfig connects to the redis.default config group. Without that group
// the service runs uncached (Noop). With it, an unreachable Redis is reported
// but not fatal: every cache call fails fast and reads go to MySQL.
func FromConfig(ctx context.Context) (service.Cache, error) {
	section, err := g.Cfg().Get(ctx, "redis.default")
	if err != nil {
		return nil, gerror.Wrap(err, "read redis config")
	}
	if section.IsEmpty() {
		g.Log().Warning(ctx, "redis.default is not configured; goods are served without a cache")
		return Noop{}, nil
	}
	var config gredis.Config
	if err := section.Scan(&config); err != nil {
		return nil, gerror.Wrap(err, "parse redis config")
	}
	redis, err := gredis.New(&config)
	if err != nil {
		return nil, gerror.Wrap(err, "create redis client")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := redis.Do(pingCtx, "PING"); err != nil {
		g.Log().Warningf(ctx, "redis is unreachable, reads fall back to MySQL until it returns: %v", err)
	}
	return WithRetry(NewRedis(redis, DefaultSettings), DefaultRetryDelays), nil
}

func goodsKey(goodsID uint64) string {
	return goodsKeyPrefix + strconv.FormatUint(goodsID, 10)
}

func (r *Redis) GetGoods(ctx context.Context, goodsID uint64) (*service.Goods, bool, error) {
	raw, found, err := r.get(ctx, goodsKey(goodsID))
	if err != nil || !found {
		return nil, false, err
	}
	if raw == absentMarker {
		return nil, true, nil
	}
	var goods service.Goods
	if err := json.Unmarshal([]byte(raw), &goods); err != nil {
		// A corrupt entry is a miss; the reload overwrites it.
		return nil, false, gerror.Wrap(err, "decode cached goods")
	}
	return &goods, true, nil
}

func (r *Redis) SetGoods(ctx context.Context, goods *service.Goods) error {
	data, err := json.Marshal(goods)
	if err != nil {
		return err
	}
	return r.set(ctx, goodsKey(goods.ID), string(data), r.settings.GoodsTTL)
}

func (r *Redis) SetGoodsAbsent(ctx context.Context, goodsID uint64) error {
	return r.set(ctx, goodsKey(goodsID), absentMarker, r.settings.AbsentTTL)
}

func (r *Redis) DeleteGoods(ctx context.Context, goodsID uint64) error {
	return r.del(ctx, goodsKey(goodsID))
}

func (r *Redis) GetCategories(ctx context.Context) ([]*service.Category, bool, error) {
	raw, found, err := r.get(ctx, categoriesKey)
	if err != nil || !found {
		return nil, false, err
	}
	var categories []*service.Category
	if err := json.Unmarshal([]byte(raw), &categories); err != nil {
		return nil, false, gerror.Wrap(err, "decode cached categories")
	}
	return categories, true, nil
}

func (r *Redis) SetCategories(ctx context.Context, categories []*service.Category) error {
	data, err := json.Marshal(categories)
	if err != nil {
		return err
	}
	return r.set(ctx, categoriesKey, string(data), r.settings.CategoriesTTL)
}

func (r *Redis) DeleteCategories(ctx context.Context) error {
	return r.del(ctx, categoriesKey)
}

// Every Redis call gets its own short deadline: a slow cache must cost a few
// milliseconds, not the request's whole time budget.

func (r *Redis) get(ctx context.Context, key string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.settings.OpTimeout)
	defer cancel()
	value, err := r.redis.Do(ctx, "GET", key)
	if err != nil {
		return "", false, opError("GET", key, err)
	}
	if value.IsNil() {
		return "", false, nil
	}
	return value.String(), true, nil
}

func (r *Redis) set(ctx context.Context, key, value string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, r.settings.OpTimeout)
	defer cancel()
	_, err := r.redis.Do(ctx, "SET", key, value, "PX", r.jittered(ttl).Milliseconds())
	return opError("SET", key, err)
}

func (r *Redis) del(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, r.settings.OpTimeout)
	defer cancel()
	_, err := r.redis.Do(ctx, "DEL", key)
	return opError("DEL", key, err)
}

// opError keeps the operation, the key and the root cause (for example
// "connection refused"), and drops the rest. GoFrame's own message repeats
// every command argument, which for SET is the whole cached value: that would
// copy cached data into the logs.
func opError(op, key string, err error) error {
	if err == nil {
		return nil
	}
	return gerror.Newf("redis %s %s: %v", op, key, gerror.Cause(err))
}

func (r *Redis) jittered(ttl time.Duration) time.Duration {
	if r.settings.Jitter <= 0 {
		return ttl
	}
	spread := float64(ttl) * r.settings.Jitter
	return ttl + time.Duration((rand.Float64()*2-1)*spread)
}

// Noop is the cache used when Redis is not configured: always a miss.
type Noop struct{}

func (Noop) GetGoods(context.Context, uint64) (*service.Goods, bool, error)   { return nil, false, nil }
func (Noop) SetGoods(context.Context, *service.Goods) error                   { return nil }
func (Noop) SetGoodsAbsent(context.Context, uint64) error                     { return nil }
func (Noop) DeleteGoods(context.Context, uint64) error                        { return nil }
func (Noop) GetCategories(context.Context) ([]*service.Category, bool, error) { return nil, false, nil }
func (Noop) SetCategories(context.Context, []*service.Category) error         { return nil }
func (Noop) DeleteCategories(context.Context) error                           { return nil }
