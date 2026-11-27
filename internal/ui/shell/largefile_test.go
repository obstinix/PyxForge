package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// TestLargeFiles opens the three files of owner review F4 the way a click in the explorer
// does, and times each open and its first frame: a 100 MB log, a 2 MB file on one line, and a
// 1 GB binary.
func TestLargeFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 100 MB and a 1 GB file")
	}
	s, root, _ := newEditorShell(t)
	noToastExpiry(s)
	s.win.Resize(fyne.NewSize(1440, 900))

	log := filepath.Join(root, "build.log")
	var b strings.Builder
	for i := 0; b.Len() < 100<<20; i++ {
		fmt.Fprintf(&b, "2026-11-28T10:%02d:%02d.%06d INFO worker %d finished step %d of the build\n", i/60%60, i%60, i, i%16, i)
	}
	write(t, log, b.String())
	b.Reset()
	line := filepath.Join(root, "one-line.json")
	write(t, line, "["+strings.Repeat(`{"id":1,"v":"x"},`, (2<<20)/17)+"{}]")
	bin := filepath.Join(root, "disk.img")
	f, err := os.Create(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 30); err != nil {
		t.Fatal(err)
	}
	f.Close()

	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	for _, c := range []struct {
		path   string
		budget time.Duration
		mem    uint64 // heap growth allowed
	}{
		{log, 1500 * time.Millisecond, 160 << 20},
		{line, 300 * time.Millisecond, 24 << 20},
		{bin, 200 * time.Millisecond, 8 << 20},
	} {
		// The first open of a file just written waits for the virus scanner (4.6 s for the
		// JSON file under Windows Defender): that is not PyxForge's time, so it is spent first.
		if f, err := os.Open(c.path); err == nil {
			f.Close()
		}
		before := heap()
		start := time.Now()
		s.openFile(c.path, true)
		if d := s.activeDoc(); d != nil && d.path == c.path {
			test.WidgetRenderer(d.ed).Refresh() // the first frame
		}
		took := time.Since(start)
		grew := int64(heap()) - int64(before)
		t.Logf("%s: open and first frame %v, heap %+.1f MB", filepath.Base(c.path), took, float64(grew)/(1<<20))
		if !raceEnabled && took > c.budget {
			t.Errorf("%s took %v, budget %v", filepath.Base(c.path), took, c.budget)
		}
		if grew > int64(c.mem) {
			t.Errorf("%s grew the heap by %.1f MB", filepath.Base(c.path), float64(grew)/(1<<20))
		}
	}
	if d := s.activeDoc(); d != nil && d.path == bin {
		t.Errorf("the 1 GB binary opened in the editor: %s", d.name())
	}
}

func write(t *testing.T, p, text string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
