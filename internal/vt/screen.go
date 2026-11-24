// Package vt is the Terminal panel's terminal emulator (D17): it reads a program's output, the
// xterm escape sequences included, into a grid of cells with a scrollback history. It draws
// nothing and runs nothing; the pseudo-terminal is internal/pty's, the drawing the panel's.
//
// It covers what shells, compilers, debuggers and full-screen programs (less, vim, htop) send:
// cursor movement, erasing, scroll regions, inserting and deleting, 16, 256 and 24-bit colours,
// the alternate screen, DEC line drawing, bracketed paste, and the status and cursor-position
// reports programs ask for. A Screen is safe for use from several goroutines.
package vt

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// Color is a cell's colour: Default, one of the 256 indexed colours, or 24-bit RGB.
type Color uint32

// Default is the terminal's own foreground or background.
const Default Color = 0

const (
	indexedFlag Color = 1 << 24
	rgbFlag     Color = 2 << 24
)

// Indexed is palette colour i: 0–7 the ANSI colours, 8–15 their bright forms, then xterm's
// 6×6×6 cube and greys.
func Indexed(i uint8) Color { return indexedFlag | Color(i) }

// RGB is a 24-bit colour.
func RGB(r, g, b uint8) Color { return rgbFlag | Color(r)<<16 | Color(g)<<8 | Color(b) }

// Index is c's palette index, when c is indexed.
func (c Color) Index() (uint8, bool) { return uint8(c), c&^0xFFFFFF == indexedFlag }

// RGB is c's red, green and blue, when c is a 24-bit colour.
func (c Color) RGB() (r, g, b uint8, ok bool) {
	return uint8(c >> 16), uint8(c >> 8), uint8(c), c&^0xFFFFFF == rgbFlag
}

// Attr is a set of cell attributes.
type Attr uint16

const (
	Bold Attr = 1 << iota
	Faint
	Italic
	Underline
	Blink
	Inverse
	Hidden
	Strike
)

// Cell is one character position.
type Cell struct {
	R      rune // the character; 0 is blank
	FG, BG Color
	A      Attr
	// Width is 1, or 2 for the first cell of a wide character, whose second cell has 0. A
	// blank cell has width 1.
	Width int8
}

// Line is a row of cells. Wrapped marks a line that continues on the next: a long line the
// terminal wrapped, not one the program ended.
type Line struct {
	Cells   []Cell
	Wrapped bool
}

// MouseMode is which mouse events the program asked for.
type MouseMode int

const (
	MouseOff    MouseMode = 0
	MouseClick  MouseMode = 1000 // presses and releases
	MouseDrag   MouseMode = 1002 // and moves with a button down
	MouseMotion MouseMode = 1003 // and every move
)

// DefaultHistory is the scrollback kept, in lines: Neovim's terminal default.
const DefaultHistory = 10000

type charset uint8

const (
	ascii charset = iota
	decGraphics
)

type cursor struct {
	x, y     int
	pen      Cell
	origin   bool
	g        [2]charset
	gl       int
	wrapNext bool
}

// Screen is the emulated terminal.
type Screen struct {
	mu         sync.Mutex
	cols, rows int
	main, alt  []Line
	lines      []Line // main or alt, whichever shows
	altOn      bool
	history    []Line // lines scrolled off the top of the main screen, oldest first
	dropped    int    // history lines dropped from the start (MaxHistory)
	maxHistory int

	cur      cursor
	saved    cursor // DECSC
	altSaved cursor // 1049
	top, bot int    // the scroll region, inclusive
	tabs     []bool

	insert, lnm, autowrap, hidden   bool
	appCursor, appKeypad, bracketed bool
	mouse                           MouseMode
	sgrMouse, focusEvents           bool
	title                           string
	last                            rune // the last character printed, for REP
	reply                           func([]byte)
	carry                           []byte // an incomplete UTF-8 sequence from the last Write
	p                               parser
	gen                             uint64
}

// New is a blank screen of cols by rows keeping DefaultHistory lines of scrollback.
func New(cols, rows int) *Screen {
	s := &Screen{maxHistory: DefaultHistory}
	s.cols, s.rows = max(cols, 2), max(rows, 1)
	s.reset()
	return s
}

// SetReply sets where the answers to a program's queries (cursor position, device
// attributes) go: the program's input. They are sent from the goroutine calling Write.
func (s *Screen) SetReply(f func([]byte)) {
	s.mu.Lock()
	s.reply = f
	s.mu.Unlock()
}

// SetMaxHistory sets how many scrollback lines are kept.
func (s *Screen) SetMaxHistory(n int) {
	s.mu.Lock()
	s.maxHistory = max(n, 0)
	s.trimHistory()
	s.mu.Unlock()
}

func (s *Screen) reset() {
	s.main = blankLines(s.cols, s.rows, Cell{Width: 1})
	s.alt = blankLines(s.cols, s.rows, Cell{Width: 1})
	s.lines, s.altOn = s.main, false
	s.cur = cursor{}
	s.saved, s.altSaved = cursor{}, cursor{}
	s.top, s.bot = 0, s.rows-1
	s.insert, s.lnm, s.autowrap, s.hidden = false, false, true, false
	s.appCursor, s.appKeypad, s.bracketed = false, false, false
	s.mouse, s.sgrMouse, s.focusEvents = MouseOff, false, false
	s.resetTabs()
	s.gen++
}

func (s *Screen) resetTabs() {
	s.tabs = make([]bool, s.cols)
	for i := 8; i < s.cols; i += 8 {
		s.tabs[i] = true
	}
}

func blankLines(cols, rows int, blank Cell) []Line {
	ls := make([]Line, rows)
	for i := range ls {
		ls[i] = blankLine(cols, blank)
	}
	return ls
}

func blankLine(cols int, blank Cell) Line {
	cs := make([]Cell, cols)
	for i := range cs {
		cs[i] = blank
	}
	return Line{Cells: cs}
}

// blank is an erased cell: no character, the pen's background (xterm's background colour erase).
func (s *Screen) blank() Cell { return Cell{BG: s.cur.pen.BG, Width: 1} }

// Write reads a program's output into the screen. It never fails.
func (s *Screen) Write(b []byte) (int, error) {
	s.mu.Lock()
	var replies [][]byte
	s.p.replies = &replies
	data := b
	if len(s.carry) > 0 {
		data = append(s.carry, b...)
		s.carry = nil
	}
	for i := 0; i < len(data); {
		c := data[i]
		if c < utf8.RuneSelf || s.p.state != ground {
			// Escape sequences are ASCII; text inside an OSC string is kept as bytes.
			if c >= utf8.RuneSelf && s.p.state == osc {
				s.p.str = append(s.p.str, c)
				i++
				continue
			}
			if c >= utf8.RuneSelf && (s.p.state == str || s.p.state == strEsc) {
				i++ // inside an ignored string
				continue
			}
			if c >= utf8.RuneSelf {
				s.p.state = ground // a stray byte ends a sequence, then prints
				continue
			}
			s.byte(c)
			i++
			continue
		}
		if !utf8.FullRune(data[i:]) {
			s.carry = append([]byte(nil), data[i:]...)
			break
		}
		r, n := utf8.DecodeRune(data[i:])
		s.print(r)
		i += n
	}
	s.gen++
	reply := s.reply
	s.p.replies = nil
	s.mu.Unlock()
	if reply != nil {
		for _, r := range replies {
			reply(r)
		}
	}
	return len(b), nil
}

func (s *Screen) send(b string) {
	if s.p.replies != nil {
		*s.p.replies = append(*s.p.replies, []byte(b))
	}
}

// print puts a character at the cursor and moves it on, wrapping at the right margin.
func (s *Screen) print(r rune) {
	if s.cur.g[s.cur.gl] == decGraphics && r >= 0x5f && r <= 0x7e {
		r = decSpecial[r-0x5f]
	}
	w := runewidth.RuneWidth(r)
	if w == 0 {
		return // combining marks and zero-width characters are not kept
	}
	if s.cur.wrapNext && s.autowrap {
		s.lines[s.cur.y].Wrapped = true
		s.cur.x = 0
		s.lineFeed()
	}
	s.cur.wrapNext = false
	if w == 2 && s.cur.x == s.cols-1 {
		if !s.autowrap {
			return
		}
		s.clearCell(s.cur.y, s.cur.x)
		s.lines[s.cur.y].Wrapped = true
		s.cur.x = 0
		s.lineFeed()
	}
	line := s.lines[s.cur.y].Cells
	if s.insert {
		copy(line[s.cur.x+w:], line[s.cur.x:])
	}
	s.clearCell(s.cur.y, s.cur.x)
	if w == 2 {
		s.clearCell(s.cur.y, s.cur.x+1)
	}
	c := s.cur.pen
	c.R, c.Width = r, int8(w)
	line[s.cur.x] = c
	if w == 2 {
		c.R, c.Width = 0, 0
		line[s.cur.x+1] = c
	}
	s.last = r
	s.cur.x += w
	if s.cur.x >= s.cols {
		s.cur.x = s.cols - 1
		s.cur.wrapNext = true
	}
}

// clearCell blanks a cell about to be overwritten, and the other half of a wide character it
// is part of, so no half is left behind.
func (s *Screen) clearCell(y, x int) {
	line := s.lines[y].Cells
	if x < 0 || x >= len(line) {
		return
	}
	switch line[x].Width {
	case 2:
		if x+1 < len(line) {
			line[x+1] = Cell{BG: line[x+1].BG, Width: 1}
		}
	case 0:
		if line[x].R == 0 && x > 0 && line[x-1].Width == 2 {
			line[x-1] = Cell{BG: line[x-1].BG, Width: 1}
		}
	}
	line[x] = Cell{BG: line[x].BG, Width: 1}
}

// lineFeed moves the cursor down a line, scrolling the region at its bottom.
func (s *Screen) lineFeed() {
	switch {
	case s.cur.y == s.bot:
		s.scrollUp(1)
	case s.cur.y < s.rows-1:
		s.cur.y++
	}
}

// scrollUp moves the scroll region's lines up n, blank lines entering at its bottom. Lines that
// leave the top of the main screen go to the history.
func (s *Screen) scrollUp(n int) {
	n = min(n, s.bot-s.top+1)
	if n <= 0 {
		return
	}
	if s.top == 0 && !s.altOn {
		for i := range n {
			s.history = append(s.history, s.lines[i])
		}
		s.trimHistory()
	}
	copy(s.lines[s.top:], s.lines[s.top+n:s.bot+1])
	for i := s.bot - n + 1; i <= s.bot; i++ {
		s.lines[i] = blankLine(s.cols, s.blank())
	}
}

// scrollDown moves the scroll region's lines down n, blank lines entering at its top.
func (s *Screen) scrollDown(n int) {
	n = min(n, s.bot-s.top+1)
	if n <= 0 {
		return
	}
	copy(s.lines[s.top+n:s.bot+1], s.lines[s.top:s.bot+1-n])
	for i := s.top; i < s.top+n; i++ {
		s.lines[i] = blankLine(s.cols, s.blank())
	}
}

func (s *Screen) trimHistory() {
	if over := len(s.history) - s.maxHistory; over > 0 {
		s.history = append(s.history[:0:0], s.history[over:]...)
		s.dropped += over
	}
}

// Resize gives the screen a new size. Lines are cut or padded, not re-wrapped. When rows go,
// the lines below the cursor go first, then the top lines move to the history, so the cursor's
// line stays.
func (s *Screen) Resize(cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cols, rows = max(cols, 2), max(rows, 1)
	if cols == s.cols && rows == s.rows {
		return
	}
	fit := func(ls []Line, keepTop bool, cy int) ([]Line, int) {
		for i := range ls {
			ls[i] = fitLine(ls[i], cols)
		}
		for len(ls) > rows && len(ls)-1 > cy { // below the cursor
			ls = ls[:len(ls)-1]
		}
		for len(ls) > rows {
			if keepTop {
				s.history = append(s.history, ls[0])
			}
			ls = ls[1:]
			cy--
		}
		for len(ls) < rows {
			ls = append(ls, blankLine(cols, Cell{Width: 1}))
		}
		return ls, cy
	}
	mainY, altY := s.cur.y, s.cur.y
	if s.altOn {
		mainY = s.altSaved.y
	}
	s.main, mainY = fit(s.main, true, mainY)
	s.alt, altY = fit(s.alt, false, altY)
	s.trimHistory()
	s.cols, s.rows = cols, rows
	if s.altOn {
		s.lines = s.alt
		s.cur.y = altY
		s.altSaved.y = clamp(mainY, 0, rows-1)
	} else {
		s.lines = s.main
		s.cur.y = mainY
	}
	s.cur.x = clamp(s.cur.x, 0, cols-1)
	s.cur.y = clamp(s.cur.y, 0, rows-1)
	s.cur.wrapNext = false
	s.saved.x, s.saved.y = clamp(s.saved.x, 0, cols-1), clamp(s.saved.y, 0, rows-1)
	s.top, s.bot = 0, rows-1
	s.resetTabs()
	s.gen++
}

func fitLine(l Line, cols int) Line {
	switch {
	case len(l.Cells) > cols:
		l.Cells = l.Cells[:cols:cols]
		if last := &l.Cells[cols-1]; last.Width == 2 {
			*last = Cell{BG: last.BG, Width: 1}
		}
		l.Wrapped = false
	case len(l.Cells) < cols:
		cs := make([]Cell, cols)
		for i := copy(cs, l.Cells); i < cols; i++ {
			cs[i].Width = 1
		}
		l.Cells = cs
		l.Wrapped = false
	}
	return l
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

// Size is the screen's columns and rows.
func (s *Screen) Size() (cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cols, s.rows
}

// View is what shows of the screen scrolled back some lines into its history.
type View struct {
	Lines []Line // the rows on view, top first; copies
	// CursorX and CursorY are the cursor's cell in Lines; CursorVisible is false when the
	// program hid it or it is scrolled out of view.
	CursorX, CursorY int
	CursorVisible    bool
	Back             int // the lines scrolled back, after clamping
	History          int // the history's length
	// First is the absolute line number of Lines[0] (see Pos).
	First int
}

// View is the screen scrolled back back lines (0: the screen itself).
func (s *Screen) View(back int) View {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.altOn {
		back = 0 // a full-screen program has no history
	}
	back = clamp(back, 0, len(s.history))
	v := View{Back: back, History: len(s.history), Lines: make([]Line, s.rows)}
	start := len(s.history) - back
	v.First = s.dropped + start
	for i := range s.rows {
		var l Line
		if j := start + i; j < len(s.history) {
			l = fitLine(s.history[j], s.cols)
		} else {
			l = s.lines[j-len(s.history)]
		}
		v.Lines[i] = Line{Cells: append([]Cell(nil), l.Cells...), Wrapped: l.Wrapped}
	}
	v.CursorX, v.CursorY = s.cur.x, s.cur.y+back
	v.CursorVisible = !s.hidden && v.CursorY < s.rows
	return v
}

// Pos is a cell by absolute line number: lines count from the first line the terminal ever
// had, so a position stays on its line as output scrolls.
type Pos struct{ Line, Col int }

// Text is the text from a to b (inclusive of a, exclusive of b), lines a long line wrapped
// into joined back together, trailing blanks dropped.
func (s *Screen) Text(a, b Pos) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.Line < a.Line || (b.Line == a.Line && b.Col < a.Col) {
		a, b = b, a
	}
	var out strings.Builder
	for ln := a.Line; ln <= b.Line; ln++ {
		l, ok := s.line(ln)
		if !ok {
			continue
		}
		from, to := 0, len(l.Cells)
		if ln == a.Line {
			from = min(a.Col, to)
		}
		if ln == b.Line {
			to = clamp(b.Col, from, to)
		}
		out.WriteString(cellText(l.Cells[from:to], !(ln == b.Line && to < len(l.Cells))))
		if ln < b.Line && !l.Wrapped {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// line is absolute line ln, history or screen.
func (s *Screen) line(ln int) (Line, bool) {
	i := ln - s.dropped
	switch {
	case i < 0:
		return Line{}, false
	case i < len(s.history):
		return s.history[i], true
	case i-len(s.history) < s.rows:
		return s.lines[i-len(s.history)], true
	}
	return Line{}, false
}

func cellText(cs []Cell, trim bool) string {
	var b strings.Builder
	for i, c := range cs {
		if i > 0 && cs[i-1].Width == 2 {
			continue // a wide character's second half
		}
		if c.R == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteRune(c.R)
		}
	}
	if trim {
		return strings.TrimRight(b.String(), " ")
	}
	return b.String()
}

// Lines is every line, history then screen, as text, a long line the terminal wrapped joined
// back into one, trailing blanks and blank lines at the end dropped.
func (s *Screen) Lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := append(append([]Line(nil), s.history...), s.lines...)
	var out []string
	var cur strings.Builder
	for _, l := range all {
		cur.WriteString(cellText(l.Cells, !l.Wrapped))
		if !l.Wrapped {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// String is Lines joined by newlines.
func (s *Screen) String() string { return strings.Join(s.Lines(), "\n") }

// ClearHistory drops the scrollback.
func (s *Screen) ClearHistory() {
	s.mu.Lock()
	s.dropped += len(s.history)
	s.history = nil
	s.gen++
	s.mu.Unlock()
}

// Modes is the input modes a program set, which decide what keys and the mouse send.
type Modes struct {
	AppCursor      bool // arrows send ESC O A, not ESC [ A
	AppKeypad      bool
	BracketedPaste bool
	Mouse          MouseMode
	SGRMouse       bool // mouse reports in the 1006 form
	FocusEvents    bool
	AltScreen      bool // a full-screen program's screen shows
}

// Modes is the program's input modes.
func (s *Screen) Modes() Modes {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Modes{s.appCursor, s.appKeypad, s.bracketed, s.mouse, s.sgrMouse, s.focusEvents, s.altOn}
}

// Title is the window title the program set (OSC 0 or 2).
func (s *Screen) Title() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.title
}

// Generation changes whenever the screen does.
func (s *Screen) Generation() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gen
}

// decSpecial is the DEC special graphics set for 0x5f–0x7e: the line-drawing characters.
var decSpecial = [...]rune{
	' ', '◆', '▒', '␉', '␌', '␍', '␊', '°', '±', '␤', '␋', '┘', '┐', '┌', '└', '┼',
	'⎺', '⎻', '─', '⎼', '⎽', '├', '┤', '┴', '┬', '│', '≤', '≥', 'π', '≠', '£', '·',
}
