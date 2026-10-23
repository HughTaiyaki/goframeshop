package check

import (
	"errors"
	"strings"
	"testing"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
)

func TestLink(t *testing.T) {
	ok := map[string]string{
		"":                                 "",
		"  ":                               "",
		"https://shop.example.com/goods/1": "https://shop.example.com/goods/1",
		" http://example.com/a?b=c#d ":     "http://example.com/a?b=c#d",
	}
	for in, want := range ok {
		if got, err := Link(in); err != nil || got != want {
			t.Errorf("Link(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:msgbox(1)",
		"//evil.example.com",          // scheme-relative
		"/goods/1",                    // relative
		"ftp://example.com/file",      // not http(s)
		"https://user:pw@example.com", // credentials in the URL
		"https://",                    // no host
		"https://example.com/\x00",    // control character
		"https://example.com/" + strings.Repeat("a", 512),
	}
	for _, in := range bad {
		if _, err := Link(in); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("Link(%q) accepted", in)
		}
	}
}
