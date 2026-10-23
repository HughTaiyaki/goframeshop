package utility

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type envelope struct {
	Code int `json:"code"`
	Data struct {
		UserID    uint64 `json:"userId"`
		Forwarded string `json:"forwarded"`
	} `json:"data"`
}

// startGateway runs a real GoFrame server with the same middleware order as
// gateway-h5, so route matching and the response envelope are exercised.
func startGateway(t *testing.T, verifier *TokenVerifier) string {
	t.Helper()
	s := g.Server(guid.S())
	s.SetDumpRouterMap(false)
	s.SetAddr("127.0.0.1:0")
	s.Use(MiddlewareCORS)
	handler := func(r *ghttp.Request) {
		userID, _ := UserIDFromContext(r.Context())
		// Run the forward-auth client interceptor with a fake invoker to see
		// what a backend would receive.
		var forwarded string
		_ = GrpcClientForwardAuth(r.Context(), "/svc/M", nil, nil, nil,
			func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
				md, _ := metadata.FromOutgoingContext(ctx)
				if values := md.Get(AuthorizationMetadataKey); len(values) > 0 {
					forwarded = values[0]
				}
				return nil
			})
		r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"userId": userID, "forwarded": forwarded}})
	}
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Group("/frontend", func(group *ghttp.RouterGroup) {
			group.Middleware(MiddlewareAuth(verifier, "GET /frontend/open"))
			group.GET("/open", handler)
			group.GET("/closed", handler)
			group.POST("/open", handler) // same path, different method: protected
		})
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown() })
	return fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())
}

func TestMiddlewareAuth(t *testing.T) {
	signer := newTestSigner(t)
	base := startGateway(t, signer.Verifier())
	token, _, err := signer.Issue(42, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	forged, _, err := newTestSigner(t).Issue(42, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	do := func(method, path, authorization string) envelope {
		t.Helper()
		req, err := http.NewRequest(method, base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasPrefix(authorization, "X-User-Id:"):
			req.Header.Set("X-User-Id", strings.TrimSpace(strings.TrimPrefix(authorization, "X-User-Id:")))
		case authorization != "":
			req.Header.Set("Authorization", authorization)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		var out envelope
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("%s %s: bad body %q", method, path, body)
		}
		return out
	}

	tests := []struct {
		name          string
		method, path  string
		authorization string
		wantCode      int
		wantUserID    uint64
		wantForwarded string
	}{
		{"protected without token", "GET", "/frontend/closed", "", 16, 0, ""},
		{"protected with forged token", "GET", "/frontend/closed", "Bearer " + forged, 16, 0, ""},
		{"protected with legacy header only", "GET", "/frontend/closed", "X-User-Id: 42", 16, 0, ""},
		{"protected with valid token", "GET", "/frontend/closed", "Bearer " + token, 0, 42, "Bearer " + token},
		{"public without token", "GET", "/frontend/open", "", 0, 0, ""},
		{"public with forged token", "GET", "/frontend/open", "Bearer " + forged, 0, 0, "Bearer " + forged},
		{"public with valid token", "GET", "/frontend/open", "Bearer " + token, 0, 42, "Bearer " + token},
		{"public path, protected method", "POST", "/frontend/open", "", 16, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := do(tt.method, tt.path, tt.authorization)
			if got.Code != tt.wantCode || got.Data.UserID != tt.wantUserID || got.Data.Forwarded != tt.wantForwarded {
				t.Fatalf("got code=%d user=%d forwarded=%q; want code=%d user=%d forwarded=%q",
					got.Code, got.Data.UserID, got.Data.Forwarded, tt.wantCode, tt.wantUserID, tt.wantForwarded)
			}
		})
	}
}
