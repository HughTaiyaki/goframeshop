package main

import (
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/hughtaiyaki/goframeshop/app/order/internal/cmd"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

func main() {
	// Tracing first, so everything after it is traced; ctx is the quiet
	// context for the process's own background loops (see telemetry.Quiet).
	ctx, stopTracing := telemetry.Start(gctx.GetInitCtx(), "order-service")
	defer stopTracing()
	// Then service discovery: the order service both serves and calls others.
	utility.RegisterEtcdResolver(ctx)
	cmd.Main.Run(ctx)
}
