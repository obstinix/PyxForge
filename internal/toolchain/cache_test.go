package toolchain

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestVersionCache: a second detection runs no version query while the executable is unchanged
// (#53); a changed executable, Refresh, and a failed query each ask again; the cache survives
// in its file.
func TestVersionCache(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gcc.exe")
	if err := os.WriteFile(exe, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	tool := Tool{ID: "gcc", Label: "GCC", Candidates: []string{"gcc"}, VersionArgs: []string{"--version"}}
	runs, fail := 0, false
	file := filepath.Join(dir, "cache", "tool-versions.json")
	detect := func(refresh bool) Status {
		t.Helper()
		d := &Detector{
			LookPath: func(string) (string, error) { return exe, nil },
			Run: func(context.Context, string, []string) (string, error) {
				runs++
				if fail {
					return "", errors.New("no")
				}
				return "gcc 14.2.0", nil
			},
			Getenv: func(string) string { return "" }, Timeout: time.Second,
			Cache: &VersionCache{File: file}, Refresh: refresh,
		}
		st := d.Detect(context.Background(), []Tool{tool})[0]
		if err := d.Cache.Save(); err != nil {
			t.Fatal(err)
		}
		return st
	}
	if st := detect(false); st.Version != "14.2.0" || runs != 1 {
		t.Fatalf("first detection: %q after %d runs", st.Version, runs)
	}
	if st := detect(false); st.Version != "14.2.0" || runs != 1 {
		t.Fatalf("an unchanged tool was run again (%d runs), version %q", runs, st.Version)
	}
	if detect(true); runs != 2 {
		t.Fatalf("Refresh did not ask again: %d runs", runs)
	}
	// A replaced executable is asked again.
	if err := os.WriteFile(exe, []byte("version two"), 0o755); err != nil {
		t.Fatal(err)
	}
	if detect(false); runs != 3 {
		t.Fatalf("a changed executable was not asked again: %d runs", runs)
	}
	// A failure is reported and not remembered; the next detection asks again.
	if err := os.WriteFile(exe, []byte("version three"), 0o755); err != nil {
		t.Fatal(err)
	}
	fail = true
	if st := detect(false); st.Err == nil || runs != 4 {
		t.Fatalf("a failed query: %v after %d runs", st.Err, runs)
	}
	fail = false
	if st := detect(false); st.Version != "14.2.0" || runs != 5 {
		t.Fatalf("after a failure: %q after %d runs", st.Version, runs)
	}
	// A damaged cache file is only a slower start.
	if err := os.WriteFile(file, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st := detect(false); st.Version != "14.2.0" || runs != 6 {
		t.Fatalf("with a damaged cache: %q after %d runs", st.Version, runs)
	}
}
