package cmd

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"

	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/internal/controller/banner"
	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/internal/controller/goods"
	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/internal/controller/interaction"
	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/internal/controller/order"
	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/internal/controller/search"
	"github.com/hughtaiyaki/goframeshop/app/gateway-h5/internal/controller/user"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// publicRoutes are the only routes reachable without a valid token, written
// as "METHOD /registered/path". Everything else under /frontend needs one,
// including routes added later.
var publicRoutes = []string{
	"POST /frontend/user/register",
	"POST /frontend/user/login",
	"GET /frontend/comment",
	// The catalogue is browsable without logging in.
	"GET /frontend/goods",
	"GET /frontend/goods/detail",
	"GET /frontend/category",
	"GET /frontend/category/all",
	"GET /frontend/search/goods",
	// Home-page banners.
	"GET /frontend/rotation",
	"GET /frontend/position",
	// The payment simulator's notification: no token, a signature instead,
	// checked by the order service.
	"POST /frontend/payment/notify",
}

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start gateway-h5 http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// The gateway holds the public key only: it can reject a bad
			// token early, but it cannot issue one.
			verifier, err := utility.LoadTokenVerifier(ctx)
			if err != nil {
				return err
			}
			s := g.Server()
			// CORS runs before routing so browser preflight requests are
			// answered even for paths that do not exist yet.
			s.Use(utility.MiddlewareCORS)
			s.Group("/", func(group *ghttp.RouterGroup) {
				// MiddlewareHandlerResponse wraps every result in a uniform
				// {code, message, data} envelope. It must come before the auth
				// middleware so an auth rejection is wrapped as well.
				group.Middleware(ghttp.MiddlewareHandlerResponse)
				group.Group("/frontend", func(group *ghttp.RouterGroup) {
					group.Middleware(utility.MiddlewareAuth(verifier, publicRoutes...))
					group.Bind(
						user.NewV1(),
						interaction.NewV1(),
						goods.NewV1(),
						search.NewV1(),
						order.NewV1(),
						banner.NewV1(),
					)
				})
			})
			s.Run()
			return nil
		},
	}
)
