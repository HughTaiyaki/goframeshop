// Package failpoint injects faults at named points in the code, for the
// fault experiments of task 0021 (design §10). A point does nothing unless
// the process was started with it listed in GOFRAMESHOP_FAILPOINTS:
//
//	GOFRAMESHOP_FAILPOINTS="order.create.after-insert=exit,goods.reserve.after-commit=error"
//
// Production never sets that variable, so every point is a single boolean
// check. Each listed point fires once, the first time it is reached: a
// retry in the same process passes through, and a process restarted
// without the variable runs normally.
//
// Actions:
//
//	exit   end the process at once (os.Exit: no deferred calls, no
//	       flushing), as a crash would
//	error  make the step fail: Inject returns ErrInjected, and the caller
//	       reports it as if the reply were lost
//	pause  stop until the experiment lets it go: write <dir>/<name>.hit,
//	       then wait for <dir>/<name>.release (dir is
//	       GOFRAMESHOP_FAILPOINT_DIR). A barrier, not a sleep.
package failpoint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrInjected is returned by Inject for the error action.
var ErrInjected = errors.New("failpoint: injected error")

const (
	envPoints = "GOFRAMESHOP_FAILPOINTS"
	envDir    = "GOFRAMESHOP_FAILPOINT_DIR"
	// pauseLimit bounds a pause, so a forgotten barrier cannot hang a
	// process forever.
	pauseLimit = 2 * time.Minute
)

var state = struct {
	sync.Mutex
	actions map[string]string // name -> action, removed once fired
	dir     string
}{}

var enabled bool

func init() { load(os.Getenv(envPoints), os.Getenv(envDir)) }

// load parses the configuration; tests call it directly.
func load(spec, dir string) {
	state.Lock()
	defer state.Unlock()
	state.actions = map[string]string{}
	state.dir = dir
	for _, item := range strings.Split(spec, ",") {
		name, action, ok := strings.Cut(strings.TrimSpace(item), "=")
		if ok && name != "" {
			state.actions[name] = action
		}
	}
	enabled = len(state.actions) > 0
}

// exit is os.Exit; tests replace it.
var exit = os.Exit

// Inject runs the fault configured for name, if any, and returns
// ErrInjected for the error action, nil otherwise.
func Inject(name string) error {
	if !enabled {
		return nil
	}
	state.Lock()
	action, ok := state.actions[name]
	if ok {
		delete(state.actions, name) // fires once
	}
	dir := state.dir
	state.Unlock()
	if !ok {
		return nil
	}
	fmt.Fprintf(os.Stderr, "failpoint %s: %s\n", name, action)
	switch action {
	case "exit":
		exit(86)
		return nil
	case "error":
		return ErrInjected
	case "pause":
		pause(dir, name)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "failpoint %s: unknown action %q ignored\n", name, action)
		return nil
	}
}

func pause(dir, name string) {
	if dir == "" {
		fmt.Fprintf(os.Stderr, "failpoint %s: pause needs %s; not pausing\n", name, envDir)
		return
	}
	_ = os.WriteFile(filepath.Join(dir, name+".hit"), nil, 0o600)
	release := filepath.Join(dir, name+".release")
	deadline := time.Now().Add(pauseLimit)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(release); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "failpoint %s: not released within %s; continuing\n", name, pauseLimit)
}
