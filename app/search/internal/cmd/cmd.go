package cmd

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"

	v1 "github.com/hughtaiyaki/goframeshop/app/search/api/search/v1"
	"github.com/hughtaiyaki/goframeshop/app/search/internal/controller/search"
	"github.com/hughtaiyaki/goframeshop/app/search/internal/index"
	searchlogic "github.com/hughtaiyaki/goframeshop/app/search/internal/logic/search"
	"github.com/hughtaiyaki/goframeshop/app/search/internal/service"
	"github.com/hughtaiyaki/goframeshop/app/search/internal/source"
	"github.com/hughtaiyaki/goframeshop/app/search/internal/syncer"
	"github.com/hughtaiyaki/goframeshop/utility"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start search grpc server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			es, err := index.New(
				g.Cfg().MustGet(ctx, "elasticsearch.addresses").Strings(),
				g.Cfg().MustGet(ctx, "elasticsearch.alias", "goods").String(),
			)
			if err != nil {
				return err
			}
			if err := es.Ping(ctx); err != nil {
				return gerror.Wrap(err, "elasticsearch is not reachable")
			}
			var binlog syncer.Config
			if err := g.Cfg().MustGet(ctx, "binlog").Scan(&binlog); err != nil {
				return gerror.Wrap(err, "read binlog config")
			}
			// The rebuild runs before the server starts, so the first search
			// already sees a complete index.
			if err := syncer.New(es, source.New(g.DB("goods_source")), binlog).Start(ctx); err != nil {
				return err
			}
			newServer(searchlogic.New(es)).Run()
			return nil
		},
	}
)

// publicMethods: everything. The search service holds no key and accepts no
// token; any RPC added here later must be listed to be reachable at all.
var publicMethods = []string{
	v1.SearchService_SearchGoods_FullMethodName,
}

func newServer(s service.ISearch) *grpcx.GrpcServer {
	server := grpcx.Server.New(utility.NewGrpcServerConfig(nil, publicMethods...))
	search.Register(server, s)
	return server
}
