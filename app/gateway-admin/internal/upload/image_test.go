package upload

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Minimal real file headers; http.DetectContentType only looks at the start.
var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 32)...)
	gifBytes  = append([]byte("GIF89a"), make([]byte, 32)...)
	webpBytes = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
)

var now = time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

var keyPattern = regexp.MustCompile(`^images/2026/10/19/[0-9a-f]{32}\.(jpg|png|gif|webp)$`)

func TestReadImageAcceptsImagesByContent(t *testing.T) {
	tests := []struct {
		name, filename string
		data           []byte
		wantType       string
		wantExt        string
	}{
		{"png", "a.png", pngBytes, "image/png", ".png"},
		{"jpeg", "a.jpeg", jpegBytes, "image/jpeg", ".jpg"},
		{"gif", "a.gif", gifBytes, "image/gif", ".gif"},
		{"webp", "a.webp", webpBytes, "image/webp", ".webp"},
		// The extension follows the content, not the name the client sent.
		{"png named .html", "evil.html", pngBytes, "image/png", ".png"},
		{"png without extension", "noext", pngBytes, "image/png", ".png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := ReadImage(bytes.NewReader(tt.data), tt.filename, 1024, now)
			if err != nil {
				t.Fatal(err)
			}
			if img.ContentType != tt.wantType || !strings.HasSuffix(img.Key, tt.wantExt) || !keyPattern.MatchString(img.Key) {
				t.Fatalf("type=%q key=%q", img.ContentType, img.Key)
			}
		})
	}
}

func TestReadImageRejects(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		max      int64
		filename string
		want     error
	}{
		{"html disguised as jpg", []byte("<!DOCTYPE html><script>alert(1)</script>"), 1024, "photo.jpg", ErrUnsupportedType},
		// SVG can carry script, so it is not an accepted image type.
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), 1024, "a.svg", ErrUnsupportedType},
		{"plain text", []byte("hello"), 1024, "a.png", ErrUnsupportedType},
		{"empty", nil, 1024, "a.png", ErrEmpty},
		{"one byte too large", pngBytes, int64(len(pngBytes) - 1), "a.png", ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ReadImage(bytes.NewReader(tt.data), tt.filename, tt.max, now); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
	// Exactly at the limit is fine.
	if _, err := ReadImage(bytes.NewReader(pngBytes), "a.png", int64(len(pngBytes)), now); err != nil {
		t.Fatalf("at limit: %v", err)
	}
}

func TestKeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		img, err := ReadImage(bytes.NewReader(pngBytes), "same.png", 1024, now)
		if err != nil {
			t.Fatal(err)
		}
		if seen[img.Key] {
			t.Fatalf("duplicate key %s", img.Key)
		}
		seen[img.Key] = true
	}
}

func TestCleanDisplayName(t *testing.T) {
	tests := map[string]string{
		"商品主图.png":            "商品主图.png",
		"../../etc/passwd":    "passwd",
		`C:\Users\me\a b.jpg`: "a b.jpg",
		"bad\x00\nname.png":   "badname.png",
		"  spaced.png  ":      "spaced.png",
		"":                    "",
		"/":                   "",
	}
	for in, want := range tests {
		if got := CleanDisplayName(in); got != want {
			t.Errorf("CleanDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("图", 300)
	if got := CleanDisplayName(long); len([]rune(got)) != 255 {
		t.Fatalf("long name kept %d runes, want 255", len([]rune(got)))
	}
}
