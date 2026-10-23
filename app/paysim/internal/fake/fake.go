// Package fake is an in-memory service.Repository for tests. One mutex
// stands in for the row lock, which is what makes Update's decisions serial.
package fake

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
)

type Repository struct {
	mu       sync.Mutex
	payments map[string]service.Payment
	refunds  map[string]service.Refund // by payment id
}

func New() *Repository {
	return &Repository{payments: map[string]service.Payment{}, refunds: map[string]service.Refund{}}
}

func (r *Repository) Update(_ context.Context, paymentID string, decide service.Decide) (*service.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var current *service.Payment
	if p, ok := r.payments[paymentID]; ok {
		current = &p
	}
	var refund *service.Refund
	if x, ok := r.refunds[paymentID]; ok {
		refund = &x
	}
	write, err := decide(current, refund)
	if err != nil {
		return nil, err
	}
	if write.Refund != nil {
		for _, existing := range r.refunds {
			if existing.ID == write.Refund.ID {
				return nil, service.ErrConflict
			}
		}
		r.refunds[paymentID] = *write.Refund
	}
	if write.Payment == nil {
		return current, nil
	}
	r.payments[paymentID] = *write.Payment
	saved := *write.Payment
	return &saved, nil
}

func (r *Repository) Find(_ context.Context, paymentID string) (*service.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.payments[paymentID]; ok {
		return &p, nil
	}
	return nil, nil
}

func (r *Repository) DueNotifications(_ context.Context, now time.Time, limit int) ([]*service.Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*service.Payment
	for _, p := range r.payments {
		if p.NextNotifyAt != nil && !p.NextNotifyAt.After(now) {
			p := p
			out = append(out, &p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Repository) Notified(_ context.Context, paymentID string, attempts int, next *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.payments[paymentID]
	p.NotifyAttempts, p.NextNotifyAt = attempts, next
	r.payments[paymentID] = p
	return nil
}
