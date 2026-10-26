package fault

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateSnapshot = flag.Bool("update-proto-snapshot", false, "rewrite testdata/proto_snapshot.json from the current protos")

const snapshotPath = "testdata/proto_snapshot.json"

// T10 runs with every `go test ./...`: no processes needed. After an
// intended, compatible change (new fields or messages), refresh the
// snapshot:
//
//	go test ./tests/fault/ -run ProtoCompat -update-proto-snapshot
func TestProtoCompat(t *testing.T) {
	current := CurrentProtos()
	if len(current) < 100 {
		t.Fatalf("only %d messages registered: an api package import is missing", len(current))
	}
	if *updateSnapshot {
		data, _ := json.MarshalIndent(current, "", "  ")
		if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(snapshotPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("snapshot of %d messages written", len(current))
		return
	}
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("%v: create it with -update-proto-snapshot", err)
	}
	var old ProtoSnapshot
	if err := json.Unmarshal(data, &old); err != nil {
		t.Fatal(err)
	}
	for _, problem := range CompareProtos(old, current) {
		t.Error(problem)
	}
}

// The comparison itself, on made-up descriptors.
func TestCompareProtosRules(t *testing.T) {
	old := ProtoSnapshot{
		"shop.Order": {"1": "order_no string", "2": "total uint64", "3": "reserved"},
		"shop.Gone":  {"1": "x string"},
	}
	cases := map[string]struct {
		current ProtoSnapshot
		bad     int
	}{
		"unchanged":            {old, 0},
		"field added":          {ProtoSnapshot{"shop.Order": {"1": "order_no string", "2": "total uint64", "3": "reserved", "4": "remark string"}, "shop.Gone": {"1": "x string"}}, 0},
		"field renumbered":     {ProtoSnapshot{"shop.Order": {"1": "order_no string", "5": "total uint64", "3": "reserved"}, "shop.Gone": {"1": "x string"}}, 1},
		"type changed":         {ProtoSnapshot{"shop.Order": {"1": "order_no string", "2": "total string", "3": "reserved"}, "shop.Gone": {"1": "x string"}}, 1},
		"removed and reserved": {ProtoSnapshot{"shop.Order": {"1": "order_no string", "2": "reserved", "3": "reserved"}, "shop.Gone": {"1": "x string"}}, 0},
		"reserved reused":      {ProtoSnapshot{"shop.Order": {"1": "order_no string", "2": "total uint64", "3": "note string"}, "shop.Gone": {"1": "x string"}}, 1},
		"message removed":      {ProtoSnapshot{"shop.Order": old["shop.Order"]}, 1},
	}
	for name, c := range cases {
		if got := CompareProtos(old, c.current); len(got) != c.bad {
			t.Errorf("%s: %d problems %v, want %d", name, len(got), got, c.bad)
		}
	}
}
