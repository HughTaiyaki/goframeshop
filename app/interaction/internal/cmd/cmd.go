package cmd

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/os/gcmd"

	commentv1 "github.com/hughtaiyaki/goframeshop/app/interaction/api/comment/v1"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/controller/collection"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/controller/comment"
	"github.com/hughtaiyaki/goframeshop/app/interaction/internal/controller/praise"
	"github.com/hughtaiyaki/goframeshop/utility"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start interaction grpc server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			verifier, err := utility.LoadTokenVerifier(ctx)
			if err != nil {
				return err
			}
			newServer(verifier).Run()
			return nil
		},
	}
)

// publicMethods are callable without a token. Reading comments is public;
// every other RPC acts on the caller's own data and is denied by default.
var publicMethods = []string{
	commentv1.CommentService_ListComments_FullMethodName,
}

// newServer only verifies tokens: this service holds the public key and can
// never issue one.
func newServer(verifier *utility.TokenVerifier) *grpcx.GrpcServer {
	s := grpcx.Server.New(utility.NewGrpcServerConfig(verifier, publicMethods...))
	praise.Register(s)
	collection.Register(s)
	comment.Register(s)
	return s
}
