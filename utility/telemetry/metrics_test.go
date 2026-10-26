package telemetry

import (
	"context"
	"sort"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc/codes"
)

// The metrics of this package are created once per process from the global
// provider, so every test shares one reader, installed before first use.
var testReader = func() *sdkmetric.ManualReader {
	r := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(r)))
	setIdentity("order-service", "experiment")
	return r
}()

func collect(t *testing.T) map[string]metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := testReader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	out := map[string]metricdata.Metrics{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			out[m.Name] = m
		}
	}
	return out
}

// allowedLabels is design §9.2: service and environment everywhere, plus
// the listed labels per metric, and nothing else.
var allowedLabels = map[string][]string{
	MetricOrderCreate:          {"outcome"},
	MetricOrderPendingPayment:  {},
	MetricWorkflowBacklog:      {"step"},
	MetricWorkflowOldestAge:    {"step"},
	MetricRPCDuration:          {"peer", "method", "outcome"},
	MetricPaymentCallbackDelay: {},
}

func keys(set attribute.Set) []string {
	var out []string
	for _, kv := range set.ToSlice() {
		out = append(out, string(kv.Key))
	}
	sort.Strings(out)
	return out
}

func checkLabels(t *testing.T, name string, set attribute.Set) {
	t.Helper()
	want := append([]string{"environment", "service"}, allowedLabels[name]...)
	sort.Strings(want)
	got := keys(set)
	if len(got) != len(want) {
		t.Fatalf("%s labels %v, want %v", name, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s labels %v, want %v", name, got, want)
		}
	}
	if v, _ := set.Value("service"); v.AsString() != "order-service" {
		t.Fatalf("%s service label %q", name, v.AsString())
	}
	if v, _ := set.Value("environment"); v.AsString() != "experiment" {
		t.Fatalf("%s environment label %q", name, v.AsString())
	}
}

func TestSynchronousMetrics(t *testing.T) {
	ctx := context.Background()
	RecordOrderCreate(ctx, "pending_payment")
	RecordOrderCreate(ctx, "pending_payment")
	RecordOrderCreate(ctx, "rejected")
	RecordRPC(ctx, "goods-service", "/inventory.v1.InventoryService/ReserveInventory", OutcomeOK, 30*time.Millisecond)
	RecordRPC(ctx, "goods-service", "/inventory.v1.InventoryService/ReserveInventory", OutcomeError, 3*time.Second)
	RecordPaymentCallbackDelay(ctx, 1500*time.Millisecond)
	RecordPaymentCallbackDelay(ctx, -time.Second) // provider clock ahead: counted as 0

	got := collect(t)
	creates := got[MetricOrderCreate].Data.(metricdata.Sum[int64])
	if !creates.IsMonotonic {
		t.Fatal("order create is not a counter")
	}
	byOutcome := map[string]int64{}
	for _, dp := range creates.DataPoints {
		checkLabels(t, MetricOrderCreate, dp.Attributes)
		v, _ := dp.Attributes.Value("outcome")
		byOutcome[v.AsString()] = dp.Value
	}
	if byOutcome["pending_payment"] != 2 || byOutcome["rejected"] != 1 {
		t.Fatalf("order create: %v", byOutcome)
	}

	rpc := got[MetricRPCDuration]
	if rpc.Unit != "s" {
		t.Fatalf("rpc unit %q", rpc.Unit)
	}
	hist := rpc.Data.(metricdata.Histogram[float64])
	if len(hist.DataPoints) != 2 {
		t.Fatalf("%d rpc series, want one per outcome", len(hist.DataPoints))
	}
	for _, dp := range hist.DataPoints {
		checkLabels(t, MetricRPCDuration, dp.Attributes)
		if dp.Bounds[0] != 0.005 || dp.Bounds[len(dp.Bounds)-1] != 10 {
			t.Fatalf("rpc buckets %v are not in seconds", dp.Bounds)
		}
	}

	delay := got[MetricPaymentCallbackDelay].Data.(metricdata.Histogram[float64])
	dp := delay.DataPoints[0]
	checkLabels(t, MetricPaymentCallbackDelay, dp.Attributes)
	if dp.Count != 2 || dp.Sum != 1.5 {
		t.Fatalf("callback delay count %d sum %v, want 2 and 1.5", dp.Count, dp.Sum)
	}
}

func TestObservedGauges(t *testing.T) {
	old := time.Now().Add(-45 * time.Second)
	if err := ObserveWorkflow(func(context.Context) (map[string]Backlog, error) {
		return map[string]Backlog{"reserve": {Count: 2, Oldest: old}, "close": {}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := ObservePendingPayment(func(context.Context) (int64, error) { return 7, nil }); err != nil {
		t.Fatal(err)
	}
	got := collect(t)

	pending := got[MetricOrderPendingPayment].Data.(metricdata.Gauge[int64]).DataPoints
	if len(pending) != 1 || pending[0].Value != 7 {
		t.Fatalf("pending payment %v", pending)
	}
	checkLabels(t, MetricOrderPendingPayment, pending[0].Attributes)

	counts := map[string]int64{}
	for _, dp := range got[MetricWorkflowBacklog].Data.(metricdata.Gauge[int64]).DataPoints {
		checkLabels(t, MetricWorkflowBacklog, dp.Attributes)
		v, _ := dp.Attributes.Value("step")
		counts[v.AsString()] = dp.Value
	}
	ages := map[string]float64{}
	for _, dp := range got[MetricWorkflowOldestAge].Data.(metricdata.Gauge[float64]).DataPoints {
		checkLabels(t, MetricWorkflowOldestAge, dp.Attributes)
		v, _ := dp.Attributes.Value("step")
		ages[v.AsString()] = dp.Value
	}
	if counts["reserve"] != 2 || counts["close"] != 0 {
		t.Fatalf("backlog %v", counts)
	}
	if ages["reserve"] < 44 || ages["reserve"] > 50 || ages["close"] != 0 {
		t.Fatalf("oldest age %v: want about 45s for reserve and 0 with nothing waiting", ages)
	}
}

func TestRPCOutcome(t *testing.T) {
	for code, want := range map[codes.Code]string{
		codes.OK:                 OutcomeOK,
		codes.InvalidArgument:    OutcomeRejected,
		codes.NotFound:           OutcomeRejected,
		codes.FailedPrecondition: OutcomeRejected,
		codes.Unauthenticated:    OutcomeRejected,
		codes.PermissionDenied:   OutcomeRejected,
		codes.AlreadyExists:      OutcomeRejected,
		codes.Unavailable:        OutcomeError,
		codes.DeadlineExceeded:   OutcomeError,
		codes.Internal:           OutcomeError,
		codes.Unknown:            OutcomeError,
		codes.ResourceExhausted:  OutcomeError,
		codes.Canceled:           OutcomeError,
	} {
		if got := RPCOutcome(code); got != want {
			t.Errorf("%s -> %s, want %s", code, got, want)
		}
	}
}

func TestPeerName(t *testing.T) {
	for target, want := range map[string]string{
		"service:///service/default/default/goods/latest/":  "goods-service",
		"service:///service/default/default/paysim/latest/": "payment-simulator",
		"goods":               "goods-service",
		"etcd:///goods":       "goods-service",
		"paysim":              "payment-simulator",
		"user":                "user-service",
		"127.0.0.1:31005":     "127.0.0.1",
		"order-service":       "order-service",
		"dns:///search:31006": "search-service",
	} {
		if got := PeerName(target); got != want {
			t.Errorf("%q -> %q, want %q", target, got, want)
		}
	}
}
