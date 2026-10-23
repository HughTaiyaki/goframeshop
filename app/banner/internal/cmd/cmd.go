package cmd

import (
	"context"

	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/os/gcmd"

	positionv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/position/v1"
	rotationv1 "github.com/hughtaiyaki/goframeshop/app/banner/api/rotation/v1"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/clients"
	positioncontroller "github.com/hughtaiyaki/goframeshop/app/banner/internal/controller/position"
	rotationcontroller "github.com/hughtaiyaki/goframeshop/app/banner/internal/controller/rotation"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/dao"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/events"
	positionlogic "github.com/hughtaiyaki/goframeshop/app/banner/internal/logic/position"
	rotationlogic "github.com/hughtaiyaki/goframeshop/app/banner/internal/logic/rotation"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/repository"
	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
	"github.com/hughtaiyaki/goframeshop/utility"
	"github.com/hughtaiyaki/goframeshop/utility/rabbitmq"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start banner grpc server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// The ADMIN public key: only administrators change banners.
			verifier, err := utility.LoadTokenVerifier(ctx)
			if err != nil {
				return err
			}
			files := clients.NewFiles()
			rotations := rotationlogic.New(repository.NewRotation(), files)
			positions := positionlogic.New(repository.NewPosition(), files, clients.NewCatalog())
			// Keep slots in step with goods.changed events.
			amqpURL, err := rabbitmq.LoadURL(ctx)
			if err != nil {
				return err
			}
			go rabbitmq.Consume(ctx, amqpURL, rabbitmq.QueueBannerGoods, 16, events.NewGoods(dao.PositionInfo.DB()).Handle)

			newServer(verifier, rotations, positions).Run()
			return nil
		},
	}
)

// publicMethods: the home page reads banners without logging in. Every write
// needs an administrator token, including RPCs added later.
var publicMethods = []string{
	rotationv1.RotationService_ListRotations_FullMethodName,
	positionv1.PositionService_ListPositions_FullMethodName,
}

func newServer(verifier *utility.TokenVerifier, rotations service.IRotation, positions service.IPosition) *grpcx.GrpcServer {
	s := grpcx.Server.New(utility.NewGrpcServerConfig(verifier, publicMethods...))
	rotationcontroller.Register(s, rotations)
	positioncontroller.Register(s, positions)
	return s
}
