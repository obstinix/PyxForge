package crash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGoContainsAPanic: a panicking worker is logged with its stack and reported, and the
// process goes on.
func TestGoContainsAPanic(t *testing.T) {
	Dir = t.TempDir()
	got := make(chan [3]string, 1)
	Report = func(what, msg, log string) { got <- [3]string{what, msg, log} }
	defer func() { Report = nil }()
	Go("Graph layout", func() { panic("index out of range") })
	var r [3]string
	select {
	case r = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("the panic was not reported")
	}
	if r[0] != "Graph layout" || r[1] != "index out of range" || filepath.Dir(r[2]) != Dir {
		t.Fatalf("reported %q", r)
	}
	data, err := os.ReadFile(r[2])
	if err != nil {
		t.Fatal(err)
	}
	if s := string(data); !strings.Contains(s, "panic: index out of range") || !strings.Contains(s, "crash_test.go") {
		t.Fatalf("log:\n%s", s)
	}
	if !strings.Contains(filepath.Base(r[2]), "graph-layout") {
		t.Fatalf("log name %s", r[2])
	}
}

func TestPruneKeepsTheNewest(t *testing.T) {
	Dir = t.TempDir()
	for i := 0; i < keep+5; i++ {
		if _, err := Write("x", i, nil); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond) // distinct names
	}
	logs, _ := filepath.Glob(filepath.Join(Dir, "crash-*.log"))
	if len(logs) != keep {
		t.Fatalf("%d logs kept", len(logs))
	}
}
