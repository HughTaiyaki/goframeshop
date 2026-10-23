package clients

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hughtaiyaki/goframeshop/utility"
)

// The provider refusing our service token is our misconfiguration: it must
// not reach the shopper as "Unauthenticated" (they would think their own
// login expired). Other codes pass through unchanged.
func TestProviderErrorHidesOurAuthFailures(t *testing.T) {
	for _, code := range []codes.Code{codes.Unauthenticated, codes.PermissionDenied} {
		if got := utility.RPCCode(providerError(status.Error(code, "no"))); got == code {
			t.Errorf("%s leaked through as %s", code, got)
		}
	}
	unavailable := status.Error(codes.Unavailable, "down")
	if got := utility.RPCCode(providerError(unavailable)); got != codes.Unavailable {
		t.Errorf("unavailable became %s", got)
	}
	if providerError(nil) != nil {
		t.Error("nil became an error")
	}
	if err := errors.New("plain"); providerError(err) != err {
		t.Error("plain error changed")
	}
}
