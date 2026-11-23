// Package codeedit is PyxForge's native code editor widget: one document from internal/textbuf
// drawn with Fyne, coloured by internal/syntax, with a cursor and a selection, the editing
// keys of a conventional editor, bracket matching and auto-pairing, folding, and room for
// what a language server supplies (diagnostics, completion, hover). The editing logic is in
// Model, which has no Fyne in it, so it is tested on its own.
package codeedit

import (
	"strings"
	"unicode"

	"github.com/obstinix/PyxForge/internal/textbuf"
)

// Pos and Range are the document's positions.
type (
	Pos   = textbuf.Pos
	Range = textbuf.Range
)

// Model is the editing state of one document: its text, the cursor, the selection's other
// end (the anchor) and folded regions.
type Model struct {
	Buf    *textbuf.Buffer
	Cur    Pos
	Anchor Pos
	goal   int // the display column Up and Down aim for; -1 means the cursor's own

	// Indent is what Tab inserts: a tab, or spaces. NewModel takes it from the file.
	Indent string
	// TabWidth is how many columns a tab takes on screen.
	TabWidth int
	// LineComment starts a line comment in the document's language ("" when it has none).
	LineComment string

	folds map[int]int // folded regions: first line → last line hidden under it
}

// NewModel edits buf.
func NewModel(buf *textbuf.Buffer) *Model {
	m := &Model{Buf: buf, goal: -1, TabWidth: 4, folds: map[int]int{}}
	m.Indent = guessIndent(buf)
	return m
}

// guessIndent picks a tab when lines start with tabs more often than with spaces, else the
// smallest run of leading spaces seen (2 or 4), else four spaces.
func guessIndent(b *textbuf.Buffer) string {
	tabs, spaces, unit := 0, 0, 0
	for i := 0; i < min(b.LineCount(), 2000); i++ {
		l := b.Line(i)
		switch {
		case len(l) > 0 && l[0] == '\t':
			tabs++
		case len(l) > 1 && l[0] == ' ':
			n := 0
			for n < len(l) && l[n] == ' ' {
				n++
			}
			if n < len(l) && n >= 2 {
				spaces++
				if unit == 0 || n < unit {
					unit = n
				}
			}
		}
	}
	if tabs > spaces {
		return "\t"
	}
	if unit == 2 {
		return "  "
	}
	return "    "
}

// Selection is the selected range; empty when nothing is selected.
func (m *Model) Selection() Range { return textbuf.Ordered(m.Anchor, m.Cur) }

// HasSelection reports whether text is selected.
func (m *Model) HasSelection() bool { return m.Anchor != m.Cur }

// SelectedText is the selected text.
func (m *Model) SelectedText() string { return m.Buf.Slice(m.Selection()) }

// SetCursor moves the cursor; with extend the anchor stays, selecting what lies between.
func (m *Model) SetCursor(p Pos, extend bool) {
	m.Cur = m.Buf.Clamp(p)
	if !extend {
		m.Anchor = m.Cur
	}
	m.goal = -1
	m.unfoldAt(m.Cur.Line)
	m.Buf.Seal()
}

// Select selects r, the cursor at its end.
func (m *Model) Select(r Range) {
	m.Anchor, m.Cur = m.Buf.Clamp(r.Start), m.Buf.Clamp(r.End)
	m.goal = -1
	m.Buf.Seal()
}

// SelectAll selects the whole document.
func (m *Model) SelectAll() { m.Select(Range{Start: Pos{}, End: m.Buf.End()}) }

// replace puts text in place of the selection and leaves the cursor after it.
func (m *Model) replace(text string) {
	end := m.Buf.Replace(m.Selection(), text)
	m.Cur, m.Anchor, m.goal = end, end, -1
	m.Fix()
}

// Fix keeps folds and the cursor valid after the text changed under them.
func (m *Model) Fix() {
	m.Cur, m.Anchor = m.Buf.Clamp(m.Cur), m.Buf.Clamp(m.Anchor)
	for start, end := range m.folds {
		if end >= m.Buf.LineCount() || start >= end {
			delete(m.folds, start)
		}
	}
	m.unfoldAt(m.Cur.Line)
}

var pairs = map[rune]rune{'(': ')', '[': ']', '{': '}', '"': '"', '\'': '\'', '`': '`'}

func isCloser(r rune) bool { return r == ')' || r == ']' || r == '}' }

// Type inserts what the user typed, in place of any selection. An opening bracket or quote
// also inserts its closer when the next character is not part of a word, a closer typed in
// front of the same closer steps over it, and a selection typed over with a bracket or quote
// is wrapped instead.
func (m *Model) Type(r rune) {
	line := m.Buf.Line(m.Cur.Line)
	next := rune(0)
	if m.Cur.Col < len(line) {
		next = line[m.Cur.Col]
	}
	if closer, ok := pairs[r]; ok && m.HasSelection() {
		sel := m.Selection()
		text := m.SelectedText()
		m.Buf.Replace(sel, string(r)+text+string(closer))
		m.Anchor = Pos{Line: sel.Start.Line, Col: sel.Start.Col + 1}
		m.Cur = m.Buf.Clamp(Pos{Line: sel.End.Line, Col: sel.End.Col + boolInt(sel.Start.Line == sel.End.Line)})
		m.Buf.Seal()
		return
	}
	if !m.HasSelection() && next == r && (isCloser(r) || r == '"' || r == '\'' || r == '`') {
		m.SetCursor(Pos{Line: m.Cur.Line, Col: m.Cur.Col + 1}, false)
		return
	}
	if closer, ok := pairs[r]; ok && !m.HasSelection() && !isWord(next) {
		quote := closer == r
		prev := rune(0)
		if m.Cur.Col > 0 {
			prev = line[m.Cur.Col-1]
		}
		if !quote || !isWord(prev) { // don't pair the apostrophe in "don't"
			m.replace(string(r) + string(closer))
			m.Cur.Col--
			m.Anchor = m.Cur
			return
		}
	}
	m.replace(string(r))
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// Paste inserts text in place of the selection, as one undo step.
func (m *Model) Paste(text string) {
	m.Buf.Seal()
	m.replace(text)
	m.Buf.Seal()
}

func leading(line []rune) string {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return string(line[:n])
}

// Newline breaks the line, keeping its indentation, one level deeper after an opening
// bracket or a colon; between a bracket pair the closer goes to a line of its own.
func (m *Model) Newline() {
	line := m.Buf.Line(m.Cur.Line)
	sel := m.Selection()
	ind := leading(line)
	if sel.Start.Col < len(ind) {
		ind = ind[:sel.Start.Col]
	}
	before := strings.TrimRight(string(line[:sel.Start.Col]), " \t")
	after := ""
	if l := m.Buf.Line(sel.End.Line); sel.End.Col < len(l) {
		after = string(l[sel.End.Col:])
	}
	deeper := strings.HasSuffix(before, "{") || strings.HasSuffix(before, "[") || strings.HasSuffix(before, "(") || strings.HasSuffix(before, ":")
	m.Buf.Seal()
	if deeper {
		inner := ind + m.Indent
		if a := strings.TrimLeft(after, " \t"); a != "" && isCloser([]rune(a)[0]) {
			m.replace("\n" + inner + "\n" + ind)
			m.Cur = Pos{Line: m.Cur.Line - 1, Col: len([]rune(inner))}
			m.Anchor = m.Cur
			m.Buf.Seal()
			return
		}
		m.replace("\n" + inner)
	} else {
		m.replace("\n" + ind)
	}
	m.Buf.Seal()
}

// Backspace deletes the selection, or the character before the cursor; at an empty bracket
// pair it deletes both halves, and in leading spaces it deletes back to the previous indent
// stop.
func (m *Model) Backspace() {
	if m.HasSelection() {
		m.replace("")
		return
	}
	c := m.Cur
	if c.Col == 0 {
		if c.Line == 0 {
			return
		}
		m.Anchor = Pos{Line: c.Line - 1, Col: len(m.Buf.Line(c.Line - 1))}
		m.replace("")
		return
	}
	line := m.Buf.Line(c.Line)
	prev := line[c.Col-1]
	if closer, ok := pairs[prev]; ok && c.Col < len(line) && line[c.Col] == closer {
		m.Buf.Delete(Range{Start: Pos{Line: c.Line, Col: c.Col - 1}, End: Pos{Line: c.Line, Col: c.Col + 1}})
		m.SetCursorKeep(Pos{Line: c.Line, Col: c.Col - 1})
		return
	}
	from := c.Col - 1
	if ws := leading(line); c.Col <= len(ws) && strings.Trim(ws, " ") == "" && m.Indent != "\t" {
		w := len(m.Indent)
		from = max(((c.Col-1)/w)*w, 0)
	}
	m.Anchor = Pos{Line: c.Line, Col: from}
	m.replace("")
}

// SetCursorKeep moves the cursor without ending the undo group (used inside an edit).
func (m *Model) SetCursorKeep(p Pos) {
	m.Cur = m.Buf.Clamp(p)
	m.Anchor, m.goal = m.Cur, -1
}

// Delete deletes the selection or the character after the cursor.
func (m *Model) Delete() {
	if m.HasSelection() {
		m.replace("")
		return
	}
	c := m.Cur
	end := Pos{Line: c.Line, Col: c.Col + 1}
	if c.Col >= len(m.Buf.Line(c.Line)) {
		if c.Line+1 >= m.Buf.LineCount() {
			return
		}
		end = Pos{Line: c.Line + 1, Col: 0}
	}
	m.Buf.Delete(Range{Start: c, End: end})
	m.Fix()
}

// lines is the range of lines the selection touches; a selection ending at column 0 does not
// take in that line.
func (m *Model) lines() (first, last int) {
	s := m.Selection()
	first, last = s.Start.Line, s.End.Line
	if last > first && s.End.Col == 0 {
		last--
	}
	return
}

// Tab indents: with a selection over lines, each line; otherwise it inserts the indent unit.
func (m *Model) Tab() {
	if !m.HasSelection() || m.Selection().Start.Line == m.Selection().End.Line {
		if m.Indent == "\t" {
			m.replace("\t")
			return
		}
		col := displayCol(m.Buf.Line(m.Cur.Line), m.Cur.Col, m.TabWidth)
		w := len(m.Indent)
		m.replace(strings.Repeat(" ", w-col%w))
		return
	}
	first, last := m.lines()
	m.Buf.Seal()
	for l := first; l <= last; l++ {
		if len(m.Buf.Line(l)) > 0 {
			m.Buf.Insert(Pos{Line: l, Col: 0}, m.Indent)
		}
	}
	m.Buf.Seal()
	m.Anchor = Pos{Line: first, Col: 0}
	m.Cur = Pos{Line: last, Col: len(m.Buf.Line(last))}
}

// Outdent removes one level of indentation from each line the selection touches.
func (m *Model) Outdent() {
	first, last := m.lines()
	m.Buf.Seal()
	for l := first; l <= last; l++ {
		line := m.Buf.Line(l)
		n := 0
		switch {
		case len(line) > 0 && line[0] == '\t':
			n = 1
		default:
			for n < len(line) && n < len(m.Indent) && line[n] == ' ' {
				n++
			}
		}
		if n > 0 {
			m.Buf.Delete(Range{Start: Pos{Line: l, Col: 0}, End: Pos{Line: l, Col: n}})
		}
	}
	m.Buf.Seal()
	m.Fix()
}

// ToggleComment comments out the lines the selection touches, or uncomments them when every
// non-blank one already starts with the comment marker.
func (m *Model) ToggleComment() {
	if m.LineComment == "" {
		return
	}
	first, last := m.lines()
	marker := m.LineComment + " "
	all := true
	for l := first; l <= last; l++ {
		t := strings.TrimLeft(string(m.Buf.Line(l)), " \t")
		if t != "" && !strings.HasPrefix(t, m.LineComment) {
			all = false
		}
	}
	m.Buf.Seal()
	for l := first; l <= last; l++ {
		line := m.Buf.Line(l)
		ind := len([]rune(leading(line)))
		rest := string(line[ind:])
		switch {
		case rest == "":
		case all && strings.HasPrefix(rest, marker):
			m.Buf.Delete(Range{Start: Pos{Line: l, Col: ind}, End: Pos{Line: l, Col: ind + len([]rune(marker))}})
		case all:
			m.Buf.Delete(Range{Start: Pos{Line: l, Col: ind}, End: Pos{Line: l, Col: ind + len([]rune(m.LineComment))}})
		default:
			m.Buf.Insert(Pos{Line: l, Col: ind}, marker)
		}
	}
	m.Buf.Seal()
	m.Fix()
}

// Undo and Redo move the cursor to where the change was.
func (m *Model) Undo() {
	if p, ok := m.Buf.Undo(); ok {
		m.SetCursor(p, false)
		m.Fix()
	}
}

// Redo applies the last undone change again.
func (m *Model) Redo() {
	if p, ok := m.Buf.Redo(); ok {
		m.SetCursor(p, false)
		m.Fix()
	}
}

// displayCol is where rune column col of line is on screen, tabs expanded.
func displayCol(line []rune, col, tab int) int {
	d := 0
	for i := 0; i < col && i < len(line); i++ {
		if line[i] == '\t' {
			d += tab - d%tab
		} else {
			d++
		}
	}
	return d
}

// runeCol is the rune column that display column d falls in.
func runeCol(line []rune, d, tab int) int {
	x := 0
	for i, r := range line {
		w := 1
		if r == '\t' {
			w = tab - x%tab
		}
		if x+w > d {
			if d-x > w/2 && r == '\t' {
				return i + 1
			}
			return i
		}
		x += w
	}
	return len(line)
}

// Movement.
type Move int

const (
	Left Move = iota
	Right
	Up
	Down
	WordLeft
	WordRight
	LineStart // first non-blank, then column 0
	LineEnd
	DocStart
	DocEnd
)

// MoveCursor moves the cursor; extend keeps the anchor. page is how many lines Up and Down
// move (1 for the arrow keys).
func (m *Model) MoveCursor(mv Move, extend bool, page int) {
	c := m.Cur
	if !extend && m.HasSelection() && (mv == Left || mv == Right) {
		s := m.Selection()
		if mv == Left {
			m.SetCursor(s.Start, false)
		} else {
			m.SetCursor(s.End, false)
		}
		return
	}
	line := m.Buf.Line(c.Line)
	goal := m.goal
	switch mv {
	case Left:
		if c.Col > 0 {
			c.Col--
		} else if c.Line > 0 {
			c = Pos{Line: c.Line - 1, Col: len(m.Buf.Line(c.Line - 1))}
		}
	case Right:
		if c.Col < len(line) {
			c.Col++
		} else if c.Line+1 < m.Buf.LineCount() {
			c = Pos{Line: c.Line + 1, Col: 0}
		}
	case Up, Down:
		if goal < 0 {
			goal = displayCol(line, c.Col, m.TabWidth)
		}
		l := c.Line
		for i := 0; i < max(page, 1); i++ {
			if mv == Up {
				l = m.prevVisible(l)
			} else {
				l = m.nextVisible(l)
			}
		}
		if l == c.Line {
			if mv == Up {
				c.Col = 0
			} else {
				c.Col = len(line)
			}
			goal = -1
		} else {
			c = Pos{Line: l, Col: runeCol(m.Buf.Line(l), goal, m.TabWidth)}
		}
	case WordLeft:
		c = m.wordLeft(c)
	case WordRight:
		c = m.wordRight(c)
	case LineStart:
		ind := len([]rune(leading(line)))
		if c.Col == ind {
			c.Col = 0
		} else {
			c.Col = ind
		}
	case LineEnd:
		c.Col = len(line)
	case DocStart:
		c = Pos{}
	case DocEnd:
		c = m.Buf.End()
	}
	m.SetCursor(c, extend)
	if mv == Up || mv == Down {
		m.goal = goal
	}
}

func class(r rune) int {
	switch {
	case isWord(r):
		return 2
	case unicode.IsSpace(r):
		return 0
	}
	return 1
}

func (m *Model) wordLeft(c Pos) Pos {
	if c.Col == 0 {
		if c.Line == 0 {
			return c
		}
		return Pos{Line: c.Line - 1, Col: len(m.Buf.Line(c.Line - 1))}
	}
	line := m.Buf.Line(c.Line)
	i := c.Col
	for i > 0 && class(line[i-1]) == 0 {
		i--
	}
	if i > 0 {
		k := class(line[i-1])
		for i > 0 && class(line[i-1]) == k {
			i--
		}
	}
	return Pos{Line: c.Line, Col: i}
}

func (m *Model) wordRight(c Pos) Pos {
	line := m.Buf.Line(c.Line)
	if c.Col >= len(line) {
		if c.Line+1 >= m.Buf.LineCount() {
			return c
		}
		return Pos{Line: c.Line + 1, Col: 0}
	}
	i := c.Col
	k := class(line[i])
	for i < len(line) && class(line[i]) == k {
		i++
	}
	for i < len(line) && class(line[i]) == 0 {
		i++
	}
	return Pos{Line: c.Line, Col: i}
}

// WordAt is the word (or run of punctuation) under p.
func (m *Model) WordAt(p Pos) Range {
	line := m.Buf.Line(p.Line)
	if len(line) == 0 {
		return Range{Start: p, End: p}
	}
	i := min(p.Col, len(line)-1)
	k := class(line[i])
	s, e := i, i
	for s > 0 && class(line[s-1]) == k {
		s--
	}
	for e < len(line) && class(line[e]) == k {
		e++
	}
	return Range{Start: Pos{Line: p.Line, Col: s}, End: Pos{Line: p.Line, Col: e}}
}

// IdentifierBefore is the identifier the cursor is at the end of, for completion.
func (m *Model) IdentifierBefore() string {
	line := m.Buf.Line(m.Cur.Line)
	i := m.Cur.Col
	for i > 0 && isWord(line[i-1]) {
		i--
	}
	return string(line[i:m.Cur.Col])
}

var openers = map[rune]rune{')': '(', ']': '[', '}': '{'}

// MatchBracket finds the bracket that pairs with the one at p, or just before p; ok is false
// when there is none there or it has no partner. It does not look inside strings or comments
// (the highlighter's spans would be needed), which is how most editors behave too.
func (m *Model) MatchBracket(p Pos) (at, match Pos, ok bool) {
	try := func(q Pos) (Pos, Pos, bool) {
		line := m.Buf.Line(q.Line)
		if q.Col < 0 || q.Col >= len(line) {
			return q, q, false
		}
		r := line[q.Col]
		if c, isOpen := pairs[r]; isOpen && isCloser(c) {
			if mt, ok := m.scan(q, r, c, 1); ok {
				return q, mt, true
			}
		}
		if o, isClose := openers[r]; isClose {
			if mt, ok := m.scan(q, r, o, -1); ok {
				return q, mt, true
			}
		}
		return q, q, false
	}
	if a, b, ok := try(p); ok {
		return a, b, true
	}
	return try(Pos{Line: p.Line, Col: p.Col - 1})
}

const scanLimit = 20000 // lines a bracket search looks through

func (m *Model) scan(from Pos, self, other rune, dir int) (Pos, bool) {
	depth := 0
	l, c := from.Line, from.Col
	for n := 0; n < scanLimit; {
		line := m.Buf.Line(l)
		for c >= 0 && c < len(line) {
			switch line[c] {
			case self:
				depth++
			case other:
				depth--
				if depth == 0 {
					return Pos{Line: l, Col: c}, true
				}
			}
			c += dir
		}
		l += dir
		n++
		if l < 0 || l >= m.Buf.LineCount() {
			return Pos{}, false
		}
		if dir > 0 {
			c = 0
		} else {
			c = len(m.Buf.Line(l)) - 1
		}
	}
	return Pos{}, false
}

// FoldRange is the region that folding line l would hide: to the bracket closing the last
// opening bracket on l, or else the lines indented deeper than l. ok is false when there is
// nothing to fold.
func (m *Model) FoldRange(l int) (last int, ok bool) {
	line := m.Buf.Line(l)
	t := strings.TrimRight(string(line), " \t")
	if t == "" {
		return 0, false
	}
	rs := []rune(t)
	for i := len(rs) - 1; i >= 0; i-- {
		if c, open := pairs[rs[i]]; open && isCloser(c) {
			if mt, found := m.scan(Pos{Line: l, Col: i}, rs[i], c, 1); found && mt.Line > l {
				end := mt.Line - 1
				if strings.TrimSpace(string(m.Buf.Line(mt.Line)[:mt.Col])) != "" {
					end = mt.Line
				}
				return end, end > l
			}
			break
		}
	}
	base := displayCol(line, len([]rune(leading(line))), m.TabWidth)
	last = l
	for n := l + 1; n < m.Buf.LineCount(); n++ {
		nl := m.Buf.Line(n)
		if strings.TrimSpace(string(nl)) == "" {
			continue
		}
		if displayCol(nl, len([]rune(leading(nl))), m.TabWidth) <= base {
			break
		}
		last = n
	}
	return last, last > l
}

// ToggleFold folds the region starting at line l, or unfolds it. It reports whether anything
// changed.
func (m *Model) ToggleFold(l int) bool {
	if _, ok := m.folds[l]; ok {
		delete(m.folds, l)
		return true
	}
	last, ok := m.FoldRange(l)
	if !ok {
		return false
	}
	m.folds[l] = last
	if m.Cur.Line > l && m.Cur.Line <= last {
		m.SetCursor(Pos{Line: l, Col: len(m.Buf.Line(l))}, false)
	}
	return true
}

// Folded reports whether line l starts a folded region, and its last hidden line.
func (m *Model) Folded(l int) (int, bool) {
	e, ok := m.folds[l]
	return e, ok
}

// UnfoldAll opens every fold.
func (m *Model) UnfoldAll() { clear(m.folds) }

func (m *Model) unfoldAt(l int) {
	for s, e := range m.folds {
		if l > s && l <= e {
			delete(m.folds, s)
		}
	}
}

// LineHidden reports whether line l is inside a fold.
func (m *Model) LineHidden(l int) bool {
	for s, e := range m.folds {
		if l > s && l <= e {
			return true
		}
	}
	return false
}

func (m *Model) nextVisible(l int) int {
	if e, ok := m.folds[l]; ok {
		l = e
	}
	if l+1 < m.Buf.LineCount() {
		return l + 1
	}
	return l
}

func (m *Model) prevVisible(l int) int {
	for n := l - 1; n >= 0; n-- {
		if !m.LineHidden(n) {
			return n
		}
	}
	return l
}

// VisibleLines returns up to n visible line numbers starting at line from (or the first visible
// line after it).
func (m *Model) VisibleLines(from, n int) []int {
	out := make([]int, 0, n)
	l := from
	for l < m.Buf.LineCount() && m.LineHidden(l) {
		l++
	}
	for len(out) < n && l < m.Buf.LineCount() {
		out = append(out, l)
		nl := m.nextVisible(l)
		if nl == l {
			break
		}
		l = nl
	}
	return out
}

// Find returns the next match of query from p (wrapping around), or false.
func (m *Model) Find(query string, from Pos, caseSensitive, backward bool) (Range, bool) {
	if query == "" {
		return Range{}, false
	}
	q := []rune(query)
	if !caseSensitive {
		q = []rune(strings.ToLower(query))
	}
	n := m.Buf.LineCount()
	matchAt := func(l int) []int {
		line := m.Buf.Line(l)
		if !caseSensitive {
			line = []rune(strings.ToLower(string(line)))
		}
		var hits []int
		for i := 0; i+len(q) <= len(line); i++ {
			if string(line[i:i+len(q)]) == string(q) {
				hits = append(hits, i)
			}
		}
		return hits
	}
	if strings.Contains(query, "\n") {
		return Range{}, false
	}
	for k := 0; k <= n; k++ {
		var l int
		if backward {
			l = ((from.Line-k)%n + n) % n
		} else {
			l = (from.Line + k) % n
		}
		hits := matchAt(l)
		if backward {
			for i := len(hits) - 1; i >= 0; i-- {
				if k > 0 || hits[i] < from.Col {
					return Range{Start: Pos{Line: l, Col: hits[i]}, End: Pos{Line: l, Col: hits[i] + len(q)}}, true
				}
			}
		} else {
			for _, h := range hits {
				if k > 0 || h >= from.Col {
					return Range{Start: Pos{Line: l, Col: h}, End: Pos{Line: l, Col: h + len(q)}}, true
				}
			}
		}
	}
	return Range{}, false
}

// ReplaceAll replaces every match of query and reports how many there were, as one undo step.
func (m *Model) ReplaceAll(query, with string, caseSensitive bool) int {
	if query == "" {
		return 0
	}
	n := 0
	m.Buf.Seal()
	p := Pos{}
	for {
		r, ok := m.Find(query, p, caseSensitive, false)
		if !ok || r.Start.Less(p) {
			break
		}
		p = m.Buf.Replace(r, with)
		n++
	}
	m.Buf.Seal()
	m.Fix()
	return n
}
