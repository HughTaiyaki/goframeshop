// Package upload validates uploaded files before they reach object storage.
package upload

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrTooLarge        = errors.New("file is too large")
	ErrUnsupportedType = errors.New("only JPEG, PNG, GIF and WebP images are accepted")
	ErrEmpty           = errors.New("file is empty")
)

// imageTypes maps a sniffed content type to the extension the object gets.
// SVG is deliberately absent: it can carry script and would run on the
// storage domain when opened.
var imageTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",
}

// MimeLimit is the same allow-list in Qiniu's upload-policy syntax.
const MimeLimit = "image/jpeg;image/png;image/gif;image/webp"

// Image is a validated image ready to store.
type Image struct {
	Data        []byte
	ContentType string
	// Key is the object key: images/YYYY/MM/DD/<random>.<ext>.
	Key string
	// DisplayName is the cleaned original name, kept for people to read. It
	// is never part of the key, so it can contain anything printable.
	DisplayName string
}

// ReadImage reads at most maxBytes from r and checks that it is an image.
//
// The type comes from the file's own bytes, not from the client's file name or
// Content-Type header, both of which the client controls. The extension is
// then derived from that type, so "evil.html" holding a PNG is stored as
// .png, and an HTML page named "photo.jpg" is refused.
func ReadImage(r io.Reader, originalName string, maxBytes int64, now time.Time) (*Image, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrEmpty
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrTooLarge
	}
	contentType := http.DetectContentType(data)
	ext, ok := imageTypes[contentType]
	if !ok {
		return nil, ErrUnsupportedType
	}
	key, err := newKey(ext, now)
	if err != nil {
		return nil, err
	}
	return &Image{
		Data:        data,
		ContentType: contentType,
		Key:         key,
		DisplayName: CleanDisplayName(originalName),
	}, nil
}

// newKey returns a key that is unguessable and never collides in practice:
// 128 random bits, grouped by upload date to keep listings manageable.
func newKey(ext string, now time.Time) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "images/" + now.UTC().Format("2006/01/02") + "/" + hex.EncodeToString(random) + "." + ext, nil
}

// CleanDisplayName keeps the base name only, drops control characters and
// limits the length to the 255 characters the file_info.name column holds.
func CleanDisplayName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" {
		return ""
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	for utf8.RuneCountInString(name) > 255 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}
