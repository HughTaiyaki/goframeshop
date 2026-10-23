package main

import (
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/hughtaiyaki/goframeshop/app/gateway-admin/internal/cmd"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

func main() {
	// Tracing first, so everything after it is traced; ctx is the quiet
	// context for the process's own background loops (see telemetry.Quiet).
	ctx, stopTracing := telemetry.Start(gctx.GetInitCtx(), "gateway-admin")
	defer stopTracing()
	// The gateway is a gRPC client: without the resolver it cannot turn the
	// service names "admin" and "file" into addresses.
	utility.RegisterEtcdResolver(ctx)
	cmd.Main.Run(ctx)
}
