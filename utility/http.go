package utility

import (
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
	"google.golang.org/grpc/codes"
)

// MiddlewareCORS answers cross-origin browser requests. A preflight OPTIONS
// request is answered here and never reaches business handlers.
func MiddlewareCORS(r *ghttp.Request) {
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")
	r.Response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	r.Response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		r.Response.WriteHeader(204)
		return
	}
	r.Middleware.Next()
}

// codeUnauthenticated reuses the gRPC code number, so the response envelope
// carries the same code whether the gateway or a backend refused the token.
// A client needs exactly one "log in again" signal.
var codeUnauthenticated = gcode.New(int(codes.Unauthenticated), "Unauthenticated", nil)

// MiddlewareAuth verifies the bearer token at the gateway with the public key.
//
// It is deny-by-default: every route behind it needs a valid token except the
// "METHOD /path" entries in publicRoutes, which are matched against the
// registered route pattern. On a public route a valid token is still honoured,
// and a missing or invalid one is ignored.
//
// Backends verify the token again. This check exists so a request without a
// valid token never costs a network round trip, not because it is the only one.
func MiddlewareAuth(verifier *TokenVerifier, publicRoutes ...string) ghttp.HandlerFunc {
	public := make(map[string]struct{}, len(publicRoutes))
	for _, route := range publicRoutes {
		public[route] = struct{}{}
	}
	return func(r *ghttp.Request) {
		_, isPublic := public[routeKey(r)]
		token, ok := BearerToken(r.Header.Get("Authorization"))
		if !ok {
			if !isPublic {
				r.SetError(gerror.NewCode(codeUnauthenticated, "missing bearer token"))
				return
			}
			r.Middleware.Next()
			return
		}
		userID, err := verifier.Parse(token, time.Now())
		if err != nil {
			if !isPublic {
				r.SetError(gerror.NewCode(codeUnauthenticated, "invalid access token"))
				return
			}
			r.Middleware.Next()
			return
		}
		r.SetCtx(ContextWithUserID(r.GetCtx(), userID))
		r.Middleware.Next()
	}
}

func routeKey(r *ghttp.Request) string {
	uri := r.URL.Path
	if r.Router != nil {
		uri = r.Router.Uri
	}
	return strings.ToUpper(r.Method) + " " + uri
}
