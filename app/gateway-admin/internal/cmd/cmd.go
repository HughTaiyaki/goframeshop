package cmd

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"google.golang.org/grpc/codes"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/internal/controller/admin"
	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/internal/controller/banner"
	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/internal/controller/file"
	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/internal/controller/goods"
	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/internal/controller/order"
	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/internal/storage"
	"github.com/hughtaiyaki/goframeshop/utility"
)

// publicRoutes are the only back-office routes reachable without a token.
// Everything else under /backend needs an administrator token, including
// routes added later.
var publicRoutes = []string{
	"POST /backend/admin/login",
}

// multipartOverhead is the slack allowed on top of the image itself for the
// multipart boundaries and part headers.
const multipartOverhead = 1 << 20

// codeInvalidArgument reuses the gRPC code number, like every other client
// error this project returns.
var codeInvalidArgument = gcode.New(int(codes.InvalidArgument), "InvalidArgument", nil)

// middlewareBodyLimit refuses an oversized body from its Content-Length,
// before anything parses it. Without this, GoFrame's multipart parser hits the
// server limit mid-parse and answers with a plain-text 500 that names its own
// internals. A chunked body without a length still stops at the server limit.
func middlewareBodyLimit(maxBytes int64) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		if r.ContentLength > maxBytes {
			r.SetError(gerror.NewCode(codeInvalidArgument, fmt.Sprintf("request body must be at most %d bytes", maxBytes)))
			return
		}
		r.Middleware.Next()
	}
}

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start gateway-admin http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// Everything that can be misconfigured is checked here, before any
			// route is bound, so a bad setting stops startup with a clear
			// error instead of a panic inside a controller constructor.
			//
			// The ADMIN public key: a shop user's token is signed with a
			// different key and is rejected here.
			verifier, err := utility.LoadTokenVerifier(ctx)
			if err != nil {
				return err
			}
			store, err := storage.New(ctx)
			if err != nil {
				return err
			}
			maxImageBytes, err := g.Cfg().Get(ctx, "upload.maxImageBytes", storage.DefaultMaxImageBytes)
			if err != nil {
				return gerror.Wrap(err, "read upload.maxImageBytes")
			}

			// One source of truth for the size limit: the body may be the image
			// plus room for the multipart framing, and nothing more.
			maxBodyBytes := maxImageBytes.Int64() + multipartOverhead

			s := g.Server()
			s.SetClientMaxBodySize(maxBodyBytes)
			s.Use(utility.MiddlewareCORS)
			// Local storage is served by the gateway itself. It is a plain
			// static directory: it bypasses the auth middleware on purpose,
			// because images are shown to shoppers.
			if local, ok := store.(*storage.Local); ok {
				s.AddStaticPath("/files", local.Dir())
			}
			s.Group("/", func(group *ghttp.RouterGroup) {
				group.Middleware(ghttp.MiddlewareHandlerResponse)
				group.Group("/backend", func(group *ghttp.RouterGroup) {
					group.Middleware(
						utility.MiddlewareAuth(verifier, publicRoutes...),
						middlewareBodyLimit(maxBodyBytes),
					)
					group.Bind(
						admin.NewV1(),
						file.NewV1(),
						goods.NewV1(),
						order.NewV1(),
						banner.NewV1(),
					)
				})
			})
			g.Log().Infof(ctx, "image storage: %s", store.Name())
			s.Run()
			return nil
		},
	}
)
