package main

import (
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/hughtaiyaki/goframeshop/app/job-template/internal/cmd"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
