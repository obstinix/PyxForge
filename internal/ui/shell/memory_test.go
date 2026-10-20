package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func liveHeap() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestWorkingDoesNotGrowMemory: opening files from the explorer, closing them and moving
// between regions keeps no memory. Moving the active-region accent used to refresh a Fyne
// ThemeOverride, and each refresh made Fyne parse the whole font set again into a cache it
// never empties: about 13 MB a time, 270 MB for twenty files opened and closed.
func TestWorkingDoesNotGrowMemory(t *testing.T) {
	s, root := newTestShell(t)
	var files []string
	for i := range 10 {
		p := filepath.Join(root, fmt.Sprintf("file%02d.txt", i))
		if err := os.WriteFile(p, []byte("text\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	paint := func() { _ = s.win.Canvas().Capture() }
	work := func() {
		for _, f := range files {
			s.Commands().Run("view.focusExplorer")
			s.explorer.OnOpen(f)
			paint()
		}
		for range files {
			s.closeEditor()
			paint()
		}
		for range 10 {
			s.Commands().Run("view.focusEditor")
			paint()
			s.Commands().Run("panel.log")
			paint()
		}
	}
	work() // the first paint of each text style loads its fonts once
	before := liveHeap()
	work()
	work()
	if len(s.editors.Items) != 0 {
		t.Fatalf("%d tabs left open", len(s.editors.Items))
	}
	if grew := int64(liveHeap()) - int64(before); grew > 8<<20 {
		t.Errorf("two rounds of opening, closing and switching kept %.1f MB", float64(grew)/(1<<20))
	}
}
