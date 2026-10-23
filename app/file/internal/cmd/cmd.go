package cmd

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/os/gcmd"

	"github.com/hughtaiyaki/goframeshop/app/file/internal/controller/file"
	"github.com/hughtaiyaki/goframeshop/utility"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start file grpc server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// The verifier holds the ADMIN public key: only back-office
			// administrators can record or read files for now.
			verifier, err := utility.LoadTokenVerifier(ctx)
			if err != nil {
				return err
			}
			newServer(verifier).Run()
			return nil
		},
	}
)

// newServer has no public methods: every call needs an administrator token.
func newServer(verifier *utility.TokenVerifier) *grpcx.GrpcServer {
	s := grpcx.Server.New(utility.NewGrpcServerConfig(verifier))
	file.Register(s)
	return s
}
