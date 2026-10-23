// Package clients implements the order service's calls to other services.
//
//	Catalog    goods.GetGoods          public; no token
//	Addresses  user.GetConsignee       the shopper's own token, forwarded
//	Inventory  goods.InventoryService  the order service's own service token
//	Provider   paysim.PaymentProvider  the order service's own service token
package clients

import (
	"context"
	"time"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"
	inventoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
	"github.com/hughtaiyaki/goframeshop/app/order/internal/service"
	providerv1 "github.com/hughtaiyaki/goframeshop/app/paysim/api/provider/v1"
	consigneev1 "github.com/hughtaiyaki/goframeshop/app/user/api/consignee/v1"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// InventoryTimeout is the per-call budget for inventory RPCs (design §7.2:
// 800ms). Running out is not a failure of the order: it stays where it is
// and the recovery worker retries with the same operation id.
const InventoryTimeout = 800 * time.Millisecond

type Catalog struct{ goods goodsv1.GoodsServiceClient }

func NewCatalog() *Catalog {
	conn := grpcx.Client.MustNewGrpcClientConn("goods", grpcx.Client.ChainUnary(utility.GrpcClientTimeout))
	return &Catalog{goods: goodsv1.NewGoodsServiceClient(conn)}
}

func (c *Catalog) Goods(ctx context.Context, goodsID uint64) (*service.GoodsSnapshot, error) {
	res, err := c.goods.GetGoods(ctx, &goodsv1.GetGoodsRequest{GoodsId: goodsID})
	if utility.RPCCode(err) == codes.NotFound {
		return nil, service.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g := res.GetGoods()
	snapshot := &service.GoodsSnapshot{Name: g.GetName(), PriceCents: g.GetPriceCents()}
	if images := g.GetImages(); len(images) > 0 {
		snapshot.CoverURL = images[0].GetUrl()
	}
	return snapshot, nil
}

type Addresses struct {
	consignees consigneev1.ConsigneeServiceClient
}

func NewAddresses() *Addresses {
	conn := grpcx.Client.MustNewGrpcClientConn("user", grpcx.Client.ChainUnary(
		utility.GrpcClientTimeout,
		utility.GrpcClientForwardAuth,
	))
	return &Addresses{consignees: consigneev1.NewConsigneeServiceClient(conn)}
}

func (a *Addresses) Consignee(ctx context.Context, consigneeID uint64) (*service.Address, error) {
	res, err := a.consignees.GetConsignee(ctx, &consigneev1.GetConsigneeRequest{ConsigneeId: consigneeID})
	if utility.RPCCode(err) == codes.NotFound {
		return nil, service.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c := res.GetConsignee()
	return &service.Address{
		Name:    c.GetName(),
		Phone:   c.GetPhone(),
		Address: c.GetProvince() + c.GetCity() + c.GetTown() + c.GetStreet() + " " + c.GetDetail(),
	}, nil
}

type Inventory struct {
	inventory inventoryv1.InventoryServiceClient
}

func NewInventory(token *utility.ServiceToken) *Inventory {
	conn := grpcx.Client.MustNewGrpcClientConn("goods", grpcx.Client.ChainUnary(
		utility.GrpcClientTimeoutOf(InventoryTimeout),
		token.Interceptor,
	))
	return &Inventory{inventory: inventoryv1.NewInventoryServiceClient(conn)}
}

func (i *Inventory) Reserve(ctx context.Context, operationID, orderNo string, items []service.ItemInput) (string, error) {
	res, err := i.inventory.ReserveInventory(ctx, &inventoryv1.ReserveInventoryRequest{
		OperationId: operationID, OrderId: orderNo, Items: toItems(items),
	})
	if err != nil {
		return "", err
	}
	return res.GetOriginalOutcome().String(), nil
}

func (i *Inventory) Commit(ctx context.Context, operationID, orderNo string) (string, error) {
	res, err := i.inventory.CommitInventory(ctx, &inventoryv1.CommitInventoryRequest{
		OperationId: operationID, OrderId: orderNo,
	})
	if err != nil {
		return "", err
	}
	return res.GetOriginalOutcome().String(), nil
}

func (i *Inventory) Release(ctx context.Context, operationID, orderNo string, items []service.ItemInput) (string, error) {
	res, err := i.inventory.ReleaseInventory(ctx, &inventoryv1.ReleaseInventoryRequest{
		OperationId: operationID, OrderId: orderNo, Items: toItems(items),
	})
	if err != nil {
		return "", err
	}
	return res.GetOriginalOutcome().String(), nil
}

func toItems(items []service.ItemInput) []*inventoryv1.InventoryItem {
	out := make([]*inventoryv1.InventoryItem, 0, len(items))
	for _, item := range items {
		out = append(out, &inventoryv1.InventoryItem{GoodsId: item.GoodsID, Quantity: item.Quantity})
	}
	return out
}

// ProviderTimeout is the per-call budget for the payment provider. As with
// inventory, running out only means "ask again later".
const ProviderTimeout = 2 * time.Second

type Provider struct {
	provider providerv1.PaymentProviderClient
}

func NewProvider(token *utility.ServiceToken) *Provider {
	conn := grpcx.Client.MustNewGrpcClientConn("paysim", grpcx.Client.ChainUnary(
		utility.GrpcClientTimeoutOf(ProviderTimeout),
		token.Interceptor,
	))
	return &Provider{provider: providerv1.NewPaymentProviderClient(conn)}
}

func (p *Provider) Create(ctx context.Context, paymentID, orderNo string, amountCents uint64, expiresAt time.Time) (*service.ProviderPayment, error) {
	res, err := p.provider.CreatePayment(ctx, &providerv1.CreatePaymentRequest{
		PaymentId: paymentID, OrderNo: orderNo, AmountCents: amountCents,
		Currency: service.Currency, ExpiresAt: timestamppb.New(expiresAt),
	})
	return toPayment(res, err)
}

func (p *Provider) Close(ctx context.Context, paymentID, orderNo string) (*service.ProviderPayment, error) {
	res, err := p.provider.ClosePayment(ctx, &providerv1.ClosePaymentRequest{PaymentId: paymentID, OrderNo: orderNo})
	return toPayment(res, err)
}

func (p *Provider) Query(ctx context.Context, paymentID string) (*service.ProviderPayment, error) {
	res, err := p.provider.QueryPayment(ctx, &providerv1.QueryPaymentRequest{PaymentId: paymentID})
	if utility.RPCCode(err) == codes.NotFound {
		return nil, service.ErrNotFound
	}
	return toPayment(res, err)
}

func (p *Provider) Refund(ctx context.Context, refundID, paymentID string, amountCents uint64) error {
	_, err := p.provider.RefundPayment(ctx, &providerv1.RefundPaymentRequest{
		RefundId: refundID, PaymentId: paymentID, AmountCents: amountCents,
	})
	return providerError(err)
}

// providerError keeps the provider refusing our service token from reaching
// the shopper as "Unauthenticated": that is our misconfiguration, not theirs.
func providerError(err error) error {
	switch utility.RPCCode(err) {
	case codes.Unauthenticated, codes.PermissionDenied:
		// A new error, not a wrap: a wrap keeps the code, and the caller
		// would still see Unauthenticated.
		return gerror.NewCodef(gcode.CodeInternalError,
			"payment provider refused the order service token (check jwtOrder in the paysim config): %v", err)
	}
	return err
}

func toPayment(res *providerv1.PaymentResponse, err error) (*service.ProviderPayment, error) {
	if err != nil {
		return nil, providerError(err)
	}
	p := res.GetPayment()
	out := &service.ProviderPayment{
		ID: p.GetPaymentId(), OrderNo: p.GetOrderNo(), AmountCents: p.GetAmountCents(),
		Currency: p.GetCurrency(), State: p.GetState().String(), TxID: p.GetProviderTxId(), PayURL: p.GetPayUrl(),
	}
	if p.ExpiresAt != nil {
		t := p.ExpiresAt.AsTime()
		out.ExpiresAt = &t
	}
	if p.PaidAt != nil {
		t := p.PaidAt.AsTime()
		out.PaidAt = &t
	}
	return out, nil
}
