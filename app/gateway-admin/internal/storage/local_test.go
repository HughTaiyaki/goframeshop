package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalPutAndDelete(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := NewLocal(dir, "http://127.0.0.1:8299/files/")
	if err != nil {
		t.Fatal(err)
	}

	url, err := store.Put(ctx, "images/2026/10/19/abc.png", []byte("png"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if url != "http://127.0.0.1:8299/files/images/2026/10/19/abc.png" {
		t.Fatalf("url = %s", url)
	}
	path := filepath.Join(dir, "images", "2026", "10", "19", "abc.png")
	if got, err := os.ReadFile(path); err != nil || string(got) != "png" {
		t.Fatalf("stored %q, %v", got, err)
	}

	// insert-only, like the Qiniu upload policy.
	if _, err := store.Put(ctx, "images/2026/10/19/abc.png", []byte("other"), "image/png"); err == nil {
		t.Fatal("an existing object was overwritten")
	}

	if err := store.Delete(ctx, "images/2026/10/19/abc.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("object still present after delete")
	}
	if err := store.Delete(ctx, "images/2026/10/19/abc.png"); err != nil {
		t.Fatalf("deleting a missing object should be a no-op: %v", err)
	}
}

func TestLocalRefusesKeysOutsideItsDirectory(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	store, err := NewLocal(filepath.Join(parent, "uploads"), "http://x/files")
	if err != nil {
		t.Fatal(err)
	}
	// The directory must exist right away: the gateway serves it statically.
	if info, err := os.Stat(store.Dir()); err != nil || !info.IsDir() {
		t.Fatalf("storage directory not created: %v", err)
	}
	for _, key := range []string{"../escape.png", "a/../../escape.png", ""} {
		if _, err := store.Put(ctx, key, []byte("x"), "image/png"); err == nil {
			t.Errorf("Put(%q) succeeded", key)
		}
	}
	if _, err := os.Stat(filepath.Join(parent, "escape.png")); !os.IsNotExist(err) {
		t.Fatal("a file was written outside the storage directory")
	}
}

func TestQiniuNeedsCompleteConfig(t *testing.T) {
	if _, err := NewQiniu(QiniuConfig{Bucket: "b", Domain: "https://d", Region: "z2"}); err == nil {
		t.Fatal("qiniu accepted missing keys")
	}
	if _, err := NewQiniu(QiniuConfig{AccessKey: "a", SecretKey: "s", Bucket: "b", Domain: "https://d", Region: "nowhere"}); err == nil {
		t.Fatal("qiniu accepted an unknown region")
	}
	if _, err := NewQiniu(QiniuConfig{AccessKey: "a", SecretKey: "s", Bucket: "b", Domain: "https://d", Region: "z2"}); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
}
