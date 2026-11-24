// Package termview is the Terminal panel's widget (D17): it draws an emulated screen
// (internal/vt) in the theme's colours and turns keys, pastes and the mouse into what a
// terminal program reads. Which program, and its pseudo-terminal, are the caller's.
package termview

import (
	"image/color"
	"runtime"
	"strings"
	"sync/atomic"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/vt"
)

// View shows one terminal screen and sends what is typed into it to Input.
type View struct {
	widget.BaseWidget

	// Input receives what the user types, pastes or clicks, encoded for the program.
	Input func([]byte)
	// OnResize is called when the view's size in cells changes.
	OnResize func(cols, rows int)
	// ShellTakes reports a shortcut as PyxForge's own, which OnShortcut then runs instead
	// of the program getting it (the K2 key policy: Ctrl+Shift chords, Ctrl+Tab).
	ShellTakes func(fyne.Shortcut) bool
	OnShortcut func(fyne.Shortcut)
	OnFocus    func(bool)
	// Dispatch runs a function on the UI goroutine; Changed uses it. fyne.Do when nil.
	Dispatch func(func())

	screen       *vt.Screen
	textSize     float32
	cellW, cellH float32
	cols, rows   int
	back         int // lines scrolled back into the history
	lastHistory  int
	focused      bool
	pending      atomic.Bool

	selecting   bool
	selA, selB  vt.Pos // the selection, absolute lines; empty when equal
	mouseButton int    // the button a program is told is down, -1 none
}

// New is a view with no screen yet.
func New() *View {
	v := &View{textSize: 13, mouseButton: -1}
	v.measure()
	v.ExtendBaseWidget(v)
	return v
}

// SetScreen shows a screen (nil: none), at its end, and fits it to the view.
func (v *View) SetScreen(s *vt.Screen) {
	v.screen = s
	v.back, v.selA, v.selB, v.selecting = 0, vt.Pos{}, vt.Pos{}, false
	if s != nil {
		v.lastHistory = s.View(0).History
		if v.cols > 0 && v.OnResize != nil {
			v.OnResize(v.cols, v.rows)
		}
	}
	v.Refresh()
}

// Screen is the screen shown.
func (v *View) Screen() *vt.Screen { return v.screen }

// SetTextSize sets the font size.
func (v *View) SetTextSize(size float32) {
	v.textSize = size
	v.measure()
	v.fit(v.Size())
	v.Refresh()
}

// CellSize is the size of one character cell.
func (v *View) CellSize() fyne.Size { return fyne.NewSize(v.cellW, v.cellH) }

// GridSize is the view's size in cells.
func (v *View) GridSize() (cols, rows int) { return v.cols, v.rows }

func (v *View) measure() {
	m := fyne.MeasureText("M", v.textSize, fyne.TextStyle{Monospace: true})
	v.cellW, v.cellH = m.Width, m.Height+2
}

// fit works out the size in cells and reports a change.
func (v *View) fit(s fyne.Size) {
	cols := max(int(s.Width/v.cellW), 2)
	rows := max(int(s.Height/v.cellH), 1)
	if s.Width <= 0 || s.Height <= 0 || (cols == v.cols && rows == v.rows) {
		return
	}
	v.cols, v.rows = cols, rows
	if v.OnResize != nil && v.screen != nil {
		v.OnResize(cols, rows)
	}
}

// Changed is called from any goroutine when the screen's content changed: the view redraws
// once on the UI goroutine, however many calls arrive meanwhile.
func (v *View) Changed() {
	if !v.pending.CompareAndSwap(false, true) {
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

// ScrolledBack is how many lines the view is scrolled back into the history.
func (v *View) ScrolledBack() int { return v.back }

// ScrollBy scrolls the view by n lines, back into the history for positive n.
func (v *View) ScrollBy(n int) {
	if v.screen == nil {
		return
	}
	h := v.screen.View(0).History
	v.back = max(0, min(v.back+n, h))
	v.Refresh()
}

// send gives the program bytes, returning the view to the screen's end.
func (v *View) send(b []byte) {
	if len(b) == 0 || v.Input == nil || v.screen == nil {
		return
	}
	if v.back != 0 || v.hasSelection() {
		v.back, v.selA, v.selB = 0, vt.Pos{}, vt.Pos{}
		v.Refresh()
	}
	v.Input(b)
}

func (v *View) modes() vt.Modes {
	if v.screen == nil {
		return vt.Modes{}
	}
	return v.screen.Modes()
}

// Keys and shortcuts

// AcceptsTab keeps Tab for the program rather than moving focus.
func (v *View) AcceptsTab() bool { return true }

func (v *View) FocusGained() {
	v.focused = true
	v.Refresh()
	if v.OnFocus != nil {
		v.OnFocus(true)
	}
	if v.modes().FocusEvents && v.Input != nil {
		v.Input([]byte("\x1b[I"))
	}
}

func (v *View) FocusLost() {
	v.focused = false
	v.Refresh()
	if v.OnFocus != nil {
		v.OnFocus(false)
	}
	if v.modes().FocusEvents && v.Input != nil {
		v.Input([]byte("\x1b[O"))
	}
}

// Focused reports whether the view has keyboard focus.
func (v *View) Focused() bool { return v.focused }

func (v *View) TypedRune(r rune) {
	var b [4]byte
	n := encodeRune(b[:], r)
	v.send(b[:n])
}

func currentModifiers() fyne.KeyModifier {
	if a := fyne.CurrentApp(); a != nil {
		if d, ok := a.Driver().(desktop.Driver); ok {
			return d.CurrentKeyModifiers()
		}
	}
	return 0
}

func (v *View) TypedKey(e *fyne.KeyEvent) {
	m := currentModifiers()
	if m&fyne.KeyModifierShift != 0 {
		switch e.Name {
		case fyne.KeyPageUp:
			v.ScrollBy(max(v.rows-1, 1))
			return
		case fyne.KeyPageDown:
			v.ScrollBy(-max(v.rows-1, 1))
			return
		case fyne.KeyInsert:
			v.paste()
			return
		}
	}
	v.send(keyBytes(e.Name, m, v.modes()))
}

func (v *View) TypedShortcut(sc fyne.Shortcut) {
	if v.ShellTakes != nil && v.ShellTakes(sc) {
		if v.OnShortcut != nil {
			v.OnShortcut(sc)
		}
		return
	}
	switch s := sc.(type) {
	case *fyne.ShortcutCopy: // Ctrl+C: copy a selection, else interrupt
		if v.hasSelection() {
			v.copySelection()
			return
		}
		v.send([]byte{0x03})
	case *fyne.ShortcutPaste:
		if runtime.GOOS == "windows" { // Ctrl+V pastes in Windows' terminals
			v.paste()
			return
		}
		v.send([]byte{0x16})
	case *fyne.ShortcutCut:
		v.send([]byte{0x18})
	case *fyne.ShortcutSelectAll:
		v.send([]byte{0x01})
	case *fyne.ShortcutUndo:
		v.send([]byte{0x1a})
	case *fyne.ShortcutRedo:
		v.send([]byte{0x19})
	case *desktop.CustomShortcut:
		v.custom(s)
	}
}

func (v *View) custom(s *desktop.CustomShortcut) {
	ctrl := s.Modifier&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0
	shift := s.Modifier&fyne.KeyModifierShift != 0
	alt := s.Modifier&fyne.KeyModifierAlt != 0
	switch {
	case ctrl && shift && s.KeyName == fyne.KeyC:
		v.copySelection()
		return
	case ctrl && shift && s.KeyName == fyne.KeyV:
		v.paste()
		return
	case ctrl && alt:
		return // AltGr on Windows: its character arrives as a rune
	}
	if b := keyBytes(s.KeyName, s.Modifier, v.modes()); b != nil {
		v.send(b)
		return
	}
	if ctrl {
		if b, ok := ctrlBytes(s.KeyName, alt); ok {
			v.send(b)
		}
		return
	}
	if alt {
		if b, ok := altBytes(s.KeyName, shift); ok {
			v.send(b)
		}
	}
}

func (v *View) paste() {
	a := fyne.CurrentApp()
	if a == nil {
		return
	}
	if text := a.Clipboard().Content(); text != "" {
		v.send(pasteBytes(text, v.modes().BracketedPaste))
	}
}

// Selection and the mouse

func (v *View) hasSelection() bool { return v.selA != v.selB }

// SelectedText is the selection's text.
func (v *View) SelectedText() string {
	if v.screen == nil || !v.hasSelection() {
		return ""
	}
	return v.screen.Text(v.selA, v.selB)
}

func (v *View) copySelection() {
	if text := v.SelectedText(); text != "" {
		if a := fyne.CurrentApp(); a != nil {
			a.Clipboard().SetContent(text)
		}
	}
}

// cellAt is the absolute cell under a point, and its column and row on the view.
func (v *View) cellAt(p fyne.Position) (vt.Pos, int, int) {
	col := max(0, min(int(p.X/v.cellW+0.5), v.cols))
	row := max(0, min(int(p.Y/v.cellH), v.rows-1))
	first := 0
	if v.screen != nil {
		first = v.screen.View(v.back).First
	}
	return vt.Pos{Line: first + row, Col: col}, min(int(p.X/v.cellW), v.cols-1), row
}

// reporting is whether a program has the mouse: it asked for it and Shift is not held, which
// keeps selecting (xterm's way).
func (v *View) reporting(m fyne.KeyModifier) bool {
	return v.modes().Mouse != vt.MouseOff && m&fyne.KeyModifierShift == 0
}

func buttonOf(b desktop.MouseButton) int {
	switch b {
	case desktop.MouseButtonTertiary:
		return 1
	case desktop.MouseButtonSecondary:
		return 2
	}
	return 0
}

func (v *View) MouseDown(e *desktop.MouseEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(v); c != nil {
		c.Focus(v)
	}
	if v.reporting(e.Modifier) {
		_, x, y := v.cellAt(e.Position)
		v.mouseButton = buttonOf(e.Button)
		v.send(mouseBytes(v.mouseButton, x, y, false, v.modes().SGRMouse, e.Modifier))
		return
	}
	if e.Button == desktop.MouseButtonPrimary {
		p, _, _ := v.cellAt(e.Position)
		v.selA, v.selB, v.selecting = p, p, true
		v.Refresh()
	}
}

func (v *View) MouseUp(e *desktop.MouseEvent) {
	if v.mouseButton >= 0 {
		_, x, y := v.cellAt(e.Position)
		v.send(mouseBytes(v.mouseButton, x, y, true, v.modes().SGRMouse, e.Modifier))
		v.mouseButton = -1
		return
	}
	v.selecting = false
}

func (v *View) Dragged(e *fyne.DragEvent) {
	if v.mouseButton >= 0 {
		if v.modes().Mouse >= vt.MouseDrag {
			_, x, y := v.cellAt(e.Position)
			v.send(mouseBytes(v.mouseButton+32, x, y, false, v.modes().SGRMouse, 0))
		}
		return
	}
	if !v.selecting {
		return
	}
	switch {
	case e.Position.Y < 0:
		v.ScrollBy(1)
	case e.Position.Y > v.Size().Height:
		v.ScrollBy(-1)
	}
	v.selB, _, _ = v.cellAt(e.Position)
	v.Refresh()
}

func (v *View) DragEnd() { v.selecting = false }

// Tapped focuses the view; a click without a drag leaves no selection.
func (v *View) Tapped(*fyne.PointEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(v); c != nil {
		c.Focus(v)
	}
}

// DoubleTapped selects the word under the pointer.
func (v *View) DoubleTapped(e *fyne.PointEvent) {
	if v.screen == nil || v.reporting(0) {
		return
	}
	p, _, row := v.cellAt(e.Position)
	view := v.screen.View(v.back)
	cells := view.Lines[row].Cells
	word := func(i int) bool {
		if i < 0 || i >= len(cells) {
			return false
		}
		r := cells[i].R
		return r != 0 && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-./\\:~@+%#", r))
	}
	at := min(p.Col, len(cells)-1)
	if !word(at) {
		return
	}
	a, b := at, at
	for word(a - 1) {
		a--
	}
	for word(b + 1) {
		b++
	}
	v.selA, v.selB = vt.Pos{Line: p.Line, Col: a}, vt.Pos{Line: p.Line, Col: b + 1}
	v.Refresh()
}

// TappedSecondary copies the selection, or pastes when there is none (as Windows Terminal).
func (v *View) TappedSecondary(*fyne.PointEvent) {
	if v.hasSelection() {
		v.copySelection()
		v.selA, v.selB = vt.Pos{}, vt.Pos{}
		v.Refresh()
		return
	}
	v.paste()
}

// Scrolled scrolls the history, or tells a program that has the mouse about the wheel; on a
// full-screen program's screen without the mouse, the wheel is arrow keys.
func (v *View) Scrolled(e *fyne.ScrollEvent) {
	lines := int(e.Scrolled.DY / v.cellH * 1.5)
	if lines == 0 && e.Scrolled.DY != 0 {
		lines = 1
		if e.Scrolled.DY < 0 {
			lines = -1
		}
	}
	m := v.modes()
	switch {
	case v.reporting(0):
		_, x, y := v.cellAt(e.Position)
		button := 64
		if lines < 0 {
			button = 65
		}
		for range min(abs(lines), 5) {
			v.send(mouseBytes(button, x, y, false, m.SGRMouse, 0))
		}
	case m.AltScreen:
		key := fyne.KeyUp
		if lines < 0 {
			key = fyne.KeyDown
		}
		for range min(abs(lines), 5) {
			v.send(keyBytes(key, 0, m))
		}
	default:
		v.ScrollBy(lines)
	}
}

func (v *View) Cursor() desktop.Cursor { return desktop.TextCursor }

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func encodeRune(b []byte, r rune) int {
	s := string(r)
	return copy(b, s)
}

// Drawing

func (v *View) CreateRenderer() fyne.WidgetRenderer {
	r := &renderer{v: v, bg: canvas.NewRectangle(color.Transparent)}
	return r
}

type renderer struct {
	v       *View
	bg      *canvas.Rectangle
	objects []fyne.CanvasObject
}

func (r *renderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	r.v.fit(s)
}

func (r *renderer) MinSize() fyne.Size { return fyne.NewSize(r.v.cellW*10, r.v.cellH*2) }

func (r *renderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		return []fyne.CanvasObject{r.bg}
	}
	return r.objects
}

func (r *renderer) Destroy() {}

// colours resolves a cell's foreground and background in the theme.
type colours struct {
	fg, bg color.NRGBA
	pal    [16]color.NRGBA
}

func (c *colours) of(col vt.Color, def color.NRGBA) color.NRGBA {
	if i, ok := col.Index(); ok {
		if i < 16 {
			return c.pal[i]
		}
		return xterm256(i)
	}
	if rr, g, b, ok := col.RGB(); ok {
		return color.NRGBA{R: rr, G: g, B: b, A: 0xff}
	}
	return def
}

func (r *renderer) Refresh() {
	v := r.v
	t := theme.Current()
	size := v.Size()
	cs := colours{fg: theme.Over(t.Text.Primary, t.Surface.Base), bg: t.Surface.Base, pal: Palette(t)}
	r.bg.FillColor = cs.bg
	r.bg.Resize(size)
	objs := []fyne.CanvasObject{r.bg}
	if v.screen == nil {
		r.objects = objs
		r.bg.Refresh()
		return
	}
	// Keep the same lines on view while output arrives under a scrolled-back view.
	if h := v.screen.View(0).History; v.back > 0 && h > v.lastHistory {
		v.back += h - v.lastHistory
		v.lastHistory = h
	} else {
		v.lastHistory = h
	}
	view := v.screen.View(v.back)
	v.back = view.Back
	selA, selB := v.selA, v.selB
	if selB.Line < selA.Line || (selB.Line == selA.Line && selB.Col < selA.Col) {
		selA, selB = selB, selA
	}
	for row, line := range view.Lines {
		y := float32(row) * v.cellH
		abs := view.First + row
		cells := line.Cells
		// Backgrounds, then text runs of one style.
		for x := 0; x < len(cells); {
			c := cells[x]
			fg, bg := cs.of(c.FG, cs.fg), cs.of(c.BG, cs.bg)
			if c.A&vt.Inverse != 0 {
				fg, bg = bg, fg
			}
			end := x + 1
			for end < len(cells) && sameStyle(cells[end], c) {
				end++
			}
			if bg != cs.bg {
				objs = append(objs, rect(bg, float32(x)*v.cellW, y, float32(end-x)*v.cellW, v.cellH))
			}
			if c.A&vt.Hidden == 0 {
				if c.A&vt.Faint != 0 {
					fg = mix(fg, bg, 0.45)
				}
				objs = v.textRun(objs, cells[x:end], x, y, fg, c.A)
			}
			x = end
		}
		// The selection over the text.
		if v.hasSelection() && abs >= selA.Line && abs <= selB.Line {
			from, to := 0, len(cells)
			if abs == selA.Line {
				from = selA.Col
			}
			if abs == selB.Line {
				to = selB.Col
			}
			if to > from {
				objs = append(objs, rect(t.Accent.Selection, float32(from)*v.cellW, y, float32(to-from)*v.cellW, v.cellH))
			}
		}
	}
	// The cursor: a block while focused, an outline otherwise.
	if view.CursorVisible && view.CursorY < len(view.Lines) {
		x, y := float32(view.CursorX)*v.cellW, float32(view.CursorY)*v.cellH
		if v.focused {
			objs = append(objs, rect(cs.fg, x, y, v.cellW, v.cellH))
			if c := view.Lines[view.CursorY].Cells[view.CursorX]; c.R != 0 && c.R != ' ' {
				tx := canvas.NewText(string(c.R), cs.bg)
				tx.TextSize, tx.TextStyle = v.textSize, fyne.TextStyle{Monospace: true, Bold: c.A&vt.Bold != 0}
				tx.Move(fyne.NewPos(x, y+1))
				objs = append(objs, tx)
			}
		} else {
			box := rect(color.Transparent, x, y, v.cellW, v.cellH)
			box.StrokeColor, box.StrokeWidth = t.Text.Secondary, 1
			objs = append(objs, box)
		}
	}
	// Scrolled back: where in the history, on the right edge.
	if view.Back > 0 && view.History > 0 {
		total := float32(view.History + len(view.Lines))
		h := max(size.Height*float32(len(view.Lines))/total, 12)
		top := (size.Height - h) * float32(view.History-view.Back) / float32(view.History)
		objs = append(objs, rect(t.Border.Strong, size.Width-4, top, 3, h))
	}
	r.objects = objs
	r.bg.Refresh()
}

func sameStyle(a, b vt.Cell) bool {
	return a.FG == b.FG && a.BG == b.BG && a.A == b.A && a.Width == 1 && b.Width == 1
}

// textRun draws cells of one style starting at column x. A wide character is drawn on its
// own, at its cell, so the font's width for it cannot shift what follows.
func (v *View) textRun(objs []fyne.CanvasObject, cells []vt.Cell, x int, y float32, fg color.NRGBA, a vt.Attr) []fyne.CanvasObject {
	var b strings.Builder
	for i, c := range cells {
		if c.Width == 0 {
			continue
		}
		if arms, ok := boxArms[c.R]; ok {
			// Drawn as lines that reach the cell's edges, so borders join up; a glyph is
			// shorter than the row.
			objs = v.boxChar(objs, arms, float32(x+i)*v.cellW, y, fg)
			b.WriteByte(' ')
			continue
		}
		if c.R == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteRune(c.R)
		}
	}
	s := b.String()
	if strings.TrimSpace(s) != "" {
		tx := canvas.NewText(strings.TrimRight(s, " "), fg)
		tx.TextSize = v.textSize
		tx.TextStyle = fyne.TextStyle{Monospace: true, Bold: a&vt.Bold != 0, Italic: a&vt.Italic != 0}
		tx.Move(fyne.NewPos(float32(x)*v.cellW, y+1))
		objs = append(objs, tx)
	}
	w := float32(len(cells)) * v.cellW
	if cells[0].Width == 2 {
		w = 2 * v.cellW
	}
	if a&vt.Underline != 0 {
		objs = append(objs, rect(fg, float32(x)*v.cellW, y+v.cellH-2, w, 1))
	}
	if a&vt.Strike != 0 {
		objs = append(objs, rect(fg, float32(x)*v.cellW, y+v.cellH/2, w, 1))
	}
	return objs
}

// Box-drawing arms: which edges of its cell a light line character reaches.
const (
	armUp = 1 << iota
	armDown
	armLeft
	armRight
)

var boxArms = map[rune]int{
	'─': armLeft | armRight, '│': armUp | armDown,
	'┌': armDown | armRight, '┐': armDown | armLeft, '└': armUp | armRight, '┘': armUp | armLeft,
	'├': armUp | armDown | armRight, '┤': armUp | armDown | armLeft,
	'┬': armDown | armLeft | armRight, '┴': armUp | armLeft | armRight,
	'┼': armUp | armDown | armLeft | armRight,
	'╭': armDown | armRight, '╮': armDown | armLeft, '╰': armUp | armRight, '╯': armUp | armLeft,
}

// boxChar draws a light box-drawing character in the cell at x, y.
func (v *View) boxChar(objs []fyne.CanvasObject, arms int, x, y float32, c color.NRGBA) []fyne.CanvasObject {
	w := max(float32(1), float32(int(v.textSize/12)))
	cx, cy := x+float32(int(v.cellW/2)), y+float32(int(v.cellH/2))
	if arms&armLeft != 0 {
		objs = append(objs, rect(c, x, cy, cx-x+w, w))
	}
	if arms&armRight != 0 {
		objs = append(objs, rect(c, cx, cy, x+v.cellW-cx, w))
	}
	if arms&armUp != 0 {
		objs = append(objs, rect(c, cx, y, w, cy-y+w))
	}
	if arms&armDown != 0 {
		objs = append(objs, rect(c, cx, cy, w, y+v.cellH-cy))
	}
	return objs
}

func rect(c color.Color, x, y, w, h float32) *canvas.Rectangle {
	b := canvas.NewRectangle(c)
	b.Move(fyne.NewPos(x, y))
	b.Resize(fyne.NewSize(w, h))
	return b
}

// mix is a at weight 1-f over b.
func mix(a, b color.NRGBA, f float32) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(float32(x)*(1-f) + float32(y)*f) }
	return color.NRGBA{R: m(a.R, b.R), G: m(a.G, b.G), B: m(a.B, b.B), A: 0xff}
}
