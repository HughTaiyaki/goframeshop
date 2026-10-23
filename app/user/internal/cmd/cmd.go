package cmd

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/os/gcmd"

	userv1 "github.com/hughtaiyaki/goframeshop/app/user/api/user/v1"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/controller/consignee"
	"github.com/hughtaiyaki/goframeshop/app/user/internal/controller/user"
	"github.com/hughtaiyaki/goframeshop/utility"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start user grpc server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			signer, err := utility.LoadTokenSigner(ctx)
			if err != nil {
				return err
			}
			newServer(signer).Run()
			return nil
		},
	}
)

// publicMethods are the only RPCs callable without a token: you cannot hold a
// token before you have registered and logged in. Everything else is denied
// unless authenticated, including RPCs added later.
var publicMethods = []string{
	userv1.UserService_RegisterUser_FullMethodName,
	userv1.UserService_Login_FullMethodName,
}

func newServer(signer *utility.TokenSigner) *grpcx.GrpcServer {
	s := grpcx.Server.New(utility.NewGrpcServerConfig(signer.Verifier(), publicMethods...))
	user.Register(s, signer)
	consignee.Register(s)
	return s
}
