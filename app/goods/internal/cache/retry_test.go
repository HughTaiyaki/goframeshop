package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/goods/internal/service"
)

// flaky fails the first `failures` deletes.
type flaky struct {
	service.Cache
	failures int32
	calls    atomic.Int32
}

func (f *flaky) DeleteGoods(context.Context, uint64) error {
	if f.calls.Add(1) <= f.failures {
		return errors.New("redis down")
	}
	return nil
}

func (f *flaky) DeleteCategories(ctx context.Context) error { return f.DeleteGoods(ctx, 0) }

func TestFailedInvalidationIsRetriedInTheBackground(t *testing.T) {
	inner := &flaky{Cache: Noop{}, failures: 3}
	c := WithRetry(inner, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond})

	// The first failure is reported to the caller, who logs it.
	ctx, cancel := context.WithCancel(context.Background())
	if err := c.DeleteGoods(ctx, 1); err == nil {
		t.Fatal("first failure was swallowed")
	}
	// The request finishing must not stop the retries.
	cancel()
	deadline := time.Now().Add(time.Second)
	for inner.calls.Load() < 4 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := inner.calls.Load(); got != 4 {
		t.Fatalf("delete attempted %d times, want 1 + 3 retries", got)
	}
}

func TestSuccessfulInvalidationIsNotRetried(t *testing.T) {
	inner := &flaky{Cache: Noop{}}
	c := WithRetry(inner, []time.Duration{time.Millisecond})
	if err := c.DeleteCategories(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if got := inner.calls.Load(); got != 1 {
		t.Fatalf("delete attempted %d times, want 1", got)
	}
}
