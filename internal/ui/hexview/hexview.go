// Package hexview shows bytes as a hex dump, read a page at a time from an io.ReaderAt (owner
// review F4). Nothing larger than one screen is ever held, so a file of gigabytes opens at once
// and costs a few kilobytes: the view for binaries, and for files too large for the editor.
package hexview

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Width is the number of bytes a row shows.
const Width = 16

// barWidth is the width of the scroll bar strip along the right edge.
const barWidth = 10

// View is a scrolling hex dump: offset, sixteen bytes in hex, and the same bytes as text.
type View struct {
	widget.BaseWidget

	src  io.ReaderAt
	size int64

	// Base is added to every offset shown: a memory dump's start address. 0 for a file.
	Base uint64

	mu      sync.Mutex
	top     int64 // the first row shown
	rows    int   // rows that fit
	page    []byte
	pageAt  int64
	err     error
	focused bool
	cellW   float32
	cellH   float32
}

// New returns a view of size bytes read from src.
func New(src io.ReaderAt, size int64) *View {
	v := &View{src: src, size: size, pageAt: -1}
	v.ExtendBaseWidget(v)
	v.measure()
	return v
}

// Open returns a view of a file. The file is opened for each page read and closed again, so
// the view holds no handle: a tab can go without anyone closing it, and the file can be
// renamed or deleted while it shows.
func Open(path string) (*View, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return New(fileAt(path), st.Size()), nil
}

// fileAt reads a file by path, opening it for each read.
type fileAt string

func (p fileAt) ReadAt(b []byte, off int64) (int, error) {
	f, err := os.Open(string(p))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return f.ReadAt(b, off)
}

// Len is how many bytes the view shows.
func (v *View) Len() int64 { return v.size }

func (v *View) measure() {
	m := fyne.MeasureText("0", theme.TextCode, fyne.TextStyle{Monospace: true})
	v.cellW, v.cellH = m.Width, m.Height+4
}

// totalRows is the number of rows the data needs.
func (v *View) totalRows() int64 { return max((v.size+Width-1)/Width, 1) }

// maxTop is the last first row that still fills the view.
func (v *View) maxTop() int64 { return max(v.totalRows()-int64(v.rows), 0) }

// Top is the first row shown.
func (v *View) Top() int64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.top
}

// GoTo scrolls so that offset off is on the first row.
func (v *View) GoTo(off int64) {
	v.mu.Lock()
	v.top = min(max(off/Width, 0), v.maxTop())
	v.mu.Unlock()
	v.Refresh()
}

// scroll moves the view by n rows.
func (v *View) scroll(n int64) {
	v.mu.Lock()
	v.top = min(max(v.top+n, 0), v.maxTop())
	v.mu.Unlock()
	v.Refresh()
}

// rowsAt reads the bytes of the rows from top: one page, read again only when the view moves.
func (v *View) rowsAt(top int64, rows int) []byte {
	off := top * Width
	n := int64(rows * Width)
	if off >= v.size {
		return nil
	}
	n = min(n, v.size-off)
	if v.pageAt == off && int64(len(v.page)) == n {
		return v.page
	}
	buf := make([]byte, n)
	k, err := v.src.ReadAt(buf, off)
	if err != nil && err != io.EOF {
		v.err = err
	} else {
		v.err = nil
	}
	v.page, v.pageAt = buf[:k], off
	return v.page
}

// Row formats one row as offset, hex and text.
func Row(off uint64, b []byte) (string, string, string) {
	var hex strings.Builder
	for i := range Width {
		if i < len(b) {
			fmt.Fprintf(&hex, "%02x ", b[i])
		} else {
			hex.WriteString("   ")
		}
		if i == 7 {
			hex.WriteByte(' ')
		}
	}
	text := make([]byte, len(b))
	for i, c := range b {
		if c >= 0x20 && c <= 0x7e {
			text[i] = c
		} else {
			text[i] = '.'
		}
	}
	return fmt.Sprintf("%08x", off), hex.String(), string(text)
}

// Scrolled implements fyne.Scrollable: three rows a wheel step.
func (v *View) Scrolled(ev *fyne.ScrollEvent) {
	switch {
	case ev.Scrolled.DY > 0:
		v.scroll(-3)
	case ev.Scrolled.DY < 0:
		v.scroll(3)
	}
}

// Tapped takes focus, so the keys scroll.
func (v *View) Tapped(*fyne.PointEvent) {
	if c := fyne.CurrentApp(); c != nil {
		if w := c.Driver().CanvasForObject(v); w != nil {
			w.Focus(v)
		}
	}
}

// Dragged on the scroll bar strip moves the view to the pointer.
func (v *View) Dragged(ev *fyne.DragEvent) {
	h := v.Size().Height
	if h <= 0 {
		return
	}
	f := min(max(float64(ev.Position.Y/h), 0), 1)
	v.mu.Lock()
	v.top = int64(f * float64(v.maxTop()))
	v.mu.Unlock()
	v.Refresh()
}

// DragEnd implements fyne.Draggable.
func (v *View) DragEnd() {}

// FocusGained implements fyne.Focusable.
func (v *View) FocusGained() { v.focused = true; v.Refresh() }

// FocusLost implements fyne.Focusable.
func (v *View) FocusLost() { v.focused = false; v.Refresh() }

// TypedRune implements fyne.Focusable.
func (v *View) TypedRune(rune) {}

// TypedKey scrolls: arrows by a row, Page Up and Down by a screen, Home and End to the ends.
func (v *View) TypedKey(ev *fyne.KeyEvent) {
	page := int64(max(v.rows-1, 1))
	switch ev.Name {
	case fyne.KeyUp:
		v.scroll(-1)
	case fyne.KeyDown:
		v.scroll(1)
	case fyne.KeyPageUp:
		v.scroll(-page)
	case fyne.KeyPageDown:
		v.scroll(page)
	case fyne.KeyHome:
		v.GoTo(0)
	case fyne.KeyEnd:
		v.GoTo(v.size)
	}
}

// Cursor is the arrow.
func (v *View) Cursor() desktop.Cursor { return desktop.DefaultCursor }

// MinSize is a dump row wide and a few rows high.
func (v *View) MinSize() fyne.Size {
	return fyne.NewSize(v.cellW*(8+2+3*Width+1+2+Width)+barWidth+8, v.cellH*4)
}

// CreateRenderer implements fyne.Widget.
func (v *View) CreateRenderer() fyne.WidgetRenderer {
	r := &renderer{v: v, bg: canvas.NewRectangle(color.Transparent)}
	r.Refresh()
	return r
}

type renderer struct {
	v       *View
	bg      *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *renderer) Layout(fyne.Size)             { r.Refresh() }
func (r *renderer) MinSize() fyne.Size           { return r.v.MinSize() }
func (r *renderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *renderer) Destroy()                     {}

func (r *renderer) Refresh() {
	v := r.v
	t := theme.Current()
	size := v.Size()
	r.bg.FillColor = t.Surface.Base
	r.bg.Resize(size)
	objs := []fyne.CanvasObject{r.bg}

	v.mu.Lock()
	v.rows = max(int(size.Height/v.cellH), 1)
	v.top = min(v.top, v.maxTop())
	top := v.top
	data := v.rowsAt(top, v.rows)
	err := v.err
	v.mu.Unlock()

	text := func(s string, c color.NRGBA, x, y float32) {
		tx := canvas.NewText(s, c)
		tx.TextSize, tx.TextStyle = theme.TextCode, fyne.TextStyle{Monospace: true}
		tx.Move(fyne.NewPos(x, y+2))
		objs = append(objs, tx)
	}
	x0 := v.cellW
	xHex := x0 + v.cellW*10
	xText := xHex + v.cellW*(3*Width+2)
	for i := 0; i*Width < len(data); i++ {
		b := data[i*Width : min((i+1)*Width, len(data))]
		off, hex, txt := Row(v.Base+uint64((top+int64(i))*Width), b)
		y := float32(i) * v.cellH
		text(off, t.Text.Tertiary, x0, y)
		text(hex, t.Text.Primary, xHex, y)
		text(txt, t.Text.Secondary, xText, y)
	}
	if err != nil {
		text("Cannot read: "+err.Error(), t.Status.Error, x0, float32(len(data)/Width)*v.cellH)
	}
	// Scroll bar: a thumb the size of the screen's share of the data.
	if total := v.totalRows(); total > int64(v.rows) {
		track := canvas.NewRectangle(t.Border.Hairline)
		track.Move(fyne.NewPos(size.Width-barWidth+4, 0))
		track.Resize(fyne.NewSize(1, size.Height))
		frac := float32(v.rows) / float32(total)
		th := max(size.Height*frac, 24)
		pos := float32(float64(top) / float64(max(v.maxTop(), 1)))
		thumb := canvas.NewRectangle(t.Border.Strong)
		thumb.CornerRadius = 3
		thumb.Move(fyne.NewPos(size.Width-barWidth+1, pos*(size.Height-th)))
		thumb.Resize(fyne.NewSize(barWidth-3, th))
		objs = append(objs, track, thumb)
	}
	if v.focused {
		ring := canvas.NewRectangle(color.Transparent)
		ring.StrokeColor, ring.StrokeWidth = t.Accent.Primary, 1
		ring.Resize(size)
		objs = append(objs, ring)
	}
	r.objects = objs
	r.bg.Refresh()
}
