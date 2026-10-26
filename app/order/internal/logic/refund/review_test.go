package refund

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry/logcapture"
)

func TestApprovePaysTheLineBack(t *testing.T) {
	ctx := context.Background()
	svc, _, _, provider := setupAll()
	r, _ := svc.Create(ctx, alice, "PAID-1", 10, "坏了")

	got, err := svc.Approve(ctx, r.No, " 同意 ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != service.RefundRefunded || got.ReviewNote != "同意" || got.ReviewedAt == nil || got.RefundedAt == nil {
		t.Fatalf("approved: %+v", got)
	}
	// The provider was asked for exactly this line, under the fixed id.
	if provider.refunds[service.AfterSaleRefundID(r.No)] != 39800 || len(provider.refunds) != 1 {
		t.Fatalf("provider refunds %v", provider.refunds)
	}
	// Approving again changes nothing and pays nothing more.
	if again, err := svc.Approve(ctx, r.No, "again"); err != nil || again.Status != service.RefundRefunded || provider.calls != 1 {
		t.Fatalf("approve again: %+v %v calls=%d", again, err, provider.calls)
	}
	if _, err := svc.Reject(ctx, r.No, "no"); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("reject a refunded request: %v", err)
	}
	if _, err := svc.Approve(ctx, "nope", ""); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestApprovedWithoutAnAnswerIsRetried(t *testing.T) {
	ctx := context.Background()
	svc, _, refunds, provider := setupAll()
	r, _ := svc.Create(ctx, alice, "PAID-1", 11, "少了")
	provider.fail = true

	got, err := svc.Approve(ctx, r.No, "")
	if err != nil {
		t.Fatalf("approve is the administrator's decision, not the provider's: %v", err)
	}
	if got.Status != service.RefundApproved || got.Attempts != 1 {
		t.Fatalf("after a failed refund: %+v", got)
	}
	if err := svc.Retry(ctx); err != nil {
		t.Fatal(err)
	}
	if row, _ := refunds.FindByNo(ctx, r.No); row.Status != service.RefundApproved || row.Attempts != 2 {
		t.Fatalf("after a failed retry: %+v", row)
	}

	provider.fail = false
	if err := svc.Retry(ctx); err != nil {
		t.Fatal(err)
	}
	row, _ := refunds.FindByNo(ctx, r.No)
	if row.Status != service.RefundRefunded || provider.refunds[service.AfterSaleRefundID(r.No)] != 990 {
		t.Fatalf("after the provider came back: %+v %v", row, provider.refunds)
	}
	// Nothing left to retry.
	before := provider.calls
	if err := svc.Retry(ctx); err != nil || provider.calls != before {
		t.Fatalf("retried a refunded request: calls %d -> %d", before, provider.calls)
	}
}

func TestRejectNeedsAReasonAndIsFinal(t *testing.T) {
	ctx := context.Background()
	svc, _, _, provider := setupAll()
	r, _ := svc.Create(ctx, alice, "PAID-1", 10, "不想要了")

	for _, note := range []string{"", "   ", strings.Repeat("否", 256)} {
		if _, err := svc.Reject(ctx, r.No, note); !errors.Is(err, service.ErrInvalidArgument) {
			t.Fatalf("note %q: err = %v", note, err)
		}
	}
	got, err := svc.Reject(ctx, r.No, " 已超过七天 ")
	if err != nil || got.Status != service.RefundRejected || got.ReviewNote != "已超过七天" || got.ReviewedAt == nil {
		t.Fatalf("rejected: %+v %v", got, err)
	}
	if again, err := svc.Reject(ctx, r.No, "again"); err != nil || again.ReviewNote != "已超过七天" {
		t.Fatalf("reject again: %+v %v", again, err)
	}
	if _, err := svc.Approve(ctx, r.No, ""); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("approve a rejected request: %v", err)
	}
	if provider.calls != 0 {
		t.Fatal("a rejected request reached the provider")
	}
}

func TestApproveAndRejectRace(t *testing.T) {
	ctx := context.Background()
	outcomes := map[string]int{}
	for run := 0; run < 50; run++ {
		svc, _, refunds, provider := setupAll()
		r, _ := svc.Create(ctx, alice, "PAID-1", 10, "坏了")
		var wg sync.WaitGroup
		var approveErr, rejectErr error
		wg.Add(2)
		go func() { defer wg.Done(); _, approveErr = svc.Approve(ctx, r.No, "") }()
		go func() { defer wg.Done(); _, rejectErr = svc.Reject(ctx, r.No, "no") }()
		wg.Wait()
		row, _ := refunds.FindByNo(ctx, r.No)
		switch row.Status {
		case service.RefundRefunded:
			if approveErr != nil || !errors.Is(rejectErr, service.ErrFailedPrecondition) || provider.calls != 1 {
				t.Fatalf("run %d: refunded, approve=%v reject=%v calls=%d", run, approveErr, rejectErr, provider.calls)
			}
		case service.RefundRejected:
			if rejectErr != nil || !errors.Is(approveErr, service.ErrFailedPrecondition) || provider.calls != 0 {
				t.Fatalf("run %d: rejected, approve=%v reject=%v calls=%d", run, approveErr, rejectErr, provider.calls)
			}
		default:
			t.Fatalf("run %d: ended %s", run, row.Status)
		}
		outcomes[row.Status]++
	}
	t.Logf("outcomes: %v", outcomes)
}

// After-sale refunds are logged as payment.refund.requested, failures with
// their error type, and the worker's attempts as workflow.retry.
func TestAfterSaleRefundsAreLogged(t *testing.T) {
	logs := logcapture.Start(t)
	ctx := context.Background()
	svc, _, _, provider := setupAll()
	r, _ := svc.Create(ctx, alice, "PAID-1", 10, "坏了")
	provider.fail = true
	svc.Approve(ctx, r.No, "")
	provider.fail = false
	svc.Retry(ctx)

	requests := logs.Events(telemetry.EventPaymentRefundRequest)
	if len(requests) != 2 {
		t.Fatalf("%d refund events, want 2: %v", len(requests), requests)
	}
	failed, done := requests[0], requests[1]
	if failed["order_id"] != "PAID-1" || failed["error.type"] == "" || failed.Attr("outcome") != "unknown" ||
		failed.Attr("kind") != "after_sale" || failed.Attr("refund_id") != service.AfterSaleRefundID(r.No) || failed.Attr("attempt") != float64(1) {
		t.Fatalf("failed attempt: %v", failed)
	}
	if done["error.type"] != "" || done.Attr("outcome") != "succeeded" || done.Attr("attempt") != float64(2) {
		t.Fatalf("second attempt: %v", done)
	}
	retries := logs.Events(telemetry.EventWorkflowRetry)
	if len(retries) != 1 || retries[0].Attr("step") != "after_sale_refund" || retries[0]["order_id"] != "PAID-1" {
		t.Fatalf("workflow.retry: %v", retries)
	}
}
