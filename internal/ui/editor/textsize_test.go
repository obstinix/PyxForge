package editor

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/neovim"
)

// TestTextSizeResizesNeovim: a larger text size draws bigger cells, and Neovim, attached to
// the view, is given the smaller grid that now fits the same space; a smaller size, a larger
// grid. Sizes outside the limits are clamped.
func TestTextSizeResizesNeovim(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	test.NewTempApp(t)
	s, err := neovim.Start(context.Background(), neovim.Options{Dir: t.TempDir(), Width: 80, Height: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	v := NewView()
	w := test.NewWindow(v)
	defer w.Close()
	w.Resize(fyne.NewSize(800, 480))
	v.Attach(s)
	v.Resize(fyne.NewSize(800, 480))
	v.Refresh()
	grid := func() (int, int) {
		t.Helper()
		var rows, cols int
		for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
			rows, cols = s.Grid().Size()
			if rows == v.rows && cols == v.cols {
				return rows, cols
			}
		}
		t.Fatalf("Neovim's grid is %dx%d, the view asked for %dx%d", rows, cols, v.rows, v.cols)
		return 0, 0
	}
	rows0, cols0 := grid()

	cell0 := v.CellSize()
	v.SetTextSize(20)
	if c := v.CellSize(); c.Width <= cell0.Width || c.Height <= cell0.Height {
		t.Fatalf("cells at 20 are %v, at the default %v", c, cell0)
	}
	if rows, cols := grid(); rows >= rows0 || cols >= cols0 {
		t.Errorf("at 20 Neovim's grid is %dx%d, at the default %dx%d: it should be smaller", rows, cols, rows0, cols0)
	}
	v.SetTextSize(10)
	if rows, cols := grid(); rows <= rows0 || cols <= cols0 {
		t.Errorf("at 10 Neovim's grid is %dx%d, at the default %dx%d: it should be larger", rows, cols, rows0, cols0)
	}
	v.SetTextSize(1)
	if v.TextSize() != MinTextSize {
		t.Errorf("a size of 1 became %v, want %v", v.TextSize(), MinTextSize)
	}
	v.SetTextSize(100)
	if v.TextSize() != MaxTextSize {
		t.Errorf("a size of 100 became %v, want %v", v.TextSize(), MaxTextSize)
	}
}
