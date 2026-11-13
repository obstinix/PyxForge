package kit

import (
	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// NewParagraph is body text that wraps at word boundaries to the width it is given: captions
// and explanations longer than one line. Text (a single canvas line) is cut off at the edge.
// Primary, Secondary and Tertiary are the only roles a paragraph takes.
func NewParagraph(text string, r Role) *widget.RichText {
	name := fynetheme.ColorNameForeground
	switch r {
	case Secondary:
		name = theme.ColorNameTextSecondary
	case Tertiary:
		name = fynetheme.ColorNamePlaceHolder // the Fyne theme maps it to tertiary text
	}
	p := widget.NewRichText(&widget.TextSegment{Text: text,
		Style: widget.RichTextStyle{ColorName: name, Inline: true, SizeName: fynetheme.SizeNameText, TextStyle: fyne.TextStyle{}}})
	p.Wrapping = fyne.TextWrapWord
	return p
}

// WrapGrid places items of one size in rows, as many to a row as the width holds. Unlike
// Fyne's GridWrap, its height follows the width it was laid out at: GridWrap reports the
// height of the rows it had before, so a page that grew narrower drew its second row over
// what came next. When the row count changes, Layout calls Relayout, which must have the page
// laid out again with the new height later, not from inside this layout (fyne.Do, or a
// shell's dispatch).
type WrapGrid struct {
	Cell     fyne.Size
	Relayout func()
	width    float32
}

// NewWrapGrid returns a WrapGrid for items of the size cell.
func NewWrapGrid(cell fyne.Size, relayout func()) *WrapGrid {
	return &WrapGrid{Cell: cell, Relayout: relayout}
}

func (g *WrapGrid) columns(width float32, n int) int {
	if width <= 0 {
		return max(n, 1) // not laid out yet: one row, corrected by the first layout
	}
	return max(int((width+theme.Space2)/(g.Cell.Width+theme.Space2)), 1)
}

func (g *WrapGrid) rows(width float32, n int) int {
	c := g.columns(width, n)
	return (n + c - 1) / c
}

// Layout implements fyne.Layout.
func (g *WrapGrid) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	n := len(objects)
	before := g.rows(g.width, n)
	g.width = size.Width
	c := g.columns(size.Width, n)
	for i, o := range objects {
		o.Move(fyne.NewPos(float32(i%c)*(g.Cell.Width+theme.Space2), float32(i/c)*(g.Cell.Height+theme.Space2)))
		o.Resize(g.Cell)
	}
	if g.rows(size.Width, n) != before && g.Relayout != nil {
		g.Relayout()
	}
}

// MinSize implements fyne.Layout: one item wide, as tall as the rows at the last width.
func (g *WrapGrid) MinSize(objects []fyne.CanvasObject) fyne.Size {
	r := g.rows(g.width, len(objects))
	if r == 0 {
		return fyne.Size{}
	}
	return fyne.NewSize(g.Cell.Width, float32(r)*g.Cell.Height+float32(r-1)*theme.Space2)
}
