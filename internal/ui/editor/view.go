// Package editor draws an embedded Neovim inside the Fyne window and feeds it keyboard and
// mouse input. Everything that is Vim (modes, motions, buffers, syntax) happens in Neovim; the
// view draws its grid and translates input (decision D4).
package editor

import (
	"image/color"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// View is the editor widget for one Neovim session.
type View struct {
	widget.BaseWidget
	sess *neovim.Session

	// OnShortcut receives the shell's own chords (Ctrl+Shift+…), which Neovim never sees
	// (KEYMAP.md). It reports whether the shell handled the shortcut.
	OnShortcut func(fyne.Shortcut) bool
	// IsShellChord decides which shortcuts belong to the shell.
	IsShellChord func(fyne.Shortcut) bool
	// Dispatch runs work on the UI thread; nil means fyne.Do.
	Dispatch func(func())
	// OnFocus, when set, is told when the view gains or loses keyboard focus.
	OnFocus func(focused bool)

	focused bool
	shift   bool

	cellW, cellH float32
	textSize     float32
	cols, rows   int // the size last requested from Neovim

	pending atomic.Bool // a refresh is queued on the UI thread
	stats   RenderStats
	wheel   float32 // accumulated scroll, in pixels
}

// RenderStats measure the view's drawing, for the feasibility record.
type RenderStats struct {
	mu     sync.Mutex
	Frames int
	Total  time.Duration
	Max    time.Duration
}

// Snapshot returns a copy of the counters.
func (r *RenderStats) Snapshot() (frames int, avg, worst time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Frames > 0 {
		avg = r.Total / time.Duration(r.Frames)
	}
	return r.Frames, avg, r.Max
}

func (r *RenderStats) add(d time.Duration) {
	r.mu.Lock()
	r.Frames++
	r.Total += d
	r.Max = max(r.Max, d)
	r.mu.Unlock()
}

// NewView returns a view; Attach connects it to a session.
func NewView() *View {
	v := &View{textSize: theme.TextCode}
	v.ExtendBaseWidget(v)
	v.measure()
	return v
}

// Attach connects the view to a running session. Call FlushHook from the session's OnFlush.
func (v *View) Attach(s *neovim.Session) {
	v.sess = s
	v.cols, v.rows = 0, 0
	v.Refresh()
}

// FlushHook schedules a redraw on the UI thread; repeated flushes before it runs coalesce.
// It is safe to call from any goroutine.
func (v *View) FlushHook() {
	if v.pending.Swap(true) {
		return
	}
	do := v.Dispatch
	if do == nil {
		do = fyne.Do
	}
	do(func() {
		v.pending.Store(false)
		v.Refresh()
	})
}

// Stats returns the rendering counters.
func (v *View) Stats() *RenderStats { return &v.stats }

// CellSize is the size of one grid cell in the current font.
func (v *View) CellSize() fyne.Size { return fyne.NewSize(v.cellW, v.cellH) }

func (v *View) measure() {
	m := fyne.MeasureText("M", v.textSize, fyne.TextStyle{Monospace: true})
	v.cellW, v.cellH = m.Width, m.Height
}

// Focus handling: Neovim takes every key while the view has focus (KEYMAP.md).

func (v *View) FocusGained() {
	v.focused = true
	v.Refresh()
	if v.OnFocus != nil {
		v.OnFocus(true)
	}
}

func (v *View) FocusLost() {
	v.focused = false
	v.shift = false
	v.Refresh()
	if v.OnFocus != nil {
		v.OnFocus(false)
	}
}
func (v *View) AcceptsTab() bool {
	return true
}

func (v *View) TypedRune(r rune) {
	if v.sess != nil {
		if k := runeKeys(r); k != "" {
			v.sess.Input(k)
		}
	}
}

func (v *View) TypedKey(e *fyne.KeyEvent) {
	if v.sess == nil {
		return
	}
	if k, ok := specialKey(e.Name, v.shift); ok {
		v.sess.Input(k)
	}
}

// KeyDown and KeyUp track Shift, which Fyne does not report with TypedKey (Shift+Tab).
func (v *View) KeyDown(e *fyne.KeyEvent) {
	if e.Name == desktop.KeyShiftLeft || e.Name == desktop.KeyShiftRight {
		v.shift = true
	}
}

func (v *View) KeyUp(e *fyne.KeyEvent) {
	if e.Name == desktop.KeyShiftLeft || e.Name == desktop.KeyShiftRight {
		v.shift = false
	}
}

func (v *View) TypedShortcut(s fyne.Shortcut) {
	if v.IsShellChord != nil && v.IsShellChord(s) {
		if v.OnShortcut != nil {
			v.OnShortcut(s)
		}
		return
	}
	if v.sess == nil {
		return
	}
	if k, ok := shortcutKeys(s); ok {
		v.sess.Input(k)
	}
}

// Mouse: clicks place the cursor, drags select, the wheel scrolls.

func (v *View) cellAt(p fyne.Position) (row, col int) {
	return int(p.Y / v.cellH), int(p.X / v.cellW)
}

func (v *View) MouseDown(e *desktop.MouseEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(v); c != nil {
		c.Focus(v)
	}
	if v.sess == nil || e.Button != desktop.MouseButtonPrimary {
		return
	}
	r, c := v.cellAt(e.Position)
	go func() { _ = v.sess.Mouse("left", "press", "", r, c) }()
}

func (v *View) MouseUp(e *desktop.MouseEvent) {
	if v.sess == nil || e.Button != desktop.MouseButtonPrimary {
		return
	}
	r, c := v.cellAt(e.Position)
	go func() { _ = v.sess.Mouse("left", "release", "", r, c) }()
}

func (v *View) Dragged(e *fyne.DragEvent) {
	if v.sess == nil {
		return
	}
	r, c := v.cellAt(e.Position)
	go func() { _ = v.sess.Mouse("left", "drag", "", r, c) }()
}

func (v *View) DragEnd() {}

func (v *View) Scrolled(e *fyne.ScrollEvent) {
	if v.sess == nil {
		return
	}
	v.wheel += e.Scrolled.DY
	r, c := v.cellAt(e.Position)
	for v.wheel >= v.cellH {
		v.wheel -= v.cellH
		go func() { _ = v.sess.Mouse("wheel", "up", "", r, c) }()
	}
	for v.wheel <= -v.cellH {
		v.wheel += v.cellH
		go func() { _ = v.sess.Mouse("wheel", "down", "", r, c) }()
	}
}

func (v *View) Cursor() desktop.Cursor { return desktop.TextCursor }

func (v *View) CreateRenderer() fyne.WidgetRenderer {
	return &viewRenderer{v: v, bg: canvas.NewRectangle(color.Black)}
}

type viewRenderer struct {
	v       *View
	bg      *canvas.Rectangle
	objects []fyne.CanvasObject
	size    fyne.Size
}

func rgb(c int32) color.NRGBA {
	return color.NRGBA{R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c), A: 0xff}
}

func (r *viewRenderer) Layout(s fyne.Size) {
	r.size = s
	r.bg.Resize(s)
	v := r.v
	if v.sess == nil || v.cellW == 0 {
		return
	}
	cols, rows := max(int(s.Width/v.cellW), 1), max(int(s.Height/v.cellH), 1)
	if cols != v.cols || rows != v.rows {
		v.cols, v.rows = cols, rows
		go func() { _ = v.sess.Resize(cols, rows) }()
	}
}

func (r *viewRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.v.cellW*20, r.v.cellH*4)
}

// Refresh rebuilds the frame from a grid snapshot: one rectangle per run of non-default
// background, one text object per run of equal attributes, and the cursor.
func (r *viewRenderer) Refresh() {
	start := time.Now()
	v := r.v
	objs := []fyne.CanvasObject{r.bg}
	if v.sess == nil {
		r.bg.FillColor = theme.Current().Surface.Base
		r.bg.Refresh()
		r.objects = objs
		return
	}
	g := v.sess.Grid()
	r.bg.FillColor = rgb(g.BG)
	r.bg.Refresh()
	rows, cols := g.Size()
	for row := 0; row < rows; row++ {
		y := float32(row) * v.cellH
		for col := 0; col < cols; {
			cell := g.Rows[row][col]
			end := col + 1
			// A run ends at a highlight change or after a non-ASCII character, whose width
			// may differ from the cell width.
			if utf8.RuneCountInString(cell.Text) == 1 && cell.Text[0] < 0x80 {
				for end < cols {
					n := g.Rows[row][end]
					if n.HL != cell.HL || len(n.Text) != 1 || n.Text[0] >= 0x80 {
						break
					}
					end++
				}
			}
			a := g.Attr(cell.HL)
			x := float32(col) * v.cellW
			if a.BG != g.BG {
				bg := canvas.NewRectangle(rgb(a.BG))
				bg.Move(fyne.NewPos(x, y))
				bg.Resize(fyne.NewSize(float32(end-col)*v.cellW, v.cellH))
				objs = append(objs, bg)
			}
			text := ""
			for c := col; c < end; c++ {
				text += g.Rows[row][c].Text
			}
			if text != "" && text != " " {
				t := canvas.NewText(text, rgb(a.FG))
				t.TextSize = v.textSize
				t.TextStyle = fyne.TextStyle{Monospace: true, Bold: a.Bold, Italic: a.Italic}
				t.Move(fyne.NewPos(x, y))
				objs = append(objs, t)
			}
			col = end
		}
	}
	objs = append(objs, r.cursor(g)...)
	r.objects = objs
	v.stats.add(time.Since(start))
}

func (r *viewRenderer) cursor(g neovim.Snapshot) []fyne.CanvasObject {
	v := r.v
	rows, cols := g.Size()
	if g.Busy || g.CursorRow >= rows || g.CursorCol >= cols {
		return nil
	}
	cell := g.Rows[g.CursorRow][g.CursorCol]
	a := g.Attr(cell.HL)
	x, y := float32(g.CursorCol)*v.cellW, float32(g.CursorRow)*v.cellH
	cur := canvas.NewRectangle(rgb(a.FG))
	if !v.focused {
		cur.FillColor = color.Transparent
		cur.StrokeColor, cur.StrokeWidth = rgb(a.FG), 1
		cur.Move(fyne.NewPos(x, y))
		cur.Resize(fyne.NewSize(v.cellW, v.cellH))
		return []fyne.CanvasObject{cur}
	}
	pct := float32(max(g.Cursor.Percent, 10)) / 100
	switch g.Cursor.Shape {
	case "vertical":
		cur.Move(fyne.NewPos(x, y))
		cur.Resize(fyne.NewSize(max(v.cellW*pct, 2), v.cellH))
		return []fyne.CanvasObject{cur}
	case "horizontal":
		h := max(v.cellH*pct, 2)
		cur.Move(fyne.NewPos(x, y+v.cellH-h))
		cur.Resize(fyne.NewSize(v.cellW, h))
		return []fyne.CanvasObject{cur}
	}
	cur.Move(fyne.NewPos(x, y))
	cur.Resize(fyne.NewSize(v.cellW, v.cellH))
	out := []fyne.CanvasObject{cur}
	if cell.Text != "" && cell.Text != " " {
		t := canvas.NewText(cell.Text, rgb(a.BG))
		t.TextSize = v.textSize
		t.TextStyle = fyne.TextStyle{Monospace: true, Bold: a.Bold, Italic: a.Italic}
		t.Move(fyne.NewPos(x, y))
		out = append(out, t)
	}
	return out
}

func (r *viewRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		return []fyne.CanvasObject{r.bg}
	}
	return r.objects
}

func (r *viewRenderer) Destroy() {}
