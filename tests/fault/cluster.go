// Package fault runs the fault experiments of task 0021 (design §10)
// against real processes: it builds and starts user, goods, order and the
// payment simulator, injects faults through utility/failpoint, and checks
// the invariants once the system has recovered on its own.
//
//	make infra-up
//	make fault-trade        # GOFRAMESHOP_FAULT=1 go test ./tests/fault/ -v
package fault

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/gogf/gf/v2/encoding/gjson"
)

// Service is one process of the shop under test.
type Service struct {
	Name  string // directory under app/
	Ports []int  // listening ports; the first is gRPC
}

var services = []Service{
	{"user", []int{31001}},
	{"goods", []int{31005}},
	{"order", []int{31007}},
	{"paysim", []int{31009, 8399}},
}

// overrides make the experiments fast: short payment timeout, quick
// recovery passes, and every signal labelled environment=experiment.
var overrides = map[string]map[string]any{
	"*": {"otel.environment": "experiment"},
	"order": {
		"order.paymentTimeout":   "8s",
		"order.reserveTimeout":   "20s",
		"order.recoveryInterval": "1s",
	},
}

// Cluster owns the processes of one experiment run.
type Cluster struct {
	Root   string // repository root
	Work   string // binaries, configs, logs, failpoint barriers
	mu     sync.Mutex
	procs  map[string]*exec.Cmd
	exited map[string]chan struct{} // closed when the process has ended
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// NewCluster builds the binaries and writes the experiment configs into
// work. It refuses to run if a service port is already taken (a local dev
// copy would answer instead of the process under test).
func NewCluster(work string) (*Cluster, error) {
	c := &Cluster{Root: repoRoot(), Work: work, procs: map[string]*exec.Cmd{}, exited: map[string]chan struct{}{}}
	for _, s := range services {
		for _, port := range s.Ports {
			if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond); err == nil {
				conn.Close()
				return nil, fmt.Errorf("port %d is in use: stop the local %s before running the experiments", port, s.Name)
			}
		}
	}
	for _, dir := range []string{"bin", "config", "logs", "failpoints"} {
		if err := os.MkdirAll(filepath.Join(work, dir), 0o700); err != nil {
			return nil, err
		}
	}
	for _, s := range services {
		build := exec.Command("go", "build", "-o", filepath.Join(work, "bin", s.Name), "./app/"+s.Name)
		build.Dir = c.Root
		if out, err := build.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("build %s: %v\n%s", s.Name, err, out)
		}
		if err := c.writeConfig(s.Name); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// writeConfig copies the local (git-ignored) config of name with the
// experiment overrides. The local file is never changed.
func (c *Cluster) writeConfig(name string) error {
	src := filepath.Join(c.Root, "app", name, "manifest", "config", "config.yaml")
	cfg, err := gjson.LoadPath(src, gjson.Options{Safe: true})
	if err != nil {
		return fmt.Errorf("read %s (run make dev-config first): %w", src, err)
	}
	for _, set := range []map[string]any{overrides["*"], overrides[name]} {
		for k, v := range set {
			if err := cfg.Set(k, v); err != nil {
				return err
			}
		}
	}
	out, err := cfg.ToYaml()
	if err != nil {
		return err
	}
	dir := filepath.Join(c.Work, "config", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), out, 0o600)
}

// FailpointDir is where paused points and the experiment meet.
func (c *Cluster) FailpointDir() string { return filepath.Join(c.Work, "failpoints") }

// LogPath is the combined output of a service across its restarts.
func (c *Cluster) LogPath(name string) string { return filepath.Join(c.Work, "logs", name+".log") }

// Start launches name with the given failpoints ("" for none) and waits
// until it listens.
func (c *Cluster) Start(name, failpoints string) error {
	svc := find(name)
	logFile, err := os.OpenFile(c.LogPath(name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(c.Work, "bin", name))
	cmd.Dir = filepath.Join(c.Root, "app", name)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.Env = append(os.Environ(),
		"GF_GCFG_PATH="+filepath.Join(c.Work, "config", name),
		"GF_GCFG_FILE=config.yaml",
		"GOFRAMESHOP_FAILPOINTS="+failpoints,
		"GOFRAMESHOP_FAILPOINT_DIR="+c.FailpointDir(),
	)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	// Closed, not sent on: WaitExit, Kill and the start-up check may all
	// wait for the same exit.
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); logFile.Close(); close(done) }()
	c.mu.Lock()
	c.procs[name], c.exited[name] = cmd, done
	c.mu.Unlock()
	return waitListening(svc.Ports, done, 30*time.Second)
}

// WaitExit waits for name to end on its own (an exit failpoint).
func (c *Cluster) WaitExit(name string, timeout time.Duration) error {
	c.mu.Lock()
	done := c.exited[name]
	c.mu.Unlock()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("%s did not exit within %s", name, timeout)
	}
}

// Kill ends name at once (SIGKILL) and waits until it is gone.
func (c *Cluster) Kill(name string) error {
	c.mu.Lock()
	cmd, done := c.procs[name], c.exited[name]
	c.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(syscall.SIGKILL)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("%s did not stop", name)
	}
	return waitFree(find(name).Ports, 10*time.Second)
}

// Restart kills name (if still running) and starts it with failpoints.
func (c *Cluster) Restart(name, failpoints string) error {
	if err := c.Kill(name); err != nil {
		return err
	}
	return c.Start(name, failpoints)
}

// StopAll ends every process.
func (c *Cluster) StopAll() {
	for _, s := range services {
		_ = c.Kill(s.Name)
	}
}

// Release lets a paused failpoint continue; Hit reports whether it was
// reached.
func (c *Cluster) Release(point string) error {
	return os.WriteFile(filepath.Join(c.FailpointDir(), point+".release"), nil, 0o600)
}

func (c *Cluster) Hit(point string) bool {
	_, err := os.Stat(filepath.Join(c.FailpointDir(), point+".hit"))
	return err == nil
}

func find(name string) Service {
	for _, s := range services {
		if s.Name == name {
			return s
		}
	}
	panic("unknown service " + name)
}

func waitListening(ports []int, exited <-chan struct{}, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for _, port := range ports {
		for {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
			if err == nil {
				conn.Close()
				break
			}
			select {
			case <-exited:
				return fmt.Errorf("exited before listening on %d", port)
			default:
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("not listening on %d after %s", port, timeout)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

func waitFree(ports []int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for _, port := range ports {
		for {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
			if err != nil {
				break
			}
			conn.Close()
			if time.Now().After(deadline) {
				return fmt.Errorf("port %d still in use after %s", port, timeout)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

// Eventually polls cond until it holds or timeout passes: waiting for the
// system to reach a state, never a fixed sleep.
func Eventually(ctx context.Context, timeout time.Duration, cond func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for {
		ok, err := cond()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("condition not met within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
