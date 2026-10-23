package cmd

import (
	"context"
	"time"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"

	categoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/category/v1"
	goodsv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/goods/v1"
	inventoryv1 "github.com/hughtaiyaki/goframeshop/app/goods/api/inventory/v1"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/controller/category"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/controller/goods"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/controller/inventory"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/goods/internal/logic"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/eventbus"
	"github.com/hughtaiyaki/goframeshop/utility/rabbitmq"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start goods grpc server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// The ADMIN public key: only administrators may change the
			// catalogue. Reads are public, so shop users need no token here.
			admin, err := utility.LoadTokenVerifier(ctx)
			if err != nil {
				return err
			}
			// The ORDER service's public key, for its service tokens on the
			// inventory RPCs. Optional: without it only administrators can
			// call them (scripts/inventory).
			var order *utility.TokenVerifier
			if g.Cfg().MustGet(ctx, "jwtOrder").IsEmpty() {
				g.Log().Warning(ctx, "jwtOrder is not configured: the order service cannot reserve stock")
			} else if order, err = utility.LoadTokenVerifierAt(ctx, "jwtOrder"); err != nil {
				return err
			}
			if err := logic.Init(ctx); err != nil {
				return err
			}
			// Publish goods.changed events written by the catalogue writes.
			amqpURL, err := rabbitmq.LoadURL(ctx)
			if err != nil {
				return err
			}
			go eventbus.NewRelay(dao.GoodsInfo.DB(), rabbitmq.NewPublisher(amqpURL, 5*time.Second), eventbus.RelaySettings{
				Interval: g.Cfg().MustGet(ctx, "outbox.interval", "500ms").Duration(),
				Lease:    15 * time.Second,
				Batch:    100,
			}, time.Now).Run(ctx)
			newServer(admin, order).Run()
			return nil
		},
	}
)

// publicMethods are the reads. Every other RPC is administrator-only unless
// listed in serviceMethods, including RPCs added later.
var publicMethods = []string{
	goodsv1.GoodsService_ListGoods_FullMethodName,
	goodsv1.GoodsService_GetGoods_FullMethodName,
	categoryv1.CategoryService_ListCategories_FullMethodName,
	categoryv1.CategoryService_ListAllCategories_FullMethodName,
}

// serviceMethods are the inventory RPCs: the order service calls them with its
// own service token, and administrators may call them to debug.
var serviceMethods = []string{
	inventoryv1.InventoryService_ReserveInventory_FullMethodName,
	inventoryv1.InventoryService_CommitInventory_FullMethodName,
	inventoryv1.InventoryService_ReleaseInventory_FullMethodName,
	inventoryv1.InventoryService_QueryInventoryOperation_FullMethodName,
}

func newServer(admin, order *utility.TokenVerifier) *grpcx.GrpcServer {
	policy := utility.AuthPolicy{
		Verifiers:    map[utility.Role]*utility.TokenVerifier{utility.RoleAdmin: admin},
		Public:       publicMethods,
		Methods:      map[string][]utility.Role{},
		DefaultRoles: []utility.Role{utility.RoleAdmin},
	}
	if order != nil {
		policy.Verifiers[utility.RoleService] = order
	}
	for _, method := range serviceMethods {
		policy.Methods[method] = []utility.Role{utility.RoleAdmin, utility.RoleService}
	}
	s := grpcx.Server.New(utility.NewGrpcServerConfigWithPolicy(policy))
	goods.Register(s)
	category.Register(s)
	inventory.Register(s)
	return s
}
