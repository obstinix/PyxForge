package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// TestStartupOpensOnlyTheSavedFiles: a restart reopens the files the last session had open
// and no others (#74, where 20 to 25 tabs appeared at start-up). A session that had 25 files
// open and closed 24 reopens the one left; a state without files opens none; a file opened at
// start-up adds only itself.
func TestStartupOpensOnlyTheSavedFiles(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	store := &workspace.StateStore{Dir: filepath.Join(t.TempDir(), "workspaces")}
	var files []string
	for i := range 25 {
		p := filepath.Join(root, fmt.Sprintf("f%02d.c", i))
		if err := os.WriteFile(p, []byte(fmt.Sprintf("int f%02d;\n", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	start := func() *Shell {
		q := make(queue, 4096)
		s := NewWithOptions(a, root, Options{Editor: true, Dispatch: q.post, State: store})
		noToastExpiry(s)
		t.Cleanup(func() { s.stopEditor(); s.term.stop() }) // a failed test still lets go of root
		return s
	}
	stop := func(s *Shell) {
		s.saveState()
		s.stopEditor()
		s.term.stop()
	}

	// 25 files open, 24 closed: the next start reopens the one left.
	s := start()
	for _, f := range files {
		s.OpenFile(f)
	}
	if len(s.nat.docs) != 25 {
		t.Fatalf("%d documents open, want 25", len(s.nat.docs))
	}
	for _, f := range files[1:] {
		s.nat.close(s.nat.docFor(f))
	}
	stop(s)
	s = start()
	if got := tabTitles(s); !slices.Equal(got, []string{"f00.c"}) {
		t.Fatalf("the restart opened %q, want only f00.c", got)
	}

	// A file opened at start-up adds only itself.
	s.OpenFile(files[7])
	if got := tabTitles(s); !slices.Equal(got, []string{"f00.c", "f07.c"}) {
		t.Errorf("tabs %q after opening f07.c", got)
	}
	stop(s)

	// A saved state with no files opens none.
	if err := store.Save(workspace.State{Root: root}); err != nil {
		t.Fatal(err)
	}
	s = start()
	if got := tabTitles(s); len(got) != 0 {
		t.Errorf("a state without files opened tabs %q", got)
	}
	stop(s)
}
