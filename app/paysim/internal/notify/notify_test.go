package notify

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/fake"
	providerlogic "github.com/hughtaiyaki/goframeshop/app/paysim/internal/logic/provider"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

func TestNotifierSignsRetriesAndStops(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := utility.NewNoticeSigner(private.Seed())
	verifier, _ := utility.NewNoticeVerifier(public)

	var calls, failFirst atomic.Int32
	failFirst.Store(2)
	merchant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if !verifier.Verify(body, r.Header.Get(utility.PaySignatureHeader)) {
			t.Errorf("bad signature on %s", body)
		}
		var notice utility.PaymentNotice
		if err := json.Unmarshal(body, &notice); err != nil || notice.PaymentID != "P1" || notice.State != service.StateSucceeded {
			t.Errorf("notice %+v %v", notice, err)
		}
		if failFirst.Add(-1) >= 0 {
			w.Write([]byte(`{"code":14,"message":"Unavailable"}`))
			return
		}
		w.Write([]byte(`{"code":0}`))
	}))
	defer merchant.Close()

	now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	repo := fake.New()
	provider := providerlogic.New(repo, clock)
	ctx := context.Background()
	provider.Create(ctx, &service.CreateInput{PaymentID: "P1", OrderNo: "O1", AmountCents: 100, Currency: "CNY", ExpiresAt: now.Add(time.Hour)})
	provider.Pay(ctx, "P1", true)

	n := New(repo, signer, Settings{URL: merchant.URL, MaxAttempts: 10}, clock)
	n.Pass(ctx) // refused: retry in 2s
	p, _ := repo.Find(ctx, "P1")
	if p.NotifyAttempts != 1 || p.NextNotifyAt == nil || !p.NextNotifyAt.Equal(now.Add(2*time.Second)) {
		t.Fatalf("after a refusal: %+v", p)
	}
	n.Pass(ctx) // not due yet
	if calls.Load() != 1 {
		t.Fatalf("sent before it was due: %d calls", calls.Load())
	}
	now = now.Add(2 * time.Second)
	n.Pass(ctx) // refused again: retry in 4s
	now = now.Add(4 * time.Second)
	n.Pass(ctx) // acknowledged
	p, _ = repo.Find(ctx, "P1")
	if calls.Load() != 3 || p.NextNotifyAt != nil || p.NotifyAttempts != 3 {
		t.Fatalf("after the ack: %d calls, %+v", calls.Load(), p)
	}
	now = now.Add(time.Hour)
	n.Pass(ctx)
	if calls.Load() != 3 {
		t.Fatalf("notified again after the ack")
	}
}

func TestNotifierGivesUp(t *testing.T) {
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := utility.NewNoticeSigner(private.Seed())
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer down.Close()

	now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	repo := fake.New()
	provider := providerlogic.New(repo, clock)
	ctx := context.Background()
	provider.Create(ctx, &service.CreateInput{PaymentID: "P1", OrderNo: "O1", AmountCents: 100, Currency: "CNY", ExpiresAt: now.Add(time.Hour)})
	provider.Pay(ctx, "P1", true)

	n := New(repo, signer, Settings{URL: down.URL, MaxAttempts: 3}, clock)
	for i := 0; i < 5; i++ {
		n.Pass(ctx)
		now = now.Add(10 * time.Minute)
	}
	p, _ := repo.Find(ctx, "P1")
	if p.NotifyAttempts != 3 || p.NextNotifyAt != nil {
		t.Fatalf("did not give up after 3 attempts: %+v", p)
	}
}

func TestBackoff(t *testing.T) {
	for attempt, want := range map[int]time.Duration{1: 2 * time.Second, 2: 4 * time.Second, 8: 256 * time.Second, 9: 5 * time.Minute, 40: 5 * time.Minute} {
		if got := backoff(attempt); got != want {
			t.Errorf("backoff(%d) = %s, want %s", attempt, got, want)
		}
	}
}
