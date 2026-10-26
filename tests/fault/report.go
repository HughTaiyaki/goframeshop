package fault

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Result is one experiment's outcome. The fault it injected is recorded
// here, in the report for people, and never in logs, metrics or spans
// (design §9.3).
type Result struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Fault      string        `json:"fault"`
	Passed     bool          `json:"passed"`
	Duration   time.Duration `json:"duration_ns"`
	Recovery   time.Duration `json:"recovery_ns,omitempty"` // fault injected → invariant holds again
	Orders     []string      `json:"orders,omitempty"`
	Notes      []string      `json:"notes,omitempty"`
	Violations []string      `json:"violations,omitempty"`
}

// Report collects the results of a run.
type Report struct {
	Started     time.Time `json:"started"`
	Finished    time.Time `json:"finished"`
	Environment string    `json:"environment"`
	Results     []*Result `json:"results"`
}

// Write stores the report as JSON and Markdown in dir and returns the
// Markdown path.
func (r *Report) Write(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), data, 0o644); err != nil {
		return "", err
	}
	var b strings.Builder
	passed := 0
	for _, res := range r.Results {
		if res.Passed {
			passed++
		}
	}
	fmt.Fprintf(&b, "# 故障实验报告\n\n开始 %s，用时 %s，环境 `%s`：**%d / %d 通过**。\n\n",
		r.Started.Format("2006-01-02 15:04:05"), r.Finished.Sub(r.Started).Round(time.Second), r.Environment, passed, len(r.Results))
	b.WriteString("| ID | 实验 | 注入的故障 | 结果 | 用时 | 恢复用时 |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, res := range r.Results {
		mark := "✅ 通过"
		if !res.Passed {
			mark = "❌ 失败"
		}
		recovery := "—"
		if res.Recovery > 0 {
			recovery = res.Recovery.Round(10 * time.Millisecond).String()
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", res.ID, res.Title, res.Fault, mark,
			res.Duration.Round(10*time.Millisecond), recovery)
	}
	for _, res := range r.Results {
		fmt.Fprintf(&b, "\n## %s %s\n\n", res.ID, res.Title)
		if len(res.Orders) > 0 {
			fmt.Fprintf(&b, "订单：`%s`\n\n", strings.Join(res.Orders, "`、`"))
		}
		for _, n := range res.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
		for _, v := range res.Violations {
			fmt.Fprintf(&b, "- ❌ %s\n", v)
		}
	}
	path := filepath.Join(dir, "report.md")
	return path, os.WriteFile(path, []byte(b.String()), 0o644)
}

// LogEvents returns the structured log lines of a service's log file whose
// event.name is name (task 0019 format; other lines are skipped).
func LogEvents(path, name string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var line map[string]any
		if json.Unmarshal(scanner.Bytes(), &line) != nil {
			continue
		}
		if line["event.name"] == name {
			out = append(out, line)
		}
	}
	return out, scanner.Err()
}

// Attr reads attributes[key] of a log line as a string.
func Attr(line map[string]any, key string) string {
	attrs, _ := line["attributes"].(map[string]any)
	if v, ok := attrs[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}
