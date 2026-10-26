// Package logcapture lets a test read the structured log lines (task 0019)
// that the code under test writes through g.Log().
//
//	logs := logcapture.Start(t)
//	... run the code ...
//	logs.Events(telemetry.EventOrderTransition)
package logcapture

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/hughtaiyaki/goframeshop/utility/telemetry"
)

// Line is one decoded log line.
type Line map[string]any

// Attr returns attributes[key], or nil.
func (l Line) Attr(key string) any {
	attrs, _ := l["attributes"].(map[string]any)
	return attrs[key]
}

// Recorder holds what was logged since Start.
type Recorder struct {
	t   *testing.T
	mu  sync.Mutex
	buf bytes.Buffer
}

func (r *Recorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

// Start sends g.Log() to a buffer through the JSON handler until the test
// ends. Tests using it must not run in parallel with other loggers' users.
func Start(t *testing.T) *Recorder {
	t.Helper()
	r := &Recorder{t: t}
	logger := g.Log()
	logger.SetWriter(r)
	logger.SetStdoutPrint(false)
	logger.SetHandlers(telemetry.JSONHandler("test-service", "test", "test"))
	t.Cleanup(func() {
		logger.SetWriter(nil)
		logger.SetStdoutPrint(true)
		logger.SetHandlers()
	})
	return r
}

// Lines decodes every line written so far; a line that is not JSON fails
// the test.
func (r *Recorder) Lines() []Line {
	r.t.Helper()
	r.mu.Lock()
	text := r.buf.String()
	r.mu.Unlock()
	var out []Line
	for _, raw := range strings.Split(strings.TrimSpace(text), "\n") {
		if raw == "" {
			continue
		}
		var l Line
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			r.t.Fatalf("log line is not JSON: %q: %v", raw, err)
		}
		out = append(out, l)
	}
	return out
}

// Events returns the lines whose event.name is name.
func (r *Recorder) Events(name string) []Line {
	r.t.Helper()
	var out []Line
	for _, l := range r.Lines() {
		if l["event.name"] == name {
			out = append(out, l)
		}
	}
	return out
}
