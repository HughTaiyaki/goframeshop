package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gcmd"

	v1 "github.com/hughtaiyaki/goframeshop/app/admin/api/admin/v1"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/controller/admin"
	"github.com/hughtaiyaki/goframeshop/app/admin/internal/logic"
	"github.com/hughtaiyaki/goframeshop/utility"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start admin grpc server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			signer, err := utility.LoadTokenSigner(ctx)
			if err != nil {
				return err
			}
			newServer(signer).Run()
			return nil
		},
	}

	// CreateAdmin bootstraps a super administrator. There is no public
	// registration endpoint, so this is how the first account comes to exist.
	// The password is read from ADMIN_PASSWORD or stdin, never from argv,
	// which would leave it in shell history and the process list.
	CreateAdmin = &gcmd.Command{
		Name:  "create-admin",
		Usage: "create-admin -name NAME",
		Brief: "create a super administrator (password from ADMIN_PASSWORD or stdin)",
		Arguments: []gcmd.Argument{
			{Name: "name", Short: "n", Brief: "administrator name"},
		},
		Func: func(ctx context.Context, parser *gcmd.Parser) error {
			name := parser.GetOpt("name").String()
			if name == "" {
				return gerror.New("-name is required")
			}
			password := os.Getenv("ADMIN_PASSWORD")
			if password == "" {
				fmt.Fprint(os.Stderr, "password: ")
				line, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil && line == "" {
					return gerror.New("no password on stdin")
				}
				password = strings.TrimRight(line, "\r\n")
			}
			id, err := logic.Admin().Bootstrap(ctx, name, password)
			if err != nil {
				return err
			}
			fmt.Printf("created super administrator %q with id %d\n", name, id)
			return nil
		},
	}
)

func init() {
	if err := Main.AddCommand(CreateAdmin); err != nil {
		panic(err)
	}
}

// publicMethods: logging in is the only thing possible without a token.
var publicMethods = []string{
	v1.AdminService_Login_FullMethodName,
}

func newServer(signer *utility.TokenSigner) *grpcx.GrpcServer {
	s := grpcx.Server.New(utility.NewGrpcServerConfig(signer.Verifier(), publicMethods...))
	admin.Register(s, signer)
	return s
}
