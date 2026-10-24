package cmd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	orderv1 "github.com/hughtaiyaki/goframeshop/app/order/api/order/v1"
	paymentv1 "github.com/hughtaiyaki/goframeshop/app/order/api/payment/v1"
	refundv1 "github.com/hughtaiyaki/goframeshop/app/order/api/refund/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
)

func newSigner(t *testing.T, issuer string) *utility.TokenSigner {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := utility.NewTokenSigner(key.Seed(), issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// Shopper methods take user tokens only, admin methods admin tokens only, and
// only the payment notification is public (its signature is checked inside). Every case is decided before a handler runs (an
// InvalidArgument proves the call passed authentication), so no database or
// downstream service is needed.
func TestWhoMayCallWhat(t *testing.T) {
	user, admin := newSigner(t, "goframeshop-user"), newSigner(t, "goframeshop-admin")
	server := newServer(user.Verifier(), admin.Verifier(), nil, nil)
	listener := bufconn.Listen(1 << 20)
	done := make(chan error, 1)
	go func() { done <- server.Server.Serve(listener) }()
	t.Cleanup(func() { server.Server.Stop(); _ = listener.Close(); <-done })
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	orders, refunds := orderv1.NewOrderServiceClient(conn), refundv1.NewRefundServiceClient(conn)
	payments := paymentv1.NewPaymentServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	userToken, _, _ := user.Issue(1, time.Now())
	adminToken, _, _ := admin.Issue(1, time.Now())
	strangerToken, _, _ := newSigner(t, "goframeshop-user").Issue(1, time.Now())
	as := map[string]context.Context{
		"nobody":   ctx,
		"stranger": metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+strangerToken),
		"user":     metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+userToken),
		"admin":    metadata.AppendToOutgoingContext(ctx, utility.AuthorizationMetadataKey, "Bearer "+adminToken),
		"legacy":   metadata.AppendToOutgoingContext(ctx, "x-user-id", "1"),
	}
	// Empty requests: validation refuses them once authentication passes.
	shopper := map[string]func(context.Context) error{
		"CreateOrder": func(c context.Context) error {
			_, err := orders.CreateOrder(c, &orderv1.CreateOrderRequest{})
			return err
		},
		"CancelOrder": func(c context.Context) error {
			_, err := orders.CancelOrder(c, &orderv1.CancelOrderRequest{})
			return err
		},
		"GetMyOrder": func(c context.Context) error {
			_, err := orders.GetMyOrder(c, &orderv1.GetMyOrderRequest{})
			return err
		},
		"ListMyOrders": func(c context.Context) error {
			_, err := orders.ListMyOrders(c, &orderv1.ListMyOrdersRequest{})
			return err
		},
		"CreateRefund": func(c context.Context) error {
			_, err := refunds.CreateRefund(c, &refundv1.CreateRefundRequest{})
			return err
		},
		"GetMyRefund": func(c context.Context) error {
			_, err := refunds.GetMyRefund(c, &refundv1.GetMyRefundRequest{})
			return err
		},
		"ListMyRefunds": func(c context.Context) error {
			_, err := refunds.ListMyRefunds(c, &refundv1.ListMyRefundsRequest{})
			return err
		},
		"PayOrder": func(c context.Context) error {
			_, err := payments.PayOrder(c, &paymentv1.PayOrderRequest{})
			return err
		},
		"ConfirmReceipt": func(c context.Context) error {
			_, err := orders.ConfirmReceipt(c, &orderv1.ConfirmReceiptRequest{})
			return err
		},
	}
	public := map[string]func(context.Context) error{
		"NotifyPayment": func(c context.Context) error {
			_, err := payments.NotifyPayment(c, &paymentv1.NotifyPaymentRequest{})
			return err
		},
	}
	administrator := map[string]func(context.Context) error{
		"ListOrders": func(c context.Context) error {
			_, err := orders.ListOrders(c, &orderv1.ListOrdersRequest{})
			return err
		},
		"GetOrder": func(c context.Context) error { _, err := orders.GetOrder(c, &orderv1.GetOrderRequest{}); return err },
		"ListRefunds": func(c context.Context) error {
			_, err := refunds.ListRefunds(c, &refundv1.ListRefundsRequest{})
			return err
		},
		"ShipOrder": func(c context.Context) error {
			_, err := orders.ShipOrder(c, &orderv1.ShipOrderRequest{})
			return err
		},
		"ApproveRefund": func(c context.Context) error {
			_, err := refunds.ApproveRefund(c, &refundv1.ApproveRefundRequest{})
			return err
		},
		"RejectRefund": func(c context.Context) error {
			_, err := refunds.RejectRefund(c, &refundv1.RejectRefundRequest{})
			return err
		},
	}
	check := func(group map[string]func(context.Context) error, want map[string]codes.Code) {
		for name, rpc := range group {
			for who, code := range want {
				if got := status.Code(rpc(as[who])); got != code {
					t.Errorf("%s as %s: got %s, want %s", name, who, got, code)
				}
			}
		}
	}
	check(shopper, map[string]codes.Code{
		"nobody": codes.Unauthenticated, "stranger": codes.Unauthenticated, "legacy": codes.Unauthenticated,
		"user": codes.InvalidArgument, "admin": codes.PermissionDenied,
	})
	// Anyone reaches the handler; the empty request fails validation there.
	check(public, map[string]codes.Code{
		"nobody": codes.InvalidArgument, "user": codes.InvalidArgument, "admin": codes.InvalidArgument,
	})
	check(administrator, map[string]codes.Code{
		"nobody": codes.Unauthenticated, "stranger": codes.Unauthenticated, "legacy": codes.Unauthenticated,
		"user": codes.PermissionDenied, "admin": codes.InvalidArgument,
	})
}
