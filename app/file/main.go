package main

import (
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/hughtaiyaki/goframeshop/app/file/internal/cmd"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

func main() {
	// Tracing first, so everything after it is traced; ctx is the quiet
	// context for the process's own background loops (see telemetry.Quiet).
	ctx, stopTracing := telemetry.Start(gctx.GetInitCtx(), "file-service")
	defer stopTracing()
	// Service discovery has to be configured before the server starts.
	utility.RegisterEtcdResolver(ctx)
	cmd.Main.Run(ctx)
}
