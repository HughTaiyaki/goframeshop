package fault

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	cartv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/cart/v1"
	inventoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"
	paymentv1 "github.com/hughtaiyaki/goframeshop/app/order/api/payment/v1"
	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"
	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/ittest"
)

// Shop talks to the processes of a Cluster the way their callers do (gRPC
// with tokens, the simulator's checkout over HTTP), and reads the databases
// to set up goods and check invariants.
type Shop struct {
	Users      userv1.UserServiceClient
	Consignees consigneev1.ConsigneeServiceClient
	Orders     orderv1.OrderServiceClient
	Payments   paymentv1.PaymentServiceClient
	Inventory  inventoryv1.InventoryServiceClient
	Carts      cartv1.CartServiceClient

	Goods, OrdersDB, Paysim gdb.DB

	service *utility.TokenSigner // the order service's own identity
	run     string               // unique per run: user names, request ids
	seq     atomic.Int64
}

func dial(t *testing.T, port int) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func NewShop(t *testing.T, c *Cluster) *Shop {
	t.Helper()
	cfg := gdb.Config{}
	for group, db := range map[string]string{"goods": "goods", "orders": "orders", "paysim": "paysim"} {
		cfg[group] = gdb.ConfigGroup{{Link: ittest.MySQL(t, db), MaxOpenConnCount: 32}}
	}
	if err := gdb.SetConfig(cfg); err != nil {
		t.Fatal(err)
	}
	s := &Shop{
		Users:      userv1.NewUserServiceClient(dial(t, 31001)),
		Consignees: consigneev1.NewConsigneeServiceClient(dial(t, 31001)),
		Orders:     orderv1.NewOrderServiceClient(dial(t, 31007)),
		Payments:   paymentv1.NewPaymentServiceClient(dial(t, 31007)),
		Inventory:  inventoryv1.NewInventoryServiceClient(dial(t, 31005)),
		Carts:      cartv1.NewCartServiceClient(dial(t, 31005)),
		run:        fmt.Sprintf("ft%d", time.Now().Unix()),
	}
	var err error
	for name, target := range map[string]*gdb.DB{"goods": &s.Goods, "orders": &s.OrdersDB, "paysim": &s.Paysim} {
		if *target, err = gdb.Instance(name); err != nil {
			t.Fatal(err)
		}
	}
	// The order service's key, from its config, to call the inventory API
	// as order does (T4).
	orderCfg, err := gjson.LoadPath(filepath.Join(c.Work, "config", "order", "config.yaml"), gjson.Options{Safe: true})
	if err != nil {
		t.Fatal(err)
	}
	seed, err := base64.StdEncoding.DecodeString(orderCfg.Get("jwtService.privateKey").String())
	if err != nil {
		t.Fatalf("order jwtService.privateKey: %v", err)
	}
	if s.service, err = utility.NewTokenSigner(seed, orderCfg.Get("jwtService.issuer", "goframeshop-order").String(), time.Hour); err != nil {
		t.Fatal(err)
	}
	return s
}

// ID is a fresh identifier for this run: request ids, user names, orders.
func (s *Shop) ID(prefix string) string {
	return fmt.Sprintf("%s-%s-%d", s.run, prefix, s.seq.Add(1))
}

// Shopper is a registered, logged-in customer with one address.
type Shopper struct {
	UserID      uint64
	Token       string
	ConsigneeID uint64
}

func (sh *Shopper) Ctx(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+sh.Token)
}

func (s *Shop) NewShopper(ctx context.Context, t *testing.T) *Shopper {
	t.Helper()
	name, password := s.ID("user"), "Fault-"+s.run+"-pw"
	reg, err := s.Users.RegisterUser(ctx, &userv1.RegisterRequest{Username: name, Password: password})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := s.Users.Login(ctx, &userv1.LoginRequest{Username: name, Password: password})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	sh := &Shopper{UserID: reg.UserId, Token: login.AccessToken}
	address, err := s.Consignees.CreateConsignee(sh.Ctx(ctx), &consigneev1.CreateConsigneeRequest{
		IsDefault: true, Name: "实验", Phone: "13800000000", Province: "广东省", City: "东莞市", Town: "松山湖", Street: "实验路", Detail: "1 号",
	})
	if err != nil {
		t.Fatalf("address: %v", err)
	}
	sh.ConsigneeID = address.ConsigneeId
	return sh
}

// NewGoods puts a product on shelf with stock units, straight into the
// goods database: each experiment gets its own goods, untouched by anything
// else.
func (s *Shop) NewGoods(ctx context.Context, t *testing.T, stock uint32) uint64 {
	t.Helper()
	res, err := s.Goods.Exec(ctx, "INSERT INTO goods_info (name, status, price_cents, level1_category_id) VALUES (?, 1, 100, 1)", s.ID("goods"))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := s.Goods.Exec(ctx, "INSERT INTO inventory (goods_id, available) VALUES (?, ?)", id, stock); err != nil {
		t.Fatal(err)
	}
	return uint64(id)
}

// Place submits an order of quantity units of goodsID.
func (s *Shop) Place(ctx context.Context, sh *Shopper, requestID string, goodsID uint64, quantity uint32, fromCart bool) (*orderv1.Order, error) {
	res, err := s.Orders.CreateOrder(sh.Ctx(ctx), &orderv1.CreateOrderRequest{
		RequestId: requestID, ConsigneeId: sh.ConsigneeID, FromCart: fromCart,
		Items: []*orderv1.OrderItemInput{{GoodsId: goodsID, Quantity: quantity}},
	})
	if err != nil {
		return nil, err
	}
	return res.Order, nil
}

// PayAtProvider opens the payment and pays it at the simulator without the
// notification, so the order learns of it only when it asks (PayOrder
// again, or a close).
func (s *Shop) PayAtProvider(ctx context.Context, sh *Shopper, orderNo string) error {
	res, err := s.Payments.PayOrder(sh.Ctx(ctx), &paymentv1.PayOrderRequest{OrderNo: orderNo})
	if err != nil {
		return err
	}
	if res.GetPayUrl() == "" {
		return fmt.Errorf("order %s: no pay url (state %s)", orderNo, res.GetState())
	}
	resp, err := http.Post(res.GetPayUrl()+"/pay?notify=0", "application/json", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("simulator pay: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Status reads an order's state from the database.
func (s *Shop) Status(ctx context.Context, orderNo string) (string, error) {
	v, err := s.OrdersDB.GetValue(ctx, "SELECT status FROM order_info WHERE order_no = ?", orderNo)
	return v.String(), err
}

// WaitStatus waits until the order is in want.
func (s *Shop) WaitStatus(ctx context.Context, orderNo, want string, timeout time.Duration) error {
	var last string
	err := Eventually(ctx, timeout, func() (bool, error) {
		var err error
		last, err = s.Status(ctx, orderNo)
		return last == want, err
	})
	if err != nil {
		return fmt.Errorf("order %s: still %s, want %s: %w", orderNo, last, want, err)
	}
	return nil
}

// ServiceCtx calls as the order service (inventory API).
func (s *Shop) ServiceCtx(ctx context.Context) context.Context {
	// Subject 1, as utility.ServiceToken issues it.
	token, _, err := s.service.Issue(1, time.Now())
	if err != nil {
		panic(err)
	}
	return metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+token)
}
