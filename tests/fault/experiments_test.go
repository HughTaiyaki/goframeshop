package fault

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cartv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"
	inventoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"
	paymentv1 "github.com/hughtaiyaki/goframeshop/app/order/api/payment/v1"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"github.com/hughtaiyaki/goframeshop/utility/ittest"
	"github.com/hughtaiyaki/goframeshop/utility/rabbitmq"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

// TestFaultTrade runs every experiment against freshly started processes
// and writes reports/fault/<time>/report.{md,json}. Opt-in, like the
// integration tests, because it needs the Compose infrastructure and free
// service ports:
//
//	make infra-up && make fault-trade
func TestFaultTrade(t *testing.T) {
	if os.Getenv("GOFRAMESHOP_FAULT") != "1" {
		t.Skip("set GOFRAMESHOP_FAULT=1 (make fault-trade) to run the fault experiments")
	}
	started := time.Now()
	out := filepath.Join(repoRoot(), "reports", "fault", started.Format("20060102-150405"))
	cluster, err := NewCluster(filepath.Join(out, "work"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cluster.StopAll)
	for _, s := range services {
		if err := cluster.Start(s.Name, ""); err != nil {
			t.Fatalf("start %s: %v", s.Name, err)
		}
	}
	shop := NewShop(t, cluster)
	report := &Report{Started: started, Environment: "experiment"}
	ctx := context.Background()

	run := func(id, title, fault string, experiment func(t *testing.T, r *Result)) {
		t.Run(id, func(t *testing.T) {
			r := &Result{ID: id, Title: title, Fault: fault}
			begin := time.Now()
			experiment(t, r)
			r.Duration = time.Since(begin)
			for _, v := range r.Violations {
				t.Error(v)
			}
			r.Passed = !t.Failed()
			report.Results = append(report.Results, r)
		})
	}
	// check records the invariants of the goods an experiment used.
	check := func(t *testing.T, r *Result, initial map[uint64]int64) {
		t.Helper()
		bad, err := shop.Invariants(ctx, initial)
		if err != nil {
			t.Fatal(err)
		}
		r.Violations = append(r.Violations, bad...)
	}
	// restart puts a service back with the given failpoints.
	restart := func(t *testing.T, name, failpoints string) {
		t.Helper()
		if err := cluster.Restart(name, failpoints); err != nil {
			t.Fatalf("restart %s: %v", name, err)
		}
	}

	run("T1", "100 个请求抢 10 件库存", "并发：100 个不同的下单请求同时购买 1 件", func(t *testing.T, r *Result) {
		goods := shop.NewGoods(ctx, t, 10)
		sh := shop.NewShopper(ctx, t)
		var wg sync.WaitGroup
		var mu sync.Mutex
		errs := 0
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := shop.Place(ctx, sh, shop.ID("t1"), goods, 1, false); err != nil {
					mu.Lock()
					errs++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if err := shop.WaitSettled(ctx, goods, 30*time.Second); err != nil {
			t.Fatal(err)
		}
		counts, err := shop.StatusCounts(ctx, goods)
		if err != nil {
			t.Fatal(err)
		}
		if counts["PENDING_PAYMENT"] > 10 {
			t.Errorf("%d orders hold stock, only 10 exist", counts["PENDING_PAYMENT"])
		}
		r.Notes = append(r.Notes, fmt.Sprintf("100 个请求：%d 个进入待支付，%d 个因库存不足关闭，%d 个调用出错",
			counts["PENDING_PAYMENT"], counts["CLOSED"], errs))
		check(t, r, map[uint64]int64{goods: 10})
	})

	run("T2", "同一个 requestId 并发 20 次", "重复提交：20 个并发请求用同一个 requestId；再换参数提交", func(t *testing.T, r *Result) {
		goods := shop.NewGoods(ctx, t, 5)
		sh := shop.NewShopper(ctx, t)
		requestID := shop.ID("t2")
		var wg sync.WaitGroup
		var mu sync.Mutex
		seen := map[string]bool{}
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if o, err := shop.Place(ctx, sh, requestID, goods, 1, false); err == nil {
					mu.Lock()
					seen[o.OrderNo] = true
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		n, err := shop.OrdersDB.GetValue(ctx, "SELECT COUNT(*) FROM order_info WHERE user_id = ? AND request_id = ?", sh.UserID, requestID)
		if err != nil {
			t.Fatal(err)
		}
		if n.Int() != 1 || len(seen) != 1 {
			t.Errorf("%d orders stored, %d distinct order numbers answered; want 1 and 1", n.Int(), len(seen))
		}
		_, err = shop.Place(ctx, sh, requestID, goods, 2, false)
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("same request id, other quantity: %v, want AlreadyExists", err)
		}
		for no := range seen {
			r.Orders = append(r.Orders, no)
		}
		r.Notes = append(r.Notes, fmt.Sprintf("20 个并发请求只产生 1 个订单；换参数返回 %s", status.Code(err)))
		if err := shop.WaitSettled(ctx, goods, 20*time.Second); err != nil {
			t.Fatal(err)
		}
		check(t, r, map[uint64]int64{goods: 5})
	})

	run("T4", "释放先到、预留后到", "乱序：直接调库存接口，先释放、再预留同一个订单", func(t *testing.T, r *Result) {
		goods := shop.NewGoods(ctx, t, 3)
		orderID := shop.ID("t4")
		items := []*inventoryv1.InventoryItem{{GoodsId: goods, Quantity: 2}}
		rel, err := shop.Inventory.ReleaseInventory(shop.ServiceCtx(ctx), &inventoryv1.ReleaseInventoryRequest{
			OperationId: "release-" + orderID, OrderId: orderID, Items: items})
		if err != nil {
			t.Fatal(err)
		}
		res, err := shop.Inventory.ReserveInventory(shop.ServiceCtx(ctx), &inventoryv1.ReserveInventoryRequest{
			OperationId: "reserve-" + orderID, OrderId: orderID, Items: items})
		if err != nil {
			t.Fatal(err)
		}
		if res.OriginalOutcome != inventoryv1.OperationOutcome_ALREADY_RELEASED {
			t.Errorf("late reserve: %s, want ALREADY_RELEASED", res.OriginalOutcome)
		}
		st, _ := shop.Stock(ctx, goods)
		if st != (Stock{Available: 3}) {
			t.Errorf("stock %+v, want untouched 3/0/0", st)
		}
		r.Notes = append(r.Notes, fmt.Sprintf("先释放：%s（留下墓碑）；迟到的预留：%s；库存 %d/%d/%d",
			rel.OriginalOutcome, res.OriginalOutcome, st.Available, st.Reserved, st.Sold))
		check(t, r, map[uint64]int64{goods: 3})
	})

	run("T6", "已支付的订单收到到期消息", "重放：给已支付的订单投递一条到期消息", func(t *testing.T, r *Result) {
		goods := shop.NewGoods(ctx, t, 5)
		sh := shop.NewShopper(ctx, t)
		o := shop.mustPaid(ctx, t, sh, goods)
		r.Orders = append(r.Orders, o)
		eventID := uuid.NewString()
		shop.publishExpiry(ctx, t, eventID, o)
		if err := Eventually(ctx, 20*time.Second, func() (bool, error) {
			n, err := shop.OrdersDB.GetValue(ctx, "SELECT COUNT(*) FROM inbox_event WHERE consumer = 'order.expiry' AND event_id = ?", eventID)
			return n.Int() == 1, err
		}); err != nil {
			t.Fatalf("the expiry message was never consumed: %v", err)
		}
		if s, _ := shop.Status(ctx, o); s != "PAID" {
			t.Errorf("order %s is %s after a stale expiry, want PAID", o, s)
		}
		if n := shop.opCount(ctx, t, "release-"+o); n != 0 {
			t.Errorf("a paid order got %d release operations", n)
		}
		r.Notes = append(r.Notes, "到期消息被消费（Inbox 有记录），订单仍是已支付，没有释放库存的操作")
		check(t, r, map[uint64]int64{goods: 5})
	})

	run("T9", "到期消息丢失", "消息丢失：订单的到期消息进入延迟队列后被清空", func(t *testing.T, r *Result) {
		goods := shop.NewGoods(ctx, t, 5)
		sh := shop.NewShopper(ctx, t)
		o, err := shop.Place(ctx, sh, shop.ID("t9"), goods, 1, false)
		if err != nil {
			t.Fatal(err)
		}
		r.Orders = append(r.Orders, o.OrderNo)
		eventID := shop.waitSent(ctx, t, o.OrderNo, "order.payment_pending")
		purged := shop.purgeExpiryDelay(t)
		if err := shop.WaitStatus(ctx, o.OrderNo, "CLOSED", 40*time.Second); err != nil {
			t.Fatal(err)
		}
		row, _ := shop.OrdersDB.GetOne(ctx, "SELECT close_reason, expires_at, closed_at FROM order_info WHERE order_no = ?", o.OrderNo)
		if row["close_reason"].String() != "PAYMENT_TIMEOUT" {
			t.Errorf("closed for %s, want PAYMENT_TIMEOUT", row["close_reason"].String())
		}
		if n, _ := shop.OrdersDB.GetValue(ctx, "SELECT COUNT(*) FROM inbox_event WHERE consumer = 'order.expiry' AND event_id = ?", eventID); n.Int() != 0 {
			t.Error("the purged message was consumed after all")
		}
		if n := shop.opCount(ctx, t, "release-"+o.OrderNo); n != 1 {
			t.Errorf("%d release operations, want 1", n)
		}
		r.Recovery = row["closed_at"].GTime().Sub(row["expires_at"].GTime())
		r.Notes = append(r.Notes, fmt.Sprintf("清空了延迟队列里的 %d 条消息；数据库扫描在到期后 %s 关单并释放库存",
			purged, r.Recovery.Round(10*time.Millisecond)))
		check(t, r, map[uint64]int64{goods: 5})
	})

	run("T3", "预留成功但响应丢失", "goods.reserve.after-commit=error", func(t *testing.T, r *Result) {
		restart(t, "goods", "goods.reserve.after-commit=error")
		defer restart(t, "goods", "")
		goods := shop.NewGoods(ctx, t, 5)
		sh := shop.NewShopper(ctx, t)
		begin := time.Now()
		o, err := shop.Place(ctx, sh, shop.ID("t3"), goods, 2, false)
		if err != nil {
			t.Fatal(err)
		}
		r.Orders = append(r.Orders, o.OrderNo)
		if o.Status != orderv1.OrderStatus_RESERVING {
			t.Errorf("order answered %s, want RESERVING (the reply was lost)", o.Status)
		}
		if err := shop.WaitStatus(ctx, o.OrderNo, "PENDING_PAYMENT", 20*time.Second); err != nil {
			t.Fatal(err)
		}
		r.Recovery = time.Since(begin)
		if n, _ := shop.Goods.GetValue(ctx, "SELECT COUNT(*) FROM reservation WHERE order_id = ?", o.OrderNo); n.Int() != 1 {
			t.Errorf("%d reservations, want 1", n.Int())
		}
		if n := shop.opCount(ctx, t, "reserve-"+o.OrderNo); n != 1 {
			t.Errorf("%d reserve operations recorded, want 1", n)
		}
		retries := shop.logEvents(t, cluster, "order", telemetry.EventWorkflowRetry, "order_id", o.OrderNo)
		r.Notes = append(r.Notes, fmt.Sprintf("下单时收到 RESERVING；恢复任务重试 %d 次后进入待支付；只有 1 条预留和 1 条操作记录", len(retries)))
		check(t, r, map[uint64]int64{goods: 5})
	})

	run("C1", "订单写入后进程崩溃", "order.create.after-insert=exit", func(t *testing.T, r *Result) {
		restart(t, "order", "order.create.after-insert=exit")
		goods := shop.NewGoods(ctx, t, 5)
		sh := shop.NewShopper(ctx, t)
		requestID := shop.ID("c1")
		_, placeErr := shop.Place(ctx, sh, requestID, goods, 1, false)
		if err := cluster.WaitExit("order", 15*time.Second); err != nil {
			t.Fatal(err)
		}
		crashed := time.Now()
		restart(t, "order", "")
		no, err := shop.OrdersDB.GetValue(ctx, "SELECT order_no FROM order_info WHERE user_id = ? AND request_id = ?", sh.UserID, requestID)
		if err != nil || no.String() == "" {
			t.Fatalf("the order written before the crash is missing: %v", err)
		}
		r.Orders = append(r.Orders, no.String())
		if err := shop.WaitStatus(ctx, no.String(), "PENDING_PAYMENT", 20*time.Second); err != nil {
			t.Fatal(err)
		}
		r.Recovery = time.Since(crashed)
		if n, _ := shop.Goods.GetValue(ctx, "SELECT COUNT(*) FROM reservation WHERE order_id = ?", no.String()); n.Int() != 1 {
			t.Errorf("%d reservations, want 1", n.Int())
		}
		r.Notes = append(r.Notes, fmt.Sprintf("下单请求收到 %s（进程崩溃）；重启后恢复任务接着预留，%s 后进入待支付", status.Code(placeErr), r.Recovery.Round(10*time.Millisecond)))
		check(t, r, map[uint64]int64{goods: 5})
	})

	run("T5", "付款与关单竞争", "order.payment.before-record=pause：付款记账前停住，此时顾客取消订单", func(t *testing.T, r *Result) {
		restart(t, "order", "order.payment.before-record=pause")
		defer restart(t, "order", "")
		goods := shop.NewGoods(ctx, t, 5)
		sh := shop.NewShopper(ctx, t)
		o, err := shop.Place(ctx, sh, shop.ID("t5"), goods, 1, false)
		if err != nil {
			t.Fatal(err)
		}
		r.Orders = append(r.Orders, o.OrderNo)
		if err := shop.PayAtProvider(ctx, sh, o.OrderNo); err != nil {
			t.Fatal(err)
		}
		// The shopper comes back to the order: it asks the provider, finds
		// the payment, and stops right before recording it.
		payDone := make(chan error, 1)
		go func() {
			_, err := shop.Payments.PayOrder(sh.Ctx(ctx), &paymentv1.PayOrderRequest{OrderNo: o.OrderNo})
			payDone <- err
		}()
		if err := Eventually(ctx, 15*time.Second, func() (bool, error) { return cluster.Hit("order.payment.before-record"), nil }); err != nil {
			t.Fatalf("the payment never reached the barrier: %v", err)
		}
		cancelled, err := shop.Orders.CancelOrder(sh.Ctx(ctx), &orderv1.CancelOrderRequest{OrderNo: o.OrderNo})
		if err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if err := cluster.Release("order.payment.before-record"); err != nil {
			t.Fatal(err)
		}
		payErr := <-payDone
		if err := shop.WaitStatus(ctx, o.OrderNo, "CLOSED", 20*time.Second); err != nil {
			t.Fatal(err)
		}
		paymentID := "P" + o.OrderNo
		if err := Eventually(ctx, 20*time.Second, func() (bool, error) {
			s, err := shop.Paysim.GetValue(ctx, "SELECT state FROM payment WHERE payment_id = ?", paymentID)
			return s.String() == "REFUNDED", err
		}); err != nil {
			t.Fatalf("the payment was not refunded: %v", err)
		}
		refunds, _ := shop.Paysim.GetValue(ctx, "SELECT COUNT(*) FROM refund WHERE payment_id = ?", paymentID)
		if refunds.Int() != 1 {
			t.Errorf("%d refunds, want exactly 1", refunds.Int())
		}
		st, _ := shop.Stock(ctx, goods)
		if st.Sold != 0 || st.Reserved != 0 {
			t.Errorf("stock %+v: a closed order must leave nothing reserved or sold", st)
		}
		r.Notes = append(r.Notes, fmt.Sprintf("取消时订单变为 %s；放行后付款记账返回 %v；订单关闭，付款退回 1 次，库存 %d/%d/%d",
			cancelled.Order.Status, status.Code(payErr), st.Available, st.Reserved, st.Sold))
		check(t, r, map[uint64]int64{goods: 5})
	})

	run("T7", "消费提交后、确认前崩溃", "eventbus.inbox.after-commit=exit（goods 的购物车消费者）", func(t *testing.T, r *Result) {
		goods := shop.NewGoods(ctx, t, 5)
		sh := shop.NewShopper(ctx, t)
		if _, err := shop.Carts.AddCartItem(sh.Ctx(ctx), &cartv1.AddCartItemRequest{GoodsId: goods, Quantity: 2}); err != nil {
			t.Fatal(err)
		}
		restart(t, "goods", "eventbus.inbox.after-commit=exit")
		o, err := shop.Place(ctx, sh, shop.ID("t7"), goods, 1, true)
		if err != nil {
			t.Fatal(err)
		}
		r.Orders = append(r.Orders, o.OrderNo)
		if err := cluster.WaitExit("goods", 20*time.Second); err != nil {
			t.Fatal(err)
		}
		crashed := time.Now()
		restart(t, "goods", "")
		eventID := shop.eventID(ctx, t, o.OrderNo, "order.placed")
		var redelivered []map[string]any
		if err := Eventually(ctx, 30*time.Second, func() (bool, error) {
			redelivered = shop.logEvents(t, cluster, "goods", telemetry.EventMessageRedelivered, "event_id", eventID)
			return len(redelivered) > 0, nil
		}); err != nil {
			t.Fatalf("the unacknowledged message was not redelivered: %v", err)
		}
		r.Recovery = time.Since(crashed)
		q, _ := shop.Goods.GetValue(ctx, "SELECT quantity FROM cart_item WHERE user_id = ? AND goods_id = ?", sh.UserID, goods)
		if q.Int() != 1 {
			t.Errorf("cart quantity %d, want 1 (2 minus 1 bought, once)", q.Int())
		}
		if n, _ := shop.Goods.GetValue(ctx, "SELECT COUNT(*) FROM inbox_event WHERE consumer = 'goods.cart-order-placed' AND event_id = ?", eventID); n.Int() != 1 {
			t.Errorf("%d inbox rows, want 1", n.Int())
		}
		r.Notes = append(r.Notes, fmt.Sprintf("goods 提交后崩溃；重启后 RabbitMQ 重投，Inbox 认出并只确认（%d 条 message.redelivered）；购物车 2 → %d，只减了一次", len(redelivered), q.Int()))
		check(t, r, map[uint64]int64{goods: 5})
	})

	run("T8", "事件发出后、标记 SENT 前崩溃", "eventbus.relay.after-publish=exit（order 的投递器）", func(t *testing.T, r *Result) {
		restart(t, "order", "eventbus.relay.after-publish=exit")
		goods := shop.NewGoods(ctx, t, 5)
		sh := shop.NewShopper(ctx, t)
		requestID := shop.ID("t8")
		_, _ = shop.Place(ctx, sh, requestID, goods, 1, false) // the reply may be cut by the crash
		if err := cluster.WaitExit("order", 20*time.Second); err != nil {
			t.Fatal(err)
		}
		crashed := time.Now()
		restart(t, "order", "")
		no, err := shop.OrdersDB.GetValue(ctx, "SELECT order_no FROM order_info WHERE user_id = ? AND request_id = ?", sh.UserID, requestID)
		if err != nil || no.String() == "" {
			t.Fatalf("order missing: %v", err)
		}
		r.Orders = append(r.Orders, no.String())
		// The order expires (8s) and both events have been delivered: the
		// one published twice is seen twice by its consumer.
		if err := shop.WaitStatus(ctx, no.String(), "CLOSED", 40*time.Second); err != nil {
			t.Fatal(err)
		}
		ids := map[string]string{
			shop.eventID(ctx, t, no.String(), "order.payment_pending"): "order.expiry",
			shop.eventID(ctx, t, no.String(), "order.placed"):          "goods.cart-order-placed",
		}
		var duplicated []string
		if err := Eventually(ctx, 20*time.Second, func() (bool, error) {
			duplicated = nil
			for id := range ids {
				for _, svc := range []string{"order", "goods"} {
					if len(shop.logEvents(t, cluster, svc, telemetry.EventMessageRedelivered, "event_id", id)) > 0 {
						duplicated = append(duplicated, id)
					}
				}
			}
			return len(duplicated) > 0, nil
		}); err != nil {
			t.Fatal("no event was delivered twice: the crash did not leave a published-but-PENDING row")
		}
		r.Recovery = time.Since(crashed)
		for id, consumer := range ids {
			db := shop.OrdersDB
			if consumer == "goods.cart-order-placed" {
				db = shop.Goods
			}
			if n, _ := db.GetValue(ctx, "SELECT COUNT(*) FROM inbox_event WHERE consumer = ? AND event_id = ?", consumer, id); n.Int() != 1 {
				t.Errorf("event %s: %d inbox rows for %s, want 1", id, n.Int(), consumer)
			}
			// A redelivery is a duplicate to acknowledge, never a conflict.
			if n, _ := db.GetValue(ctx, "SELECT COUNT(*) FROM quarantine_event WHERE consumer = ? AND event_id = ?", consumer, id); n.Int() != 0 {
				t.Errorf("event %s was quarantined by %s instead of acknowledged as a duplicate", id, consumer)
			}
		}
		r.Notes = append(r.Notes, fmt.Sprintf("order 发出事件后崩溃；重启后用同一个 event_id 重发（%v），消费方只处理一次", duplicated))
		check(t, r, map[uint64]int64{goods: 5})
	})

	run("T10", "新旧 Protobuf 兼容", "对照：当前协议和提交的快照（testdata/proto_snapshot.json）", func(t *testing.T, r *Result) {
		data, err := os.ReadFile(snapshotPath)
		if err != nil {
			t.Fatal(err)
		}
		var old ProtoSnapshot
		if err := json.Unmarshal(data, &old); err != nil {
			t.Fatal(err)
		}
		current := CurrentProtos()
		r.Violations = append(r.Violations, CompareProtos(old, current)...)
		r.Notes = append(r.Notes, fmt.Sprintf("快照 %d 个消息，当前 %d 个：已有字段的编号、类型、名字都没有变", len(old), len(current)))
	})

	report.Finished = time.Now()
	path, err := report.Write(out)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("report: %s", path)
}

// --- helpers used only by the experiments --------------------------------

// mustPaid places an order and gets it to PAID without the notification:
// pay at the simulator, then the shopper's PayOrder finds and confirms it.
func (s *Shop) mustPaid(ctx context.Context, t *testing.T, sh *Shopper, goods uint64) string {
	t.Helper()
	o, err := s.Place(ctx, sh, s.ID("paid"), goods, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PayAtProvider(ctx, sh, o.OrderNo); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Payments.PayOrder(sh.Ctx(ctx), &paymentv1.PayOrderRequest{OrderNo: o.OrderNo}); err != nil {
		t.Fatal(err)
	}
	if err := s.WaitStatus(ctx, o.OrderNo, "PAID", 20*time.Second); err != nil {
		t.Fatal(err)
	}
	return o.OrderNo
}

// WaitSettled waits until no order of goods is still in flight.
func (s *Shop) WaitSettled(ctx context.Context, goods uint64, timeout time.Duration) error {
	return Eventually(ctx, timeout, func() (bool, error) {
		n, err := s.OrdersDB.GetValue(ctx, "SELECT COUNT(*) FROM order_info o JOIN order_goods g ON g.order_id = o.id "+
			"WHERE g.goods_id = ? AND o.status IN ('RESERVING','CLOSING','PAYMENT_CONFIRMING','EXCEPTION')", goods)
		return n.Int() == 0, err
	})
}

// StatusCounts counts the orders of goods by state.
func (s *Shop) StatusCounts(ctx context.Context, goods uint64) (map[string]int, error) {
	rows, err := s.OrdersDB.GetAll(ctx, "SELECT o.status, COUNT(*) AS n FROM order_info o JOIN order_goods g ON g.order_id = o.id "+
		"WHERE g.goods_id = ? GROUP BY o.status", goods)
	out := map[string]int{}
	for _, r := range rows {
		out[r["status"].String()] = r["n"].Int()
	}
	return out, err
}

func (s *Shop) opCount(ctx context.Context, t *testing.T, operationID string) int {
	t.Helper()
	n, err := s.Goods.GetValue(ctx, "SELECT COUNT(*) FROM inventory_operation WHERE operation_id = ?", operationID)
	if err != nil {
		t.Fatal(err)
	}
	return n.Int()
}

// eventID is the outbox id of an order's event of type typ.
func (s *Shop) eventID(ctx context.Context, t *testing.T, orderNo, typ string) string {
	t.Helper()
	v, err := s.OrdersDB.GetValue(ctx, "SELECT event_id FROM outbox_event WHERE aggregate_id = ? AND type = ?", orderNo, typ)
	if err != nil || v.String() == "" {
		t.Fatalf("no %s event for order %s: %v", typ, orderNo, err)
	}
	return v.String()
}

// waitSent waits until the order's event of type typ is published and
// returns its id.
func (s *Shop) waitSent(ctx context.Context, t *testing.T, orderNo, typ string) string {
	t.Helper()
	if err := Eventually(ctx, 15*time.Second, func() (bool, error) {
		v, err := s.OrdersDB.GetValue(ctx, "SELECT state FROM outbox_event WHERE aggregate_id = ? AND type = ?", orderNo, typ)
		return v.String() == "SENT", err
	}); err != nil {
		t.Fatalf("%s for %s never sent: %v", typ, orderNo, err)
	}
	return s.eventID(ctx, t, orderNo, typ)
}

func amqpChannel(t *testing.T) *amqp.Channel {
	t.Helper()
	conn, err := amqp.Dial(ittest.RabbitMQ(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

// publishExpiry delivers an expiry message for orderNo straight to the
// order service's queue, as if its delay had run out.
func (s *Shop) publishExpiry(ctx context.Context, t *testing.T, eventID, orderNo string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"orderNo": orderNo, "expiresAt": time.Now().Add(-time.Minute).UTC()})
	body, _ := json.Marshal(eventbus.Envelope{SchemaVersion: eventbus.SchemaVersion, EventID: eventID, Type: "order.payment_pending",
		AggregateID: orderNo, OccurredAt: time.Now().UTC(), Payload: payload})
	if err := amqpChannel(t).PublishWithContext(ctx, rabbitmq.Exchange, rabbitmq.KeyOrderExpiryDue, true, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: eventID, Body: body,
	}); err != nil {
		t.Fatal(err)
	}
}

// purgeExpiryDelay drops every message waiting in the expiry delay queue.
func (s *Shop) purgeExpiryDelay(t *testing.T) int {
	t.Helper()
	n, err := amqpChannel(t).QueuePurge(rabbitmq.QueueOrderExpiryDelay, false)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// logEvents reads a service's log for events whose attribute key (or
// top-level field) equals value.
func (s *Shop) logEvents(t *testing.T, c *Cluster, service, name, key, value string) []map[string]any {
	t.Helper()
	lines, err := LogEvents(c.LogPath(service), name)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, l := range lines {
		if Attr(l, key) == value || fmt.Sprint(l[key]) == value {
			out = append(out, l)
		}
	}
	return out
}
