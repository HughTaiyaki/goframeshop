// Package check holds the input rules shared by rotations and positions.
package check

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode"

	"github.com/hughtaiyaki/goframeshop/app/banner/internal/service"
)

// Link accepts an empty link or an absolute http(s) URL. Anything else, in
// particular javascript: and data: URLs, is refused: a banner link ends up in
// an <a href> on the home page, and those schemes would run in the shopper's
// browser when clicked.
func Link(link string) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", nil
	}
	if len(link) > service.MaxLinkBytes || strings.IndexFunc(link, unicode.IsControl) >= 0 {
		return "", service.ErrInvalidArgument
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", service.ErrInvalidArgument
	}
	return link, nil
}

// PicURL resolves the image. A file that does not exist is a bad argument.
func PicURL(ctx context.Context, files service.Files, fileID uint64) (string, error) {
	if fileID == 0 {
		return "", service.ErrInvalidArgument
	}
	picURL, err := files.URL(ctx, fileID)
	if errors.Is(err, service.ErrNotFound) {
		return "", service.ErrInvalidArgument
	}
	return picURL, err
}

// Page validates paging.
func Page(page, size int) error {
	if page < 1 || size < 1 || size > 50 {
		return service.ErrInvalidArgument
	}
	return nil
}
