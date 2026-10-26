package failpoint

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDisabledDoesNothing(t *testing.T) {
	load("", "")
	if enabled {
		t.Fatal("enabled with nothing configured")
	}
	if err := Inject("order.create.after-insert"); err != nil {
		t.Fatal(err)
	}
}

func TestOnlyListedPointsFireAndOnlyOnce(t *testing.T) {
	load("a=error, b=error", "")
	t.Cleanup(func() { load("", "") })
	if err := Inject("c"); err != nil {
		t.Fatalf("unlisted point: %v", err)
	}
	if err := Inject("a"); !errors.Is(err, ErrInjected) {
		t.Fatalf("a: %v", err)
	}
	if err := Inject("a"); err != nil {
		t.Fatalf("a fired twice: %v", err)
	}
	if err := Inject("b"); !errors.Is(err, ErrInjected) {
		t.Fatalf("b: %v", err)
	}
}

func TestExit(t *testing.T) {
	load("crash=exit", "")
	t.Cleanup(func() { load("", ""); exit = os.Exit })
	code := -1
	exit = func(c int) { code = c }
	if err := Inject("crash"); err != nil || code != 86 {
		t.Fatalf("exit code %d, err %v", code, err)
	}
}

func TestPauseIsABarrier(t *testing.T) {
	dir := t.TempDir()
	load("wait=pause", dir)
	t.Cleanup(func() { load("", "") })
	done := make(chan struct{})
	go func() { _ = Inject("wait"); close(done) }()

	hit := filepath.Join(dir, "wait.hit")
	for i := 0; i < 250; i++ {
		if _, err := os.Stat(hit); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(hit); err != nil {
		t.Fatal("the paused point never announced itself")
	}
	select {
	case <-done:
		t.Fatal("passed the barrier before it was released")
	case <-time.After(200 * time.Millisecond):
	}
	if err := os.WriteFile(filepath.Join(dir, "wait.release"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("still paused after release")
	}
}
