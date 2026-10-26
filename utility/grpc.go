package utility

import (
	"context"
	"time"

	"github.com/gogf/gf/contrib/registry/etcd/v2"
	"github.com/gogf/gf/contrib/rpc/grpcx/v2"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

// DefaultRPCTimeout bounds every outbound gRPC call. gRPC has no default
// deadline, so without this a blocked downstream would hang the caller forever.
const DefaultRPCTimeout = 3 * time.Second

// GrpcClientTimeout is a unary client interceptor that adds a deadline.
func GrpcClientTimeout(ctx context.Context, method string, req, reply interface{},
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	return timedCall(ctx, DefaultRPCTimeout, method, req, reply, cc, invoker, opts...)
}

// timedCall bounds a client call by d and records it as
// trade_rpc_duration_seconds (design §9.2). Every gRPC client in the shop
// goes through GrpcClientTimeout or GrpcClientTimeoutOf, so every call
// between services is measured here, once.
func timedCall(ctx context.Context, d time.Duration, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	started := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)
	telemetry.RecordRPC(ctx, telemetry.PeerName(cc.Target()), method, telemetry.RPCOutcome(RPCCode(err)), time.Since(started))
	return err
}

// RegisterEtcdResolver wires service discovery for both directions: a server
// publishes itself under the name in its grpc config, and a client resolves
// that name to live addresses.
//
// Call it before starting a server or dialling a client. An empty address is a
// warning, not a crash, so a single service still runs without etcd.
func RegisterEtcdResolver(ctx context.Context) {
	address := g.Cfg().MustGet(ctx, "etcd.address").String()
	if address == "" {
		g.Log().Warning(ctx, "etcd.address is empty, falling back to the local file registry")
		return
	}
	grpcx.Resolver.Register(etcd.New(address))
	g.Log().Infof(ctx, "etcd registry enabled at %s", address)
}

// UnarySafeLog logs one line per call: method, status code and duration.
//
// grpcx's built-in access and error logs print the full request, which would
// put passwords and comment bodies into the log. Services turn those off and
// use this instead.
func UnarySafeLog(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	started := time.Now()
	response, err := handler(ctx, req)
	code := status.Code(err)
	if gfCode := gerror.Code(err).Code(); gfCode != -1 {
		code = codes.Code(gfCode)
	}
	g.Log().Infof(ctx, "grpc method=%s code=%s duration=%s", info.FullMethod, code, time.Since(started))
	return response, err
}

// NewGrpcServerConfig returns the server config every single-key service in
// this repo uses: safe logging, then authentication, then request validation.
//
// Order matters. Every outcome is logged, and an unauthenticated caller is
// refused before learning anything about the validation rules.
func NewGrpcServerConfig(verifier *TokenVerifier, publicMethods ...string) *grpcx.GrpcServerConfig {
	config := grpcx.Server.NewConfig()
	config.AccessLogEnabled = false
	config.ErrorLogEnabled = false
	config.Options = append(config.Options, grpcx.Server.ChainUnary(
		UnarySafeLog,
		UnaryServerAuth(verifier, publicMethods...),
		grpcx.Server.UnaryValidate,
	))
	return config
}

// RPCCode returns the gRPC code of an error from a downstream call.
//
// grpcx's client interceptor rewrites a gRPC status into a gerror carrying the
// same number, so status.Code alone reports Unknown for every error coming
// back through a grpcx connection. This reads both forms.
func RPCCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if code := gerror.Code(err).Code(); code > 0 && code <= int(codes.Unauthenticated) {
		return codes.Code(code)
	}
	return status.Code(err)
}

// NewGrpcServerConfigWithPolicy is NewGrpcServerConfig for a server that
// trusts more than one kind of caller (see AuthPolicy).
func NewGrpcServerConfigWithPolicy(policy AuthPolicy) *grpcx.GrpcServerConfig {
	config := grpcx.Server.NewConfig()
	config.AccessLogEnabled = false
	config.ErrorLogEnabled = false
	config.Options = append(config.Options, grpcx.Server.ChainUnary(
		UnarySafeLog,
		UnaryServerAuthPolicy(policy),
		grpcx.Server.UnaryValidate,
	))
	return config
}

// GrpcClientTimeoutOf is GrpcClientTimeout with a chosen deadline, for calls
// whose budget differs from the default (the inventory calls use 800ms).
func GrpcClientTimeoutOf(d time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		return timedCall(ctx, d, method, req, reply, cc, invoker, opts...)
	}
}
