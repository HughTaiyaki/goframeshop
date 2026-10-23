package utility

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var testNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func newTestSigner(t *testing.T) *TokenSigner {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewTokenSigner(privateKey.Seed(), defaultIssuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func signClaims(t *testing.T, method jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// useConfig swaps the global config for an in-memory one for one test.
func useConfig(t *testing.T, content string) {
	t.Helper()
	adapter, err := gcfg.NewAdapterContent(content)
	if err != nil {
		t.Fatal(err)
	}
	previous := g.Cfg().GetAdapter()
	g.Cfg().SetAdapter(adapter)
	t.Cleanup(func() { g.Cfg().SetAdapter(previous) })
}

func TestAccessTokenRoundTrip(t *testing.T) {
	signer := newTestSigner(t)
	token, expiresAt, err := signer.Issue(42, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if want := testNow.Add(time.Hour); !expiresAt.Equal(want) {
		t.Fatalf("expiresAt = %s, want %s", expiresAt, want)
	}
	userID, err := signer.Verifier().Parse(token, testNow.Add(59*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if userID != 42 {
		t.Fatalf("userID = %d, want 42", userID)
	}
}

// Every way a token can be wrong must collapse into the same error, so neither
// the caller nor a log reader can tell a forged token from an expired one.
func TestParseRejects(t *testing.T) {
	signer := newTestSigner(t)
	verifier := signer.Verifier()
	other := newTestSigner(t)

	valid := jwt.RegisteredClaims{
		Issuer:    defaultIssuer,
		Subject:   "42",
		ExpiresAt: jwt.NewNumericDate(testNow.Add(time.Hour)),
	}
	withClaims := func(edit func(*jwt.RegisteredClaims)) jwt.RegisteredClaims {
		claims := valid
		edit(&claims)
		return claims
	}
	issued, _, err := signer.Issue(42, testNow)
	if err != nil {
		t.Fatal(err)
	}
	fromOtherKey, _, err := other.Issue(42, testNow)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
		now   time.Time
	}{
		{"expired", issued, testNow.Add(time.Hour + time.Second)},
		{"signed by another key", fromOtherKey, testNow},
		{"alg none", signClaims(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid), testNow},
		// The classic algorithm-confusion attack: the public key is public, so
		// an attacker can use its bytes as an HMAC secret. Pinning EdDSA stops it.
		{"HS256 keyed with the public key", signClaims(t, jwt.SigningMethodHS256, []byte(verifier.key), valid), testNow},
		{"wrong issuer", signClaims(t, jwt.SigningMethodEdDSA, signer.key, withClaims(func(c *jwt.RegisteredClaims) { c.Issuer = "someone-else" })), testNow},
		{"no expiry", signClaims(t, jwt.SigningMethodEdDSA, signer.key, withClaims(func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil })), testNow},
		{"non-numeric subject", signClaims(t, jwt.SigningMethodEdDSA, signer.key, withClaims(func(c *jwt.RegisteredClaims) { c.Subject = "admin" })), testNow},
		{"zero subject", signClaims(t, jwt.SigningMethodEdDSA, signer.key, withClaims(func(c *jwt.RegisteredClaims) { c.Subject = "0" })), testNow},
		{"tampered payload", issued[:strings.Index(issued, ".")+1] + "eyJzdWIiOiIxIn0" + issued[strings.LastIndex(issued, "."):], testNow},
		{"garbage", "not-a-jwt", testNow},
		{"empty", "", testNow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := verifier.Parse(tt.token, tt.now); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		header string
		token  string
		ok     bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"  Bearer   abc  ", "abc", true},
		{"Bearer", "", false},
		{"Bearer ", "", false},
		{"Basic abc", "", false},
		{"abc", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		token, ok := BearerToken(tt.header)
		if token != tt.token || ok != tt.ok {
			t.Errorf("BearerToken(%q) = %q, %v; want %q, %v", tt.header, token, ok, tt.token, tt.ok)
		}
	}
}

func TestLoadKeys(t *testing.T) {
	ctx := context.Background()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	seed := base64.StdEncoding.EncodeToString(privateKey.Seed())
	public := base64.StdEncoding.EncodeToString(publicKey)

	t.Run("missing keys stop startup", func(t *testing.T) {
		useConfig(t, "{}")
		t.Setenv("JWT_PRIVATE_KEY", "")
		t.Setenv("JWT_PUBLIC_KEY", "")
		if _, err := LoadTokenSigner(ctx); err == nil {
			t.Fatal("signer loaded without a private key")
		}
		if _, err := LoadTokenVerifier(ctx); err == nil {
			t.Fatal("verifier loaded without a public key")
		}
	})

	t.Run("malformed keys stop startup", func(t *testing.T) {
		useConfig(t, `{"jwt":{"privateKey":"not base64!","publicKey":"c2hvcnQ="}}`)
		if _, err := LoadTokenSigner(ctx); err == nil {
			t.Fatal("signer accepted a non-base64 key")
		}
		if _, err := LoadTokenVerifier(ctx); err == nil {
			t.Fatal("verifier accepted a key of the wrong length")
		}
	})

	t.Run("config file keys", func(t *testing.T) {
		useConfig(t, `{"jwt":{"privateKey":"`+seed+`","publicKey":"`+public+`","ttl":"30m"}}`)
		signer, err := LoadTokenSigner(ctx)
		if err != nil {
			t.Fatal(err)
		}
		verifier, err := LoadTokenVerifier(ctx)
		if err != nil {
			t.Fatal(err)
		}
		token, expiresAt, err := signer.Issue(7, testNow)
		if err != nil {
			t.Fatal(err)
		}
		if got := expiresAt.Sub(testNow); got != 30*time.Minute {
			t.Fatalf("ttl = %s, want 30m", got)
		}
		// The point of the key pair: a verifier built from the public key alone
		// accepts what the private key signed.
		if userID, err := verifier.Parse(token, testNow); err != nil || userID != 7 {
			t.Fatalf("Parse = %d, %v; want 7, nil", userID, err)
		}
	})

	t.Run("environment keys", func(t *testing.T) {
		useConfig(t, "{}")
		t.Setenv("JWT_PRIVATE_KEY", seed)
		t.Setenv("JWT_PUBLIC_KEY", public)
		if _, err := LoadTokenSigner(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadTokenVerifier(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestUnaryServerAuth(t *testing.T) {
	const publicMethod = "/svc/Public"
	const protectedMethod = "/svc/Protected"
	signer := newTestSigner(t)
	interceptor := UnaryServerAuth(signer.Verifier(), publicMethod)

	token, _, err := signer.Issue(42, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	call := func(method string, md metadata.MD) (uint64, error) {
		ctx := context.Background()
		if md != nil {
			ctx = metadata.NewIncomingContext(ctx, md)
		}
		var seen uint64
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
			seen, _ = UserIDFromContext(ctx)
			return nil, nil
		})
		return seen, err
	}
	bearer := metadata.Pairs(AuthorizationMetadataKey, "Bearer "+token)

	t.Run("public method needs no token", func(t *testing.T) {
		if _, err := call(publicMethod, nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("public method ignores an invalid token", func(t *testing.T) {
		userID, err := call(publicMethod, metadata.Pairs(AuthorizationMetadataKey, "Bearer junk"))
		if err != nil || userID != 0 {
			t.Fatalf("got %d, %v; want 0, nil", userID, err)
		}
	})

	t.Run("public method still sees a valid caller", func(t *testing.T) {
		if userID, err := call(publicMethod, bearer); err != nil || userID != 42 {
			t.Fatalf("got %d, %v; want 42, nil", userID, err)
		}
	})

	t.Run("valid token reaches the handler with the user id", func(t *testing.T) {
		if userID, err := call(protectedMethod, bearer); err != nil || userID != 42 {
			t.Fatalf("got %d, %v; want 42, nil", userID, err)
		}
	})

	rejected := []struct {
		name string
		md   metadata.MD
	}{
		{"no metadata", nil},
		{"no authorization", metadata.Pairs("other", "x")},
		{"wrong scheme", metadata.Pairs(AuthorizationMetadataKey, "Basic "+token)},
		{"invalid token", metadata.Pairs(AuthorizationMetadataKey, "Bearer not-a-jwt")},
		// The old transit header must not grant an identity on its own.
		{"legacy x-user-id header", metadata.Pairs("x-user-id", "42")},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			_, err := call(protectedMethod, tt.md)
			if got := status.Code(err); got != codes.Unauthenticated {
				t.Fatalf("code = %s, want %s", got, codes.Unauthenticated)
			}
		})
	}
}

// A service calling another service forwards the token it received.
func TestGrpcClientForwardAuthFromIncomingMetadata(t *testing.T) {
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs(AuthorizationMetadataKey, "Bearer abc"))
	var forwarded []string
	_ = GrpcClientForwardAuth(incoming, "/svc/M", nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			md, _ := metadata.FromOutgoingContext(ctx)
			forwarded = md.Get(AuthorizationMetadataKey)
			return nil
		})
	if len(forwarded) != 1 || forwarded[0] != "Bearer abc" {
		t.Fatalf("forwarded %v", forwarded)
	}

	_ = GrpcClientForwardAuth(context.Background(), "/svc/M", nil, nil, nil,
		func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
			if md, ok := metadata.FromOutgoingContext(ctx); ok && len(md.Get(AuthorizationMetadataKey)) > 0 {
				t.Fatal("a token appeared from nowhere")
			}
			return nil
		})
}

func TestRPCCode(t *testing.T) {
	notFound := status.Error(codes.NotFound, "x")
	// This is what a grpcx client hands back for the same failure.
	rewritten := gerror.NewCode(gcode.New(int(codes.NotFound), "", nil), "x")
	for name, err := range map[string]error{"status": notFound, "grpcx gerror": rewritten, "wrapped": gerror.Wrap(rewritten, "lookup")} {
		if got := RPCCode(err); got != codes.NotFound {
			t.Errorf("%s: got %s, want NotFound", name, got)
		}
	}
	if RPCCode(nil) != codes.OK || RPCCode(errors.New("plain")) != codes.Unknown {
		t.Error("nil or plain error misclassified")
	}
}

func TestUnaryServerAuthWithoutVerifier(t *testing.T) {
	interceptor := UnaryServerAuth(nil, "/svc/Public")
	handler := func(ctx context.Context, _ any) (any, error) { return "ok", nil }
	withToken := metadata.NewIncomingContext(context.Background(), metadata.Pairs(AuthorizationMetadataKey, "Bearer anything"))

	if res, err := interceptor(withToken, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Public"}, handler); err != nil || res != "ok" {
		t.Fatalf("public method: %v %v", res, err)
	}
	if _, err := interceptor(withToken, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Other"}, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("non-public method: got %s, want Unauthenticated", status.Code(err))
	}
}

func TestAuthPolicy(t *testing.T) {
	user, admin, service := newTestSigner(t), newTestSigner(t), newTestSigner(t)
	interceptor := UnaryServerAuthPolicy(AuthPolicy{
		Verifiers: map[Role]*TokenVerifier{
			RoleUser: user.Verifier(), RoleAdmin: admin.Verifier(), RoleService: service.Verifier(),
		},
		Public: []string{"/svc/Public"},
		Methods: map[string][]Role{
			"/svc/AdminOnly":      {RoleAdmin},
			"/svc/AdminOrService": {RoleAdmin, RoleService},
			"/svc/ExplicitNobody": {},
		},
		DefaultRoles: []Role{RoleUser},
	})
	tokens := map[string]string{}
	for name, signer := range map[string]*TokenSigner{"user": user, "admin": admin, "service": service, "stranger": newTestSigner(t)} {
		token, _, err := signer.Issue(5, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = token
	}
	call := func(method, who string) (Role, codes.Code) {
		ctx := context.Background()
		if who != "" {
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(AuthorizationMetadataKey, "Bearer "+tokens[who]))
		}
		var role Role
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
			role, _ = RoleFromContext(ctx)
			return nil, nil
		})
		return role, status.Code(err)
	}

	cases := []struct {
		method, who string
		code        codes.Code
		role        Role
	}{
		{"/svc/Public", "", codes.OK, ""},
		{"/svc/Public", "admin", codes.OK, RoleAdmin},
		{"/svc/Public", "stranger", codes.OK, ""},
		{"/svc/Default", "user", codes.OK, RoleUser},
		{"/svc/Default", "admin", codes.PermissionDenied, ""},
		{"/svc/Default", "", codes.Unauthenticated, ""},
		{"/svc/Default", "stranger", codes.Unauthenticated, ""},
		{"/svc/AdminOnly", "admin", codes.OK, RoleAdmin},
		{"/svc/AdminOnly", "user", codes.PermissionDenied, ""},
		{"/svc/AdminOnly", "service", codes.PermissionDenied, ""},
		{"/svc/AdminOrService", "service", codes.OK, RoleService},
		{"/svc/AdminOrService", "admin", codes.OK, RoleAdmin},
		{"/svc/AdminOrService", "user", codes.PermissionDenied, ""},
		{"/svc/ExplicitNobody", "admin", codes.PermissionDenied, ""},
	}
	for _, c := range cases {
		role, code := call(c.method, c.who)
		if code != c.code || role != c.role {
			t.Errorf("%s as %q: got %s/%q, want %s/%q", c.method, c.who, code, role, c.code, c.role)
		}
	}
}

func TestLoadKeysAtSection(t *testing.T) {
	ctx := context.Background()
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	useConfig(t, `{"jwtService":{"privateKey":"`+base64.StdEncoding.EncodeToString(privateKey.Seed())+`","issuer":"goframeshop-order","ttl":"1m"},`+
		`"jwtUser":{"issuer":"goframeshop-user"}}`)
	signer, err := LoadTokenSignerAt(ctx, "jwtService")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWTUSER_PUBLIC_KEY", base64.StdEncoding.EncodeToString(publicKey))
	if _, err := LoadTokenVerifierAt(ctx, "jwtUser"); err != nil {
		t.Fatalf("public key from JWTUSER_PUBLIC_KEY: %v", err)
	}
	token, expiresAt, _ := signer.Issue(1, testNow)
	if expiresAt.Sub(testNow) != time.Minute {
		t.Fatalf("ttl = %s", expiresAt.Sub(testNow))
	}
	if _, err := signer.Verifier().Parse(token, testNow); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTokenVerifierAt(ctx, "jwtMissing"); err == nil || !strings.Contains(err.Error(), "JWTMISSING_PUBLIC_KEY") {
		t.Fatalf("missing section: %v", err)
	}
}

func TestServiceTokenIsReusedThenRenewed(t *testing.T) {
	signer, err := NewTokenSigner(make([]byte, 32), "goframeshop-order", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	st := NewServiceToken(signer)
	first, _ := st.current(testNow)
	again, _ := st.current(testNow.Add(29 * time.Second))
	renewed, _ := st.current(testNow.Add(31 * time.Second))
	if first != again || renewed == first {
		t.Fatalf("reuse/renew wrong: first==again %v, renewed==first %v", first == again, renewed == first)
	}
	if id, err := signer.Verifier().Parse(renewed, testNow.Add(31*time.Second)); err != nil || id != serviceSubject {
		t.Fatalf("parse: %d %v", id, err)
	}

	// The service token replaces a forwarded user token instead of adding a
	// second authorization value.
	ctx := metadata.AppendToOutgoingContext(context.Background(), AuthorizationMetadataKey, "Bearer user-token")
	var sent []string
	_ = st.Interceptor(ctx, "/svc/M", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		sent = md.Get(AuthorizationMetadataKey)
		return nil
	})
	if len(sent) != 1 || sent[0] == "Bearer user-token" {
		t.Fatalf("sent %v", sent)
	}
}
