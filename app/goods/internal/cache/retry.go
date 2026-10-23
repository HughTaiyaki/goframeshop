package cache

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

// DefaultRetryDelays spread the retries over about half a minute: long enough
// to ride out a Redis restart or a network blip, short enough that a stale
// entry does not live for its whole TTL.
var DefaultRetryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

// retrying adds background retries to the two invalidations. Reads and fills
// are not retried: a failed read just falls back to MySQL, and a failed fill
// only costs a later miss. A failed delete is different: it leaves stale data
// that would be served until the TTL expires.
type retrying struct {
	service.Cache
	delays []time.Duration
}

// WithRetry wraps c. The first failure is still returned, so the caller logs
// it; the retries happen in the background and log their own outcome.
func WithRetry(c service.Cache, delays []time.Duration) service.Cache {
	return &retrying{Cache: c, delays: delays}
}

func (r *retrying) DeleteGoods(ctx context.Context, goodsID uint64) error {
	err := r.Cache.DeleteGoods(ctx, goodsID)
	if err != nil {
		r.retry(ctx, func(ctx context.Context) error { return r.Cache.DeleteGoods(ctx, goodsID) })
	}
	return err
}

func (r *retrying) DeleteCategories(ctx context.Context) error {
	err := r.Cache.DeleteCategories(ctx)
	if err != nil {
		r.retry(ctx, r.Cache.DeleteCategories)
	}
	return err
}

// retry runs after the request has returned, so it detaches from the request's
// cancellation but keeps its values (trace id) for the logs. The retries live
// in this process only: if it exits first, the entry stays stale until its
// TTL. Closing that gap needs a durable outbox, which is out of scope here.
func (r *retrying) retry(ctx context.Context, del func(context.Context) error) {
	ctx = context.WithoutCancel(ctx)
	go func() {
		for i, delay := range r.delays {
			time.Sleep(delay)
			if err := del(ctx); err == nil {
				g.Log().Infof(ctx, "cache invalidation succeeded on retry %d", i+1)
				return
			}
		}
		g.Log().Errorf(ctx, "cache invalidation failed after %d retries; entry stays stale until its TTL", len(r.delays))
	}()
}
