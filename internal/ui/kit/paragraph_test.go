package kit

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// TestWrapGrid: the grid's height is that of the rows it holds at the width it was laid out
// at, and a change in the row count asks for the page to be laid out again. Fyne's GridWrap
// kept its old height, and the theme cards' second row was drawn over the next section.
func TestWrapGrid(t *testing.T) {
	test.NewTempApp(t)
	cell := fyne.NewSize(240, 168)
	relaid := make(chan struct{}, 4)
	g := NewWrapGrid(cell, func() { relaid <- struct{}{} })
	var items []fyne.CanvasObject
	for range 6 {
		items = append(items, canvas.NewRectangle(nil))
	}
	c := container.New(g, items...)
	wide := 4*cell.Width + 3*theme.Space2
	c.Resize(fyne.NewSize(wide, 1000)) // four to a row: two rows
	if h := g.MinSize(items).Height; h != 2*cell.Height+theme.Space2 {
		t.Errorf("four to a row, six items: height %v, want two rows", h)
	}
	if p := items[4].Position(); p.X != 0 || p.Y != cell.Height+theme.Space2 {
		t.Errorf("the fifth item is at %v, want the start of the second row", p)
	}
	c.Resize(fyne.NewSize(cell.Width+10, 1000)) // one to a row: six rows
	if h := g.MinSize(items).Height; h != 6*cell.Height+5*theme.Space2 {
		t.Errorf("one to a row: height %v, want six rows", h)
	}
	select { // it runs after the layout, on the UI thread
	case <-relaid:
	case <-time.After(2 * time.Second):
		t.Error("the row count changed and the page was not asked to lay out again")
	}
	if w := g.MinSize(items).Width; w != cell.Width {
		t.Errorf("minimum width %v, want one item: a narrow window must still fit", w)
	}
	p := NewParagraph("a long caption that should wrap at word boundaries rather than run off the edge", Tertiary)
	if p.Wrapping != fyne.TextWrapWord {
		t.Error("a paragraph does not wrap")
	}
}
