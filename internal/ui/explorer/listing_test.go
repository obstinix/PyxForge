package explorer

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// TestSlowFolderShowsLoading: a folder that takes longer than syncWait to list shows a
// "Loading…" row at once, and its rows when the worker ends; the UI thread never waits on it.
func TestSlowFolderShowsLoading(t *testing.T) {
	test.NewTempApp(t)
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "a.c"), nil, 0o644)
	release := make(chan struct{})
	listDir = func(dir string) listing { <-release; return list(dir) }
	defer func() { listDir = list }()
	e := New(root)
	posted := make(chan func(), 4)
	e.Dispatch = func(f func()) { posted <- f }

	start := time.Now()
	got := e.childUIDs("")
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("the tree waited %v for a slow folder", d)
	}
	if len(got) != 1 || !strings.HasSuffix(got[0], loadingRow) {
		t.Fatalf("rows while listing: %q", got)
	}
	if again := e.childUIDs(""); !slices.Equal(again, got) {
		t.Fatalf("a second draw started another listing: %q", again)
	}
	close(release)
	select {
	case f := <-posted:
		f()
	case <-time.After(5 * time.Second):
		t.Fatal("the listing never arrived")
	}
	if got := e.childUIDs(""); !slices.Equal(got, []string{"sub", "a.c"}) {
		t.Fatalf("rows after listing: %q", got)
	}
	// Changed on disk, the folder keeps its rows until the new listing is in.
	e.forget("")
	release = make(chan struct{})
	if got := e.childUIDs(""); !slices.Equal(got, []string{"sub", "a.c"}) {
		t.Fatalf("rows while listing again: %q", got)
	}
	close(release)
	(<-posted)()
}

// TestShowAll: past ShowLimit entries a folder ends in a "Show all (N)" row, which lists the
// rest on a click.
func TestShowAll(t *testing.T) {
	test.NewTempApp(t)
	root := t.TempDir()
	for i := 0; i < 12; i++ {
		os.WriteFile(filepath.Join(root, fmt.Sprintf("f%02d.c", i)), nil, 0o644)
	}
	e := New(root)
	e.ShowLimit = 5
	test.NewTempWindow(t, e.Widget())
	got := e.childUIDs("")
	if len(got) != 6 || got[5] != moreRow {
		t.Fatalf("rows %q", got)
	}
	e.click(got[5], false)
	if got := e.childUIDs(""); len(got) != 12 {
		t.Fatalf("after Show all: %d rows", len(got))
	}
	if !e.Reveal(filepath.Join(root, "f11.c")) {
		t.Fatal("Reveal failed past the limit")
	}
}

// TestHiddenSet: the hidden folders can be changed, for the tree and Go to File alike.
func TestHiddenSet(t *testing.T) {
	test.NewTempApp(t)
	defer SetHidden(DefaultHidden)
	root := t.TempDir()
	for _, d := range []string{"build", "src"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
		os.WriteFile(filepath.Join(root, d, "x.c"), nil, 0o644)
	}
	SetHidden([]string{"build"})
	if got := New(root).childUIDs(""); !slices.Equal(got, []string{"src"}) {
		t.Fatalf("rows %q", got)
	}
	if files, _ := Files(root, 10); !slices.Equal(files, []string{"src/x.c"}) {
		t.Fatalf("files %q", files)
	}
}
