package telemetry

// Metrics (design §9.2, task 0020). Instruments are recorded with the
// OpenTelemetry metric API and pushed over OTLP every 5 seconds to the
// collector, which exposes them to Prometheus under exactly these names.
//
// Every data point carries "service" and "environment"; the other labels
// are the ones the contract allows, all with a small fixed set of values.
// Order, user, trace and goods ids are never labels.

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/codes"
)

// Metric names, exactly as in design §9.2. The collector is configured not
// to add suffixes, so these are also the Prometheus names.
const (
	MetricOrderCreate          = "trade_order_create_total"
	MetricOrderPendingPayment  = "trade_order_pending_payment"
	MetricWorkflowBacklog      = "trade_workflow_backlog"
	MetricWorkflowOldestAge    = "trade_workflow_oldest_age_seconds"
	MetricRPCDuration          = "trade_rpc_duration_seconds"
	MetricPaymentCallbackDelay = "trade_payment_callback_delay_seconds"
	MetricDBPoolInUse          = "trade_db_pool_in_use"
	MetricDBPoolLimit          = "trade_db_pool_limit"
	MetricDBAcquireTimeout     = "trade_db_acquire_timeout_total"
	MetricDBAcquireWait        = "trade_db_acquire_wait_seconds_total"
)

// RPC outcomes: the "outcome" label of MetricRPCDuration.
const (
	OutcomeOK       = "ok"
	OutcomeRejected = "rejected" // the callee said no: a business answer
	OutcomeError    = "error"    // the call failed: a technical error
)

// MetricsInterval is how often metrics are pushed (design §9.2 scrapes
// every 5 seconds).
const MetricsInterval = 5 * time.Second

var (
	rpcBuckets      = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	callbackBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300}
)

// identity of this process for metric labels; set by Start.
var identity struct {
	sync.RWMutex
	service, environment string
}

func setIdentity(service, environment string) {
	identity.Lock()
	defer identity.Unlock()
	identity.service, identity.environment = service, environment
}

// base returns the labels every data point carries plus extra.
func base(extra ...attribute.KeyValue) metric.MeasurementOption {
	identity.RLock()
	attrs := append([]attribute.KeyValue{
		attribute.String("service", identity.service),
		attribute.String("environment", identity.environment),
	}, extra...)
	identity.RUnlock()
	return metric.WithAttributes(attrs...)
}

// metricInstruments are the synchronous instruments of this package.
type metricInstruments struct {
	orderCreate   metric.Int64Counter
	rpcDuration   metric.Float64Histogram
	callbackDelay metric.Float64Histogram
}

// instruments are created once, from the global meter provider. Created
// before InitMetrics installs the real provider, they are forwarded to it.
var instruments = sync.OnceValue(func() *metricInstruments {
	m := otel.Meter(Instrumentation)
	out := &metricInstruments{}
	out.orderCreate, _ = m.Int64Counter(MetricOrderCreate, metric.WithDescription("Order submissions by outcome."))
	out.rpcDuration, _ = m.Float64Histogram(MetricRPCDuration, metric.WithUnit("s"),
		metric.WithDescription("gRPC client calls to another service."), metric.WithExplicitBucketBoundaries(rpcBuckets...))
	out.callbackDelay, _ = m.Float64Histogram(MetricPaymentCallbackDelay, metric.WithUnit("s"),
		metric.WithDescription("From the provider's payment time to the order service confirming it."),
		metric.WithExplicitBucketBoundaries(callbackBuckets...))
	return out
})

// InitMetrics installs the global meter provider, pushing to the collector
// at endpoint every MetricsInterval. The returned function flushes and
// stops it.
func InitMetrics(ctx context.Context, endpoint string, res *resource.Resource) (func(context.Context) error, error) {
	exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(endpoint), otlpmetricgrpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(MetricsInterval))),
	)
	otel.SetMeterProvider(provider)
	return provider.Shutdown, nil
}

// RecordOrderCreate counts one order submission. outcome is one of
// pending_payment, reserving, rejected, duplicate, invalid, error.
func RecordOrderCreate(ctx context.Context, outcome string) {
	instruments().orderCreate.Add(ctx, 1, base(attribute.String("outcome", outcome)))
}

// RecordRPC records one gRPC client call to peer.
func RecordRPC(ctx context.Context, peer, method, outcome string, d time.Duration) {
	instruments().rpcDuration.Record(ctx, d.Seconds(), base(
		attribute.String("peer", peer), attribute.String("method", method), attribute.String("outcome", outcome)))
}

// RecordPaymentCallbackDelay records how long after the payment the order
// service confirmed it.
func RecordPaymentCallbackDelay(ctx context.Context, d time.Duration) {
	if d < 0 {
		d = 0 // clocks of the provider and the order service differ a little
	}
	instruments().callbackDelay.Record(ctx, d.Seconds(), base())
}

// RPCOutcome classifies a call's status code into the outcome label.
func RPCOutcome(code codes.Code) string {
	switch code {
	case codes.OK:
		return OutcomeOK
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied,
		codes.Unauthenticated, codes.FailedPrecondition, codes.OutOfRange:
		return OutcomeRejected
	default:
		return OutcomeError
	}
}

// PeerName turns a gRPC client target into the callee's service name as
// its spans and metrics report it. grpcx dials registered services as
// "service:///service/<deployment>/<namespace>/<name>/<version>/"; a plain
// name ("goods") or an address is accepted too.
func PeerName(target string) string {
	if i := strings.Index(target, "://"); i >= 0 {
		target = target[i+3:]
	}
	var parts []string
	for _, p := range strings.Split(target, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	name := parts[len(parts)-1]
	if len(parts) >= 4 && parts[0] == "service" {
		name = parts[3] // service/<deployment>/<namespace>/<name>/...
	}
	if i := strings.Index(name, ":"); i >= 0 {
		name = name[:i]
	}
	switch {
	case name == "paysim":
		return "payment-simulator"
	case name == "gateway-h5", name == "gateway-admin", strings.HasSuffix(name, "-service"):
		return name
	case strings.Contains(name, "."):
		return name // a host or address, not a registered service name
	}
	return name + "-service"
}

// Backlog is the work waiting at one step: how many, and since when the
// oldest has waited (zero when there is none).
type Backlog struct {
	Count  int64
	Oldest time.Time
}

// ObserveWorkflow reports trade_workflow_backlog and
// trade_workflow_oldest_age_seconds for steps, read by read at every
// collection (every MetricsInterval). A failing read is logged and skipped.
func ObserveWorkflow(read func(ctx context.Context) (map[string]Backlog, error)) error {
	m := otel.Meter(Instrumentation)
	backlog, err := m.Int64ObservableGauge(MetricWorkflowBacklog,
		metric.WithDescription("Work waiting for the background worker, by step."))
	if err != nil {
		return err
	}
	oldest, err := m.Float64ObservableGauge(MetricWorkflowOldestAge, metric.WithUnit("s"),
		metric.WithDescription("How long the oldest waiting work has waited, by step."))
	if err != nil {
		return err
	}
	_, err = m.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		// The reader runs every few seconds: its SQL must not start traces.
		steps, err := read(Quiet(ctx))
		if err != nil {
			g.Log().Warningf(ctx, "workflow metrics: %v", err)
			return nil
		}
		now := time.Now()
		for step, b := range steps {
			label := base(attribute.String("step", step))
			o.ObserveInt64(backlog, b.Count, metricObserve(label))
			age := 0.0
			if b.Count > 0 && !b.Oldest.IsZero() {
				age = max(now.Sub(b.Oldest).Seconds(), 0)
			}
			o.ObserveFloat64(oldest, age, metricObserve(label))
		}
		return nil
	}, backlog, oldest)
	return err
}

// ObservePendingPayment reports trade_order_pending_payment from count.
func ObservePendingPayment(count func(ctx context.Context) (int64, error)) error {
	m := otel.Meter(Instrumentation)
	gauge, err := m.Int64ObservableGauge(MetricOrderPendingPayment,
		metric.WithDescription("Orders waiting for payment."))
	if err != nil {
		return err
	}
	_, err = m.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		n, err := count(Quiet(ctx))
		if err != nil {
			g.Log().Warningf(ctx, "pending payment metric: %v", err)
			return nil
		}
		o.ObserveInt64(gauge, n, metricObserve(base()))
		return nil
	}, gauge)
	return err
}

// ObserveDBPool reports the connection pool of db under the label pool.
//
// trade_db_acquire_timeout_total counts the times a caller had to WAIT for
// a connection (sql.DBStats.WaitCount): database/sql has no acquire
// timeout of its own, a waiting caller just waits until its context ends,
// so waiting is the observable symptom of an exhausted pool.
func ObserveDBPool(pool string, db gdb.DB) error {
	m := otel.Meter(Instrumentation)
	inUse, err := m.Int64ObservableGauge(MetricDBPoolInUse, metric.WithDescription("Database connections in use."))
	if err != nil {
		return err
	}
	limit, err := m.Int64ObservableGauge(MetricDBPoolLimit, metric.WithDescription("Maximum open database connections (0: unlimited)."))
	if err != nil {
		return err
	}
	waits, err := m.Int64ObservableCounter(MetricDBAcquireTimeout,
		metric.WithDescription("Times a caller waited for a free connection."))
	if err != nil {
		return err
	}
	waited, err := m.Float64ObservableCounter(MetricDBAcquireWait, metric.WithUnit("s"),
		metric.WithDescription("Total time callers waited for a free connection."))
	if err != nil {
		return err
	}
	_, err = m.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		s := poolStats(Quiet(ctx), db)
		label := metricObserve(base(attribute.String("pool", pool)))
		o.ObserveInt64(inUse, int64(s.InUse), label)
		o.ObserveInt64(limit, int64(s.MaxOpenConnections), label)
		o.ObserveInt64(waits, s.WaitCount, label)
		o.ObserveFloat64(waited, s.WaitDuration.Seconds(), label)
		return nil
	}, inUse, limit, waits, waited)
	return err
}

// observeDatabases registers ObserveDBPool for every database group in the
// service's config (database.<group>), labelled with the group name. Start
// calls it, so no service can forget its pools.
func observeDatabases(ctx context.Context) {
	for group := range g.Cfg().MustGet(ctx, "database").Map() {
		if err := ObserveDBPool(group, g.DB(group)); err != nil {
			g.Log().Warningf(ctx, "pool metrics for database %s: %v", group, err)
		}
	}
}

// poolStats sums the stats of every node of db (one in this lab).
func poolStats(ctx context.Context, db gdb.DB) sql.DBStats {
	var total sql.DBStats
	for _, item := range db.Stats(ctx) {
		s := item.Stats()
		total.InUse += s.InUse
		total.MaxOpenConnections += s.MaxOpenConnections
		total.WaitCount += s.WaitCount
		total.WaitDuration += s.WaitDuration
	}
	return total
}

// metricObserve turns the attributes of a MeasurementOption into an
// ObserveOption (the API keeps the two option types apart).
func metricObserve(o metric.MeasurementOption) metric.ObserveOption {
	cfg := metric.NewAddConfig([]metric.AddOption{o})
	set := cfg.Attributes()
	return metric.WithAttributeSet(set)
}
