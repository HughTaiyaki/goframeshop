package cmd

import (
	"context"
	"time"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"
	paymentv1 "github.com/hughtaiyaki/goframeshop/app/order/api/payment/v1"
	refundv1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/clients"
	ordercontroller "github.com/hughtaiyaki/goframeshop/app/order/internal/controller/order"
	paymentcontroller "github.com/hughtaiyaki/goframeshop/app/order/internal/controller/payment"
	refundcontroller "github.com/hughtaiyaki/goframeshop/app/order/internal/controller/refund"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/events"
	orderlogic "github.com/hughtaiyaki/goframeshop/app/order/internal/logic/order"
	refundlogic "github.com/hughtaiyaki/goframeshop/app/order/internal/logic/refund"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"github.com/hughtaiyaki/goframeshop/utility/rabbitmq"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start order grpc server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// Three keys: shoppers' and administrators' PUBLIC keys to check
			// callers, and this service's own PRIVATE key to sign the service
			// token it shows the goods service.
			userKey, err := utility.LoadTokenVerifierAt(ctx, "jwtUser")
			if err != nil {
				return err
			}
			adminKey, err := utility.LoadTokenVerifierAt(ctx, "jwtAdmin")
			if err != nil {
				return err
			}
			serviceKey, err := utility.LoadTokenSignerAt(ctx, "jwtService")
			if err != nil {
				return err
			}
			// The payment simulator's PUBLIC key, to check its notifications.
			notices, err := utility.LoadNoticeVerifierAt(ctx, "paysim")
			if err != nil {
				return err
			}
			timing := service.Timing{
				ReserveTimeout: g.Cfg().MustGet(ctx, "order.reserveTimeout", "60s").Duration(),
				PaymentTimeout: g.Cfg().MustGet(ctx, "order.paymentTimeout", "15m").Duration(),
				AutoConfirm:    g.Cfg().MustGet(ctx, "order.autoConfirm", "240h").Duration(),
			}
			interval := g.Cfg().MustGet(ctx, "order.recoveryInterval", "5s").Duration()

			orderRepo := repository.NewOrder()
			token := utility.NewServiceToken(serviceKey)
			orders := orderlogic.New(orderlogic.Deps{
				Orders:    orderRepo,
				Payments:  repository.NewPayment(),
				Catalog:   clients.NewCatalog(),
				Addresses: clients.NewAddresses(),
				Inventory: clients.NewInventory(token),
				Provider:  clients.NewProvider(token),
				Notices:   notices,
				Timing:    timing,
				Now:       time.Now,
			})
			refunds := refundlogic.New(repository.NewRefund(orderRepo), orderRepo, time.Now)

			// Events: the relay publishes the outbox, the consumer closes
			// orders when their delayed expiry message arrives.
			amqpURL, err := rabbitmq.LoadURL(ctx)
			if err != nil {
				return err
			}
			relay := eventbus.NewRelay(dao.OrderInfo.DB(), rabbitmq.NewPublisher(amqpURL, 5*time.Second), eventbus.RelaySettings{
				Interval: g.Cfg().MustGet(ctx, "outbox.interval", "500ms").Duration(),
				Lease:    15 * time.Second,
				Batch:    100,
			}, time.Now)
			go relay.Run(ctx)
			go rabbitmq.Consume(ctx, amqpURL, rabbitmq.QueueOrderExpiry, 16,
				events.NewExpiry(dao.OrderInfo.DB(), orders, time.Now).Handle)

			go runRecovery(ctx, orders, interval)
			newServer(userKey, adminKey, orders, refunds).Run()
			return nil
		},
	}
)

// runRecovery drives stuck and expired orders forward and retries technical
// refunds. Expired orders normally close on their expiry message; this scan
// is the guarantee when a message is late or lost. It is safe to run in several instances at once: every state change
// is conditional and every inventory and provider call idempotent.
func runRecovery(ctx context.Context, orders service.IOrder, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := orders.Recover(ctx); err != nil {
				g.Log().Warningf(ctx, "order recovery pass failed: %v", err)
			}
		}
	}
}

// Who may call what. Anything not listed is refused.
var (
	// publicMethods: the provider's notification arrives without a token;
	// its signature is checked by the handler itself.
	publicMethods = []string{
		paymentv1.PaymentService_NotifyPayment_FullMethodName,
	}
	shopperMethods = []string{
		paymentv1.PaymentService_PayOrder_FullMethodName,
		orderv1.OrderService_CreateOrder_FullMethodName,
		orderv1.OrderService_CancelOrder_FullMethodName,
		orderv1.OrderService_ListMyOrders_FullMethodName,
		orderv1.OrderService_GetMyOrder_FullMethodName,
		orderv1.OrderService_ConfirmReceipt_FullMethodName,
		refundv1.RefundService_CreateRefund_FullMethodName,
		refundv1.RefundService_ListMyRefunds_FullMethodName,
		refundv1.RefundService_GetMyRefund_FullMethodName,
	}
	adminMethods = []string{
		orderv1.OrderService_ListOrders_FullMethodName,
		orderv1.OrderService_GetOrder_FullMethodName,
		orderv1.OrderService_ShipOrder_FullMethodName,
		refundv1.RefundService_ListRefunds_FullMethodName,
	}
)

func newServer(userKey, adminKey *utility.TokenVerifier, orders service.IOrder, refunds service.IRefund) *grpcx.GrpcServer {
	policy := utility.AuthPolicy{
		Verifiers: map[utility.Role]*utility.TokenVerifier{utility.RoleUser: userKey, utility.RoleAdmin: adminKey},
		Public:    publicMethods,
		Methods:   map[string][]utility.Role{},
	}
	for _, m := range shopperMethods {
		policy.Methods[m] = []utility.Role{utility.RoleUser}
	}
	for _, m := range adminMethods {
		policy.Methods[m] = []utility.Role{utility.RoleAdmin}
	}
	s := grpcx.Server.New(utility.NewGrpcServerConfigWithPolicy(policy))
	ordercontroller.Register(s, orders)
	refundcontroller.Register(s, refunds)
	paymentcontroller.Register(s, orders)
	return s
}
