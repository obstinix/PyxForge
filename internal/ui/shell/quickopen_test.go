package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestQuickOpenListsOffTheUIThread: Go to File opens at once and takes the file list when the
// walk on its worker ends, keeping what was typed; the next time it opens with that list.
func TestQuickOpenListsOffTheUIThread(t *testing.T) {
	s, root, q := newEditorShell(t)
	for _, f := range []string{"boot.asm", "src/kernel.c", "src/mm/page.c"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	s.QuickOpen()
	if d := time.Since(start); d > 50*time.Millisecond && !raceEnabled {
		t.Errorf("Go to File took %v to open", d)
	}
	if !s.palette.Visible() || len(s.palette.Results()) != 0 {
		t.Fatalf("palette visible %v with %d results before the walk", s.palette.Visible(), len(s.palette.Results()))
	}
	s.palette.Update(s.palette.Gen()-1, "", nil) // a stale list is ignored
	q.pumpUntil(t, "the file list", func() bool { return len(s.palette.Results()) == 3 })
	s.palette.Hide()
	s.QuickOpen()
	if len(s.palette.Results()) != 3 {
		t.Fatalf("reopened with %d results, want the last listing", len(s.palette.Results()))
	}
}
