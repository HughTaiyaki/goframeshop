package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"

	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/checkout"
	providercontroller "github.com/hughtaiyaki/goframeshop/app/paysim/internal/controller/provider"
	providerlogic "github.com/hughtaiyaki/goframeshop/app/paysim/internal/logic/provider"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/notify"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/paysim/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start the payment simulator: merchant gRPC API, checkout page, notifier",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// The merchant is the order service: its service token, checked
			// with the PUBLIC key of its own pair (the same one goods uses).
			merchant, err := utility.LoadTokenVerifierAt(ctx, "jwtOrder")
			if err != nil {
				return err
			}
			// The simulator's own PRIVATE key signs notifications; the order
			// service holds the public half.
			signer, err := utility.LoadNoticeSignerAt(ctx, "notify")
			if err != nil {
				return err
			}
			settings := notify.Settings{
				URL:         g.Cfg().MustGet(ctx, "notify.url").String(),
				Interval:    g.Cfg().MustGet(ctx, "notify.interval", "1s").Duration(),
				MaxAttempts: g.Cfg().MustGet(ctx, "notify.maxAttempts", 10).Int(),
			}
			if settings.URL == "" {
				return gerror.New("notify.url is required: the merchant's notify endpoint")
			}
			checkoutURL := strings.TrimRight(g.Cfg().MustGet(ctx, "checkout.baseURL", "http://127.0.0.1:8399").String(), "/")

			payments := repository.New()
			provider := providerlogic.New(payments, time.Now)

			web := g.Server("checkout")
			web.SetAddr(g.Cfg().MustGet(ctx, "checkout.address", ":8399").String())
			checkout.Register(web, provider)
			if err := web.Start(); err != nil {
				return err
			}
			go notify.New(payments, signer, settings, time.Now).Run(ctx)

			newServer(merchant, provider, checkoutURL).Run()
			return nil
		},
	}
)

// Every merchant RPC needs the order service's token.
func newServer(merchant *utility.TokenVerifier, provider service.IProvider, checkoutURL string) *grpcx.GrpcServer {
	policy := utility.AuthPolicy{
		Verifiers:    map[utility.Role]*utility.TokenVerifier{utility.RoleService: merchant},
		DefaultRoles: []utility.Role{utility.RoleService},
	}
	s := grpcx.Server.New(utility.NewGrpcServerConfigWithPolicy(policy))
	providercontroller.Register(s, provider, checkoutURL)
	return s
}
