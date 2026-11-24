package codeedit

import (
	"image/color"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/syntax"
	"github.com/obstinix/PyxForge/internal/textbuf"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Severity of a diagnostic, as language servers number them.
const (
	SevError   = 1
	SevWarning = 2
	SevInfo    = 3
	SevHint    = 4
)

// Diagnostic is a problem to mark in the text.
type Diagnostic struct {
	Range    Range
	Severity int
	Message  string
}

// Completion is one entry of the completion list.
type Completion struct {
	Label  string
	Detail string
	Insert string // the text that replaces the identifier before the cursor
}

// Editor is the widget for one document.
type Editor struct {
	widget.BaseWidget
	*Model

	// Path names the file (for the language, and the accessible label).
	Path string
	// ShellTakes decides which shortcuts belong to the window (Save, Find, Close…); they go to
	// OnShortcut instead of the editor.
	ShellTakes func(fyne.Shortcut) bool
	OnShortcut func(fyne.Shortcut) bool
	// OnEdit is called after the text changes; OnCursor after the cursor moves.
	OnEdit   func()
	OnCursor func(Pos)
	// Complete, when set, is asked for completions at the cursor: on Ctrl+Space, and after
	// typing an identifier character or a "." The answer may come later, from any goroutine,
	// through ShowCompletions.
	Complete func(at Pos)
	// OnGutterTap is told of a click in the line-number gutter (breakpoints).
	OnGutterTap func(line int)
	// OnKey, when set, sees function keys and Ctrl+letter shortcuts before the editor does
	// (F9, F12, Ctrl+K…); it reports whether it handled the key.
	OnKey func(key fyne.KeyName, mod fyne.KeyModifier) bool
	// Dispatch runs work on the UI thread; nil means fyne.Do.
	Dispatch func(func())

	hl        *syntax.Highlighter
	spans     [][]syntax.Span
	spansAt   int // the buffer version the spans were made from, -1 for none
	diags     []Diagnostic
	marks     map[int]color.NRGBA // gutter marks by line (breakpoints, the stopped line)
	completes []Completion
	sel       int // the selected completion
	info      string
	top       int // first line shown
	left      int // first display column shown
	rows      int
	cols      int

	textSize     float32
	cellW, cellH float32
	focused      bool
	shift        bool
	lastTap      time.Time
	tapCount     int
	dragging     bool
	hlPending    bool
}

// New returns an editor for buf, which was read from path.
func New(buf *textbuf.Buffer, path string) *Editor {
	e := &Editor{Model: NewModel(buf), Path: path, spansAt: -1, marks: map[int]color.NRGBA{}, textSize: theme.TextCode}
	e.hl = syntax.ForFile(path, firstLines(buf, 40))
	e.LineComment = commentFor(path, e.hl)
	e.ExtendBaseWidget(e)
	e.measure()
	return e
}

func firstLines(b *textbuf.Buffer, n int) string {
	var s strings.Builder
	for i := 0; i < min(n, b.LineCount()); i++ {
		s.WriteString(string(b.Line(i)))
		s.WriteByte('\n')
	}
	return s.String()
}

// commentFor is the line-comment marker for a file's language.
func commentFor(path string, h *syntax.Highlighter) string {
	lower := strings.ToLower(path)
	for _, ext := range []string{".asm", ".s", ".inc", ".nasm"} {
		if strings.HasSuffix(lower, ext) {
			return ";"
		}
	}
	if h == nil {
		return ""
	}
	switch h.Name {
	case "Python", "Bash", "Makefile", "TOML", "YAML", "Ruby", "Perl", "CMake", "Dockerfile", "PowerShell", "R", "Nim", "Elixir":
		return "#"
	case "Lua", "SQL", "Haskell":
		return "--"
	case "Markdown", "HTML", "XML", "JSON", "plaintext":
		return ""
	}
	return "//"
}

// Language is the highlighter's name for the document's language ("" for plain text).
func (e *Editor) Language() string {
	if e.hl == nil {
		return ""
	}
	return e.hl.Name
}

func (e *Editor) measure() {
	m := fyne.MeasureText("M", e.textSize, fyne.TextStyle{Monospace: true})
	e.cellW, e.cellH = m.Width, m.Height+2
}

// SetTextSize changes the font size of the text.
func (e *Editor) SetTextSize(size float32) {
	e.textSize = size
	e.measure()
	e.Refresh()
}

// TextSize is the font size of the text.
func (e *Editor) TextSize() float32 { return e.textSize }

// CellSize is the size of one character cell.
func (e *Editor) CellSize() fyne.Size { return fyne.NewSize(e.cellW, e.cellH) }

// SetDiagnostics replaces the marked problems.
func (e *Editor) SetDiagnostics(d []Diagnostic) {
	e.diags = d
	e.Refresh()
}

// Diagnostics are the marked problems.
func (e *Editor) Diagnostics() []Diagnostic { return e.diags }

// SetMark colours the gutter of a line (nil colour removes it).
func (e *Editor) SetMark(line int, c *color.NRGBA) {
	if c == nil {
		delete(e.marks, line)
	} else {
		e.marks[line] = *c
	}
	e.Refresh()
}

// ShowCompletions opens the completion list (empty closes it). Safe from any goroutine.
func (e *Editor) ShowCompletions(items []Completion) {
	e.do(func() {
		e.completes, e.sel = items, 0
		e.Refresh()
	})
}

// ShowInfo shows text (hover documentation, a message) in a box at the cursor until the next
// key. Safe from any goroutine.
func (e *Editor) ShowInfo(text string) {
	e.do(func() {
		e.info = text
		e.Refresh()
	})
}

func (e *Editor) do(f func()) {
	if e.Dispatch != nil {
		e.Dispatch(f)
		return
	}
	fyne.Do(f)
}

// Reveal scrolls so that p is in view and moves the cursor there.
func (e *Editor) Reveal(p Pos) {
	e.SetCursor(p, false)
	e.scrollToCursor(true)
	e.changed(false)
}

// changed runs after anything that may move the cursor or edit the text.
func (e *Editor) changed(edited bool) {
	e.scrollToCursor(false)
	if edited {
		e.info = ""
		if e.OnEdit != nil {
			e.OnEdit()
		}
	}
	if e.OnCursor != nil {
		e.OnCursor(e.Cur)
	}
	e.Refresh()
}

func (e *Editor) scrollToCursor(center bool) {
	if e.rows <= 0 {
		return
	}
	vis := e.visibleIndex(e.Cur.Line)
	topIdx := e.visibleIndex(e.top)
	switch {
	case center && (vis < topIdx || vis >= topIdx+e.rows):
		e.top = e.lineAtIndex(max(vis-e.rows/3, 0))
	case vis < topIdx:
		e.top = e.Cur.Line
	case vis >= topIdx+e.rows-1:
		e.top = e.lineAtIndex(vis - e.rows + 2)
	}
	col := displayCol(e.Buf.Line(e.Cur.Line), e.Cur.Col, e.TabWidth)
	if col < e.left {
		e.left = max(col-4, 0)
	} else if e.cols > 8 && col >= e.left+e.cols-2 {
		e.left = col - e.cols + 8
	}
}

// visibleIndex counts the visible lines before l.
func (e *Editor) visibleIndex(l int) int {
	if len(e.folds) == 0 {
		return l
	}
	n := 0
	for i := 0; i < l; i++ {
		if !e.LineHidden(i) {
			n++
		}
	}
	return n
}

func (e *Editor) lineAtIndex(idx int) int {
	if len(e.folds) == 0 {
		return min(idx, e.Buf.LineCount()-1)
	}
	v := e.VisibleLines(0, idx+1)
	if len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}

// highlight recolours the document when it has changed, after a short pause in typing so a
// burst of keys costs one pass.
func (e *Editor) highlight() {
	if e.hl == nil || e.spansAt == e.Buf.Version() {
		return
	}
	if e.spansAt < 0 || e.Buf.LineCount() < 3000 {
		e.spans = e.hl.Lines(e.Buf.Text())
		e.spansAt = e.Buf.Version()
		return
	}
	if e.hlPending {
		return
	}
	e.hlPending = true
	text, v := e.Buf.Text(), e.Buf.Version()
	go func() {
		time.Sleep(150 * time.Millisecond)
		spans := e.hl.Lines(text)
		e.do(func() {
			e.hlPending = false
			if v > e.spansAt {
				e.spans, e.spansAt = spans, v
			}
			e.Refresh()
		})
	}()
}

// FocusGained implements fyne.Focusable.
func (e *Editor) FocusGained() {
	e.focused = true
	e.Refresh()
}

// FocusLost implements fyne.Focusable.
func (e *Editor) FocusLost() {
	e.focused, e.shift = false, false
	e.Refresh()
}

// AcceptsTab keeps Tab in the editor, for indentation.
func (e *Editor) AcceptsTab() bool { return true }

// TypedRune implements fyne.Focusable.
func (e *Editor) TypedRune(r rune) {
	e.Type(r)
	e.changed(true)
	if e.Complete != nil && (isWord(r) || r == '.') {
		if len(e.completes) > 0 || r == '.' || len(e.IdentifierBefore()) >= 2 {
			e.Complete(e.Cur)
		}
	} else if len(e.completes) > 0 {
		e.completes = nil
	}
}

// KeyDown and KeyUp track Shift, which extends the selection.
func (e *Editor) KeyDown(k *fyne.KeyEvent) {
	if k.Name == desktop.KeyShiftLeft || k.Name == desktop.KeyShiftRight {
		e.shift = true
	}
}

// KeyUp implements desktop.Keyable.
func (e *Editor) KeyUp(k *fyne.KeyEvent) {
	if k.Name == desktop.KeyShiftLeft || k.Name == desktop.KeyShiftRight {
		e.shift = false
	}
}

func (e *Editor) acceptCompletion() {
	c := e.completes[e.sel]
	word := e.IdentifierBefore()
	e.Anchor = Pos{Line: e.Cur.Line, Col: e.Cur.Col - len([]rune(word))}
	ins := c.Insert
	if ins == "" {
		ins = c.Label
	}
	e.Paste(ins)
	e.completes = nil
}

// TypedKey implements fyne.Focusable.
func (e *Editor) TypedKey(k *fyne.KeyEvent) {
	if len(e.completes) > 0 {
		switch k.Name {
		case fyne.KeyDown:
			e.sel = (e.sel + 1) % len(e.completes)
			e.Refresh()
			return
		case fyne.KeyUp:
			e.sel = (e.sel - 1 + len(e.completes)) % len(e.completes)
			e.Refresh()
			return
		case fyne.KeyReturn, fyne.KeyEnter, fyne.KeyTab:
			e.acceptCompletion()
			e.changed(true)
			return
		case fyne.KeyEscape:
			e.completes = nil
			e.Refresh()
			return
		}
		e.completes = nil
	}
	if e.info != "" && k.Name == fyne.KeyEscape {
		e.info = ""
		e.Refresh()
		return
	}
	if e.OnKey != nil && strings.HasPrefix(string(k.Name), "F") && len(k.Name) > 1 && e.OnKey(k.Name, 0) {
		return
	}
	page := max(e.rows-2, 1)
	edited := true
	switch k.Name {
	case fyne.KeyReturn, fyne.KeyEnter:
		e.Newline()
	case fyne.KeyTab:
		if e.shift {
			e.Outdent()
		} else {
			e.Tab()
		}
	case fyne.KeyBackspace:
		e.Backspace()
	case fyne.KeyDelete:
		e.Delete()
	default:
		edited = false
		mv, ok := map[fyne.KeyName]Move{
			fyne.KeyLeft: Left, fyne.KeyRight: Right, fyne.KeyUp: Up, fyne.KeyDown: Down,
			fyne.KeyHome: LineStart, fyne.KeyEnd: LineEnd, fyne.KeyPageUp: Up, fyne.KeyPageDown: Down,
		}[k.Name]
		if k.Name == fyne.KeyEscape {
			e.SetCursor(e.Cur, false)
		} else if ok {
			n := 1
			if k.Name == fyne.KeyPageUp || k.Name == fyne.KeyPageDown {
				n = page
				e.scrollBy(map[bool]int{true: -page, false: page}[k.Name == fyne.KeyPageUp])
			}
			e.MoveCursor(mv, e.shift, n)
		} else {
			return
		}
	}
	e.changed(edited)
}

// TypedShortcut implements fyne.Shortcutable: the window's shortcuts go to the shell, the
// clipboard and undo go to the document, and Ctrl with the arrows moves by words.
func (e *Editor) TypedShortcut(s fyne.Shortcut) {
	if e.ShellTakes != nil && e.ShellTakes(s) {
		if e.OnShortcut != nil {
			e.OnShortcut(s)
		}
		return
	}
	clip := func() fyne.Clipboard {
		if a := fyne.CurrentApp(); a != nil {
			return a.Clipboard()
		}
		return nil
	}
	edited := true
	switch s := s.(type) {
	case *fyne.ShortcutCopy:
		if c := clip(); c != nil && e.HasSelection() {
			c.SetContent(e.SelectedText())
		}
		edited = false
	case *fyne.ShortcutCut:
		if c := clip(); c != nil && e.HasSelection() {
			c.SetContent(e.SelectedText())
			e.Paste("")
		}
	case *fyne.ShortcutPaste:
		if c := clip(); c != nil {
			e.Paste(c.Content())
		}
	case *fyne.ShortcutSelectAll:
		e.SelectAll()
		edited = false
	case *fyne.ShortcutUndo:
		e.Undo()
	case *fyne.ShortcutRedo:
		e.Redo()
	case *desktop.CustomShortcut:
		edited = e.customShortcut(s)
	default:
		return
	}
	e.changed(edited)
}

func (e *Editor) customShortcut(s *desktop.CustomShortcut) (edited bool) {
	ctrl := s.Modifier&(fyne.KeyModifierShortcutDefault) != 0
	shift := s.Modifier&fyne.KeyModifierShift != 0
	if e.OnKey != nil && e.OnKey(s.KeyName, s.Modifier) {
		return false
	}
	if !ctrl {
		return false
	}
	switch s.KeyName {
	case fyne.KeyLeft:
		e.MoveCursor(WordLeft, shift, 1)
	case fyne.KeyRight:
		e.MoveCursor(WordRight, shift, 1)
	case fyne.KeyHome:
		e.MoveCursor(DocStart, shift, 1)
	case fyne.KeyEnd:
		e.MoveCursor(DocEnd, shift, 1)
	case fyne.KeyUp:
		e.scrollBy(-1)
	case fyne.KeyDown:
		e.scrollBy(1)
	case fyne.KeyBackspace:
		if !e.HasSelection() {
			e.MoveCursor(WordLeft, true, 1)
		}
		e.Backspace()
		return true
	case fyne.KeyDelete:
		if !e.HasSelection() {
			e.MoveCursor(WordRight, true, 1)
		}
		e.Delete()
		return true
	case fyne.KeySlash:
		e.ToggleComment()
		return true
	case fyne.KeyZ:
		if shift {
			e.Redo()
			return true
		}
	case fyne.KeySpace:
		if e.Complete != nil {
			e.Complete(e.Cur)
		}
	}
	return false
}

func (e *Editor) scrollBy(lines int) {
	idx := max(e.visibleIndex(e.top)+lines, 0)
	last := e.visibleIndex(e.Buf.LineCount()-1) - e.rows/2
	e.top = e.lineAtIndex(min(idx, max(last, 0)))
	e.Refresh()
}

// Scrolled implements fyne.Scrollable.
func (e *Editor) Scrolled(ev *fyne.ScrollEvent) {
	if ev.Scrolled.DX != 0 && ev.Scrolled.DY == 0 {
		e.left = max(e.left-int(ev.Scrolled.DX/e.cellW), 0)
		e.Refresh()
		return
	}
	e.scrollBy(-int(ev.Scrolled.DY / e.cellH * 1.5))
}

func (e *Editor) gutterWidth() float32 {
	digits := len(strconv.Itoa(e.Buf.LineCount()))
	return float32(max(digits, 3)+3) * e.cellW
}

// posAt is the document position under a point in the widget.
func (e *Editor) posAt(p fyne.Position) (Pos, bool) {
	gw := e.gutterWidth()
	row := max(int(p.Y/e.cellH), 0)
	lines := e.VisibleLines(e.top, row+1)
	if len(lines) == 0 {
		return Pos{}, false
	}
	l := lines[len(lines)-1]
	if row >= len(lines) {
		return Pos{Line: l, Col: len(e.Buf.Line(l))}, p.X < gw
	}
	col := int((p.X-gw)/e.cellW+0.5) + e.left
	return Pos{Line: l, Col: runeCol(e.Buf.Line(l), max(col, 0), e.TabWidth)}, p.X < gw
}

// MouseDown implements desktop.Mouseable.
func (e *Editor) MouseDown(ev *desktop.MouseEvent) {
	if c := fyne.CurrentApp(); c != nil {
		if w := c.Driver().CanvasForObject(e); w != nil {
			w.Focus(e)
		}
	}
	if ev.Button != desktop.MouseButtonPrimary {
		return
	}
	p, gutter := e.posAt(ev.Position)
	if gutter {
		if _, folded := e.Folded(p.Line); folded || ev.Position.X > e.gutterWidth()-1.8*e.cellW {
			if e.ToggleFold(p.Line) {
				e.Refresh()
				return
			}
		}
		if e.OnGutterTap != nil {
			e.OnGutterTap(p.Line)
		}
		return
	}
	now := time.Now()
	if now.Sub(e.lastTap) < 400*time.Millisecond {
		e.tapCount++
	} else {
		e.tapCount = 1
	}
	e.lastTap = now
	e.completes = nil
	switch e.tapCount {
	case 2:
		e.Select(e.WordAt(p))
	case 3:
		end := Pos{Line: p.Line + 1, Col: 0}
		if p.Line+1 >= e.Buf.LineCount() {
			end = Pos{Line: p.Line, Col: len(e.Buf.Line(p.Line))}
		}
		e.Select(Range{Start: Pos{Line: p.Line, Col: 0}, End: end})
	default:
		e.SetCursor(p, ev.Modifier&fyne.KeyModifierShift != 0)
	}
	e.changed(false)
}

// MouseUp implements desktop.Mouseable.
func (e *Editor) MouseUp(*desktop.MouseEvent) {}

// Dragged extends the selection to the pointer.
func (e *Editor) Dragged(ev *fyne.DragEvent) {
	p, gutter := e.posAt(ev.Position)
	if gutter && !e.dragging {
		return
	}
	e.dragging = true
	e.Cur = e.Buf.Clamp(p)
	if ev.Position.Y < 0 {
		e.scrollBy(-1)
	} else if ev.Position.Y > e.Size().Height {
		e.scrollBy(1)
	}
	e.changed(false)
}

// DragEnd implements fyne.Draggable.
func (e *Editor) DragEnd() {
	e.dragging = false
	e.Buf.Seal()
}

// Cursor is the I-beam.
func (e *Editor) Cursor() desktop.Cursor { return desktop.TextCursor }

// MouseIn implements desktop.Hoverable.
func (e *Editor) MouseIn(*desktop.MouseEvent) {}

// MouseMoved shows the message of the problem under the pointer.
func (e *Editor) MouseMoved(ev *desktop.MouseEvent) {
	p, gutter := e.posAt(ev.Position)
	if gutter {
		return
	}
	for _, d := range e.diags {
		if d.Range.Start.Line == p.Line && !p.Less(d.Range.Start) && (p.Less(d.Range.End) || d.Range.Empty()) {
			if e.info != d.Message {
				e.info = d.Message
				e.Refresh()
			}
			return
		}
	}
}

// MouseOut implements desktop.Hoverable.
func (e *Editor) MouseOut() {}

// CreateRenderer implements fyne.Widget.
func (e *Editor) CreateRenderer() fyne.WidgetRenderer {
	return &renderer{e: e, bg: canvas.NewRectangle(color.Transparent)}
}

// AccessibilityLabel names the editor for assistive technology.
func (e *Editor) AccessibilityLabel() string {
	name := e.Path
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	return "Editor, " + name + ", line " + strconv.Itoa(e.Cur.Line+1)
}
