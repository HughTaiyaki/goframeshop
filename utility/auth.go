package utility

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthorizationMetadataKey is the gRPC metadata key that carries the bearer
// token. gRPC lower-cases metadata keys, so this matches the HTTP header
// "Authorization" that the gateway forwards unchanged.
const AuthorizationMetadataKey = "authorization"

// BearerScheme is the only token type this project issues.
const BearerScheme = "Bearer"

const defaultIssuer = "goframeshop-user"

// ErrInvalidToken covers every reason a token is refused. Callers do not learn
// whether it was expired, forged or malformed, and neither does the client.
var ErrInvalidToken = errors.New("invalid access token")

// Tokens are signed with Ed25519 (JWT "EdDSA"). Only the user service holds the
// private key; every other process gets the public key, which can verify a
// token but cannot mint one. With a shared HMAC secret, any service able to
// verify could also forge a token for any user.

// TokenSigner issues access tokens. Only the user service constructs one.
type TokenSigner struct {
	key    ed25519.PrivateKey
	issuer string
	ttl    time.Duration
}

// TokenVerifier checks access tokens. Gateways and services construct one.
type TokenVerifier struct {
	key    ed25519.PublicKey
	issuer string
}

// NewTokenSigner builds a signer from an Ed25519 seed (32 bytes).
func NewTokenSigner(seed []byte, issuer string, ttl time.Duration) (*TokenSigner, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, gerror.Newf("jwt private key must be a %d-byte Ed25519 seed", ed25519.SeedSize)
	}
	if ttl <= 0 {
		return nil, gerror.New("jwt.ttl must be a positive duration")
	}
	return &TokenSigner{key: ed25519.NewKeyFromSeed(seed), issuer: issuer, ttl: ttl}, nil
}

// NewTokenVerifier builds a verifier from an Ed25519 public key (32 bytes).
func NewTokenVerifier(publicKey []byte, issuer string) (*TokenVerifier, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, gerror.Newf("jwt public key must be a %d-byte Ed25519 key", ed25519.PublicKeySize)
	}
	return &TokenVerifier{key: ed25519.PublicKey(publicKey), issuer: issuer}, nil
}

// LoadTokenSigner reads jwt.privateKey (base64 seed, or env JWT_PRIVATE_KEY),
// jwt.issuer and jwt.ttl. It fails instead of falling back to a default key: a
// service that silently signs with a well-known key is worse than one that does
// not start.
func LoadTokenSigner(ctx context.Context) (*TokenSigner, error) {
	return LoadTokenSignerAt(ctx, "jwt")
}

// LoadTokenVerifier reads jwt.publicKey (base64, or env JWT_PUBLIC_KEY) and
// jwt.issuer.
func LoadTokenVerifier(ctx context.Context) (*TokenVerifier, error) {
	return LoadTokenVerifierAt(ctx, "jwt")
}

// LoadTokenSignerAt reads <section>.privateKey, .issuer and .ttl. The key may
// also come from the environment variable <SECTION>_PRIVATE_KEY (JWT_PRIVATE_KEY
// for the "jwt" section, JWTSERVICE_PRIVATE_KEY for "jwtService").
func LoadTokenSignerAt(ctx context.Context, section string) (*TokenSigner, error) {
	seed, err := configKey(ctx, section, "privateKey")
	if err != nil {
		return nil, err
	}
	issuer, err := g.Cfg().Get(ctx, section+".issuer", defaultIssuer)
	if err != nil {
		return nil, gerror.Wrapf(err, "read %s.issuer", section)
	}
	ttl, err := g.Cfg().Get(ctx, section+".ttl", "2h")
	if err != nil {
		return nil, gerror.Wrapf(err, "read %s.ttl", section)
	}
	return NewTokenSigner(seed, issuer.String(), ttl.Duration())
}

// LoadTokenVerifierAt reads <section>.publicKey (or env <SECTION>_PUBLIC_KEY)
// and <section>.issuer. A server trusting several kinds of caller loads one
// verifier per section, for example jwtUser and jwtAdmin.
func LoadTokenVerifierAt(ctx context.Context, section string) (*TokenVerifier, error) {
	publicKey, err := configKey(ctx, section, "publicKey")
	if err != nil {
		return nil, err
	}
	issuer, err := g.Cfg().Get(ctx, section+".issuer", defaultIssuer)
	if err != nil {
		return nil, gerror.Wrapf(err, "read %s.issuer", section)
	}
	return NewTokenVerifier(publicKey, issuer.String())
}

// configKey reads a base64 key from config, falling back to an environment
// variable so a deployment can keep the key out of files entirely.
func configKey(ctx context.Context, section, field string) ([]byte, error) {
	pattern := section + "." + field
	env := strings.ToUpper(section) + "_PRIVATE_KEY"
	if field == "publicKey" {
		env = strings.ToUpper(section) + "_PUBLIC_KEY"
	}
	value, err := g.Cfg().Get(ctx, pattern)
	if err != nil {
		return nil, gerror.Wrapf(err, "read %s", pattern)
	}
	encoded := value.String()
	if encoded == "" {
		encoded = os.Getenv(env)
	}
	if encoded == "" {
		return nil, gerror.Newf("%s (or %s) is required; generate a pair with `make jwt-keys`", pattern, env)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, gerror.Newf("%s is not valid base64", pattern)
	}
	return key, nil
}

// Verifier returns the verifier matching this signer's key, so the user
// service does not need a separate public key setting.
func (s *TokenSigner) Verifier() *TokenVerifier {
	return &TokenVerifier{key: s.key.Public().(ed25519.PublicKey), issuer: s.issuer}
}

// Issue signs a short-lived token whose subject is the user id.
func (s *TokenSigner) Issue(userID uint64, now time.Time) (token string, expiresAt time.Time, err error) {
	expiresAt = now.Add(s.ttl)
	claims := jwt.RegisteredClaims{
		Issuer:    s.issuer,
		Subject:   strconv.FormatUint(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(s.key)
	if err != nil {
		return "", time.Time{}, gerror.Wrap(err, "sign access token")
	}
	return token, expiresAt, nil
}

// Parse verifies the signature, algorithm, issuer and expiry, and returns the
// user id. The algorithm is pinned to EdDSA so a token cannot pick "none", or
// HS256 keyed with the (public) verification key, for itself.
func (v *TokenVerifier) Parse(token string, now time.Time) (uint64, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return v.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return 0, ErrInvalidToken
	}
	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || userID == 0 {
		return 0, ErrInvalidToken
	}
	return userID, nil
}

// BearerToken extracts the token from an "Authorization: Bearer <token>" value.
func BearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, BearerScheme) {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

// Role names the kind of caller a token proves: which key signed it decides
// the role, so a token can never claim a role for itself.
type Role string

const (
	RoleUser    Role = "user"    // shop customer, signed by the user service
	RoleAdmin   Role = "admin"   // back-office administrator, signed by the admin service
	RoleService Role = "service" // another service, signed with that service's own key
)

// roleDefault is used by the single-verifier UnaryServerAuth.
const roleDefault Role = "default"

type callerContextKey struct{}

type caller struct {
	userID uint64
	role   Role
}

// UserIDFromContext returns the caller id that an auth interceptor or the
// gateway auth middleware verified.
//
// The id lives in a private context key, not in metadata, so the only way to
// set it is to present a valid token: a client cannot simply send a header.
func UserIDFromContext(ctx context.Context) (uint64, bool) {
	c, ok := ctx.Value(callerContextKey{}).(caller)
	return c.userID, ok && c.userID != 0
}

// RoleFromContext returns which kind of token the caller presented.
func RoleFromContext(ctx context.Context) (Role, bool) {
	c, ok := ctx.Value(callerContextKey{}).(caller)
	return c.role, ok && c.userID != 0
}

// ContextWithUserID is for tests and for the gateway middleware.
func ContextWithUserID(ctx context.Context, userID uint64) context.Context {
	return ContextWithCaller(ctx, userID, roleDefault)
}

// ContextWithCaller is for tests and for the auth checks themselves.
func ContextWithCaller(ctx context.Context, userID uint64, role Role) context.Context {
	return context.WithValue(ctx, callerContextKey{}, caller{userID: userID, role: role})
}

// AuthPolicy says which callers may call which methods of one gRPC server.
//
// Verifiers maps each accepted role to the key that proves it. Public methods
// are open to anyone (and still learn the caller when a valid token is sent).
// Every other method is open only to the roles listed for it in Methods, or
// to DefaultRoles when it is not listed; an empty list means nobody. A method
// added later is therefore protected unless someone decides otherwise.
type AuthPolicy struct {
	Verifiers    map[Role]*TokenVerifier
	Public       []string
	Methods      map[string][]Role
	DefaultRoles []Role
}

// UnaryServerAuthPolicy enforces p. A missing or invalid token on a protected
// method is Unauthenticated; a valid token of the wrong role is
// PermissionDenied, so a shop user's token on an admin method is refused even
// when this server also trusts shop users elsewhere.
func UnaryServerAuthPolicy(p AuthPolicy) grpc.UnaryServerInterceptor {
	public := make(map[string]struct{}, len(p.Public))
	for _, method := range p.Public {
		public[method] = struct{}{}
	}
	// Try verifiers in a fixed order; keys differ, so at most one can accept.
	roles := make([]Role, 0, len(p.Verifiers))
	for role := range p.Verifiers {
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i] < roles[j] })

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		_, isPublic := public[info.FullMethod]
		var header string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(AuthorizationMetadataKey); len(values) > 0 {
				header = values[0]
			}
		}
		token, hasToken := BearerToken(header)
		var verified *caller
		if hasToken {
			now := time.Now()
			for _, role := range roles {
				if id, err := p.Verifiers[role].Parse(token, now); err == nil {
					verified = &caller{userID: id, role: role}
					break
				}
			}
		}
		if isPublic {
			if verified != nil {
				ctx = ContextWithCaller(ctx, verified.userID, verified.role)
			}
			return handler(ctx, req)
		}
		if verified == nil {
			if !hasToken {
				return nil, status.Error(codes.Unauthenticated, "missing bearer token")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}
		allowed, listed := p.Methods[info.FullMethod]
		if !listed {
			allowed = p.DefaultRoles
		}
		for _, role := range allowed {
			if role == verified.role {
				return handler(ContextWithCaller(ctx, verified.userID, verified.role), req)
			}
		}
		return nil, status.Error(codes.PermissionDenied, "this caller may not call this method")
	}
}

// UnaryServerAuth is the single-key form of UnaryServerAuthPolicy: every
// method except publicMethods needs a token signed by verifier's key.
//
// A nil verifier is for services that have no protected methods at all (the
// search service): every public method is served anonymously and anything
// else is refused, so deny-by-default still holds.
func UnaryServerAuth(verifier *TokenVerifier, publicMethods ...string) grpc.UnaryServerInterceptor {
	policy := AuthPolicy{Public: publicMethods, DefaultRoles: []Role{roleDefault}}
	if verifier != nil {
		policy.Verifiers = map[Role]*TokenVerifier{roleDefault: verifier}
	}
	return UnaryServerAuthPolicy(policy)
}

// GrpcClientForwardAuth copies the caller's bearer token onto every outgoing
// gRPC call. In a gateway the token comes from the HTTP Authorization header;
// in a service calling another service it comes from the incoming gRPC
// metadata. Handlers therefore never touch tokens, and every hop verifies the
// token again for itself.
func GrpcClientForwardAuth(ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	if header := callerAuthorization(ctx); header != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, AuthorizationMetadataKey, header)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

func callerAuthorization(ctx context.Context) string {
	if request := g.RequestFromCtx(ctx); request != nil {
		return request.Header.Get("Authorization")
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(AuthorizationMetadataKey); len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// ServiceToken attaches this service's own short-lived token to every
// outgoing call, so the callee can tell "the order service" from a shopper or
// an administrator. The token is reused until half its lifetime has passed,
// then re-signed; it never leaves the process except in that header.
type ServiceToken struct {
	signer  *TokenSigner
	mu      sync.Mutex
	token   string
	renewAt time.Time
}

func NewServiceToken(signer *TokenSigner) *ServiceToken {
	return &ServiceToken{signer: signer}
}

// serviceSubject is the subject of every service token. Which service it is
// follows from the key that signed it, not from this number.
const serviceSubject = 1

func (s *ServiceToken) current(now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && now.Before(s.renewAt) {
		return s.token, nil
	}
	token, expiresAt, err := s.signer.Issue(serviceSubject, now)
	if err != nil {
		return "", err
	}
	s.token, s.renewAt = token, now.Add(expiresAt.Sub(now)/2)
	return token, nil
}

// Interceptor replaces any forwarded token with the service token.
func (s *ServiceToken) Interceptor(ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	token, err := s.current(time.Now())
	if err != nil {
		return err
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set(AuthorizationMetadataKey, BearerScheme+" "+token)
	return invoker(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, opts...)
}
