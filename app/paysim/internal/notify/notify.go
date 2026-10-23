// Package notify tells the merchant about paid payments, the way a payment
// provider calls a merchant's notify URL: a signed POST, repeated with
// backoff until the merchant acknowledges it or the attempts run out. The
// notification is only a hint; the merchant is expected to query the
// payment before acting, so a lost or repeated one does no harm.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

// Settings come from config (notify.*).
type Settings struct {
	URL         string
	Interval    time.Duration
	MaxAttempts int
}

type Notifier struct {
	payments service.Repository
	signer   *utility.NoticeSigner
	settings Settings
	client   *http.Client
	now      func() time.Time
}

func New(payments service.Repository, signer *utility.NoticeSigner, settings Settings, now func() time.Time) *Notifier {
	return &Notifier{
		payments: payments,
		signer:   signer,
		settings: settings,
		// No proxy: the merchant is on the local network, and a proxy set in
		// the environment for other reasons must not see these requests.
		client: &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}},
		now:    now,
	}
}

// Run sends due notifications every interval until ctx ends.
func (n *Notifier) Run(ctx context.Context) {
	ticker := time.NewTicker(n.settings.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.Pass(ctx)
		}
	}
}

// Pass sends every notification that is due now.
func (n *Notifier) Pass(ctx context.Context) {
	due, err := n.payments.DueNotifications(ctx, n.now(), 50)
	if err != nil {
		g.Log().Warningf(ctx, "load due notifications: %v", err)
		return
	}
	for _, p := range due {
		n.notifyOne(ctx, p)
	}
}

// notifyOne sends one notification in its own trace. The request carries
// the trace context, so gateway-h5 and the order service continue it: one
// trace shows the delivery, the merchant's query back and its decision.
func (n *Notifier) notifyOne(ctx context.Context, p *service.Payment) {
	ctx, span := telemetry.Root(ctx, "paysim.notify",
		attribute.String("payment.id", p.ID), attribute.Int("notify.attempt", p.NotifyAttempts+1))
	defer span.End()
	attempts := p.NotifyAttempts + 1
	var next *time.Time
	if err := n.send(ctx, p); err != nil {
		span.SetStatus(codes.Error, err.Error())
		if attempts < n.settings.MaxAttempts {
			t := n.now().Add(backoff(attempts))
			next = &t
			g.Log().Warningf(ctx, "notify payment %s (attempt %d) failed, retry at %s: %v",
				p.ID, attempts, t.Format(time.RFC3339), err)
		} else {
			g.Log().Errorf(ctx, "notify payment %s: giving up after %d attempts: %v; the merchant must query", p.ID, attempts, err)
		}
	} else {
		g.Log().Infof(ctx, "notify payment %s (attempt %d): acknowledged", p.ID, attempts)
	}
	if err := n.payments.Notified(ctx, p.ID, attempts, next); err != nil {
		g.Log().Warningf(ctx, "record notification of %s: %v", p.ID, err)
	}
}

// send posts one notification. The merchant acknowledges with HTTP 200 and
// the gateway envelope {"code":0}.
func (n *Notifier) send(ctx context.Context, p *service.Payment) error {
	body, err := json.Marshal(utility.PaymentNotice{
		PaymentID: p.ID, OrderNo: p.OrderNo, State: p.State, NotifyTime: n.now().UTC(),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.settings.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(utility.PaySignatureHeader, n.signer.Sign(body))
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	res, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var ack struct {
		Code *int `json:"code"`
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode != http.StatusOK || json.Unmarshal(raw, &ack) != nil || ack.Code == nil || *ack.Code != 0 {
		return &rejected{status: res.StatusCode, code: ack.Code}
	}
	return nil
}

type rejected struct {
	status int
	code   *int
}

func (r *rejected) Error() string {
	if r.code == nil {
		return "merchant answered HTTP " + http.StatusText(r.status)
	}
	return "merchant refused with code " + strconv.Itoa(*r.code)
}

// backoff: 2s, 4s, 8s ... capped at 5 minutes.
func backoff(attempt int) time.Duration {
	d := time.Second << attempt
	if attempt > 8 || d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
}
