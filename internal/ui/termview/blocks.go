package termview

import (
	"fmt"
	"image/color"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/term"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/vt"
)

// Command blocks (D24): with BlocksOn, the commands an integrated shell marked are drawn as
// blocks: a 3 px bar in the gutter coloured by how the command ended, a hairline between
// blocks, a header with the command and its actions on hover, a single summary line for a
// collapsed block, and the command pinned at the top while its long output scrolls.

// Inset is the space around the grid; Gutter is the strip left of it for block bars.
const (
	Inset  = theme.Space3
	Gutter = 8
)

// origin is where the grid's first cell is drawn.
func origin() fyne.Position { return fyne.NewPos(Inset+Gutter, Inset) }

// row is one row on view: a line of the screen, or a collapsed block's summary.
type row struct {
	abs     int     // the absolute line, or the collapsed block's prompt line
	line    vt.Line // its cells
	summary int     // the collapsed block's index in v.blocks, or -1
}

// display is what the view shows now.
type display struct {
	rows          []row
	cursorRow     int // -1: not on view
	cursorX       int
	cursorVisible bool
	back, total   int // rows scrolled back, and rows there are to scroll through
}

// blocksNow are the screen's blocks, recomputed when its marks or its length change.
func (v *View) blocksNow() []term.Block {
	if !v.BlocksOn || v.screen == nil {
		v.blocks = nil
		return nil
	}
	key := [3]uint64{v.screen.MarkSeq(), uint64(v.screen.EndLine()), uint64(v.screen.FirstLine())}
	if key != v.blockKey || v.blockScreen != v.screen {
		v.blocks, v.blockKey, v.blockScreen = term.Blocks(v.screen), key, v.screen
		if key[0] != v.linkSeq {
			v.linkCache, v.linkSeq = nil, key[0] // a new command may have made the files a link names
		}
	}
	return v.blocks
}

// collapsed is the set of collapsed blocks of the screen shown, by their Seq.
func (v *View) collapsed() map[uint64]bool {
	if v.collapsedBy == nil {
		v.collapsedBy = map[*vt.Screen]map[uint64]bool{}
	}
	m := v.collapsedBy[v.screen]
	if m == nil {
		m = map[uint64]bool{}
		v.collapsedBy[v.screen] = m
	}
	return m
}

// blockAt is the index of the block absolute line ln is in, or -1.
func blockAt(bs []term.Block, ln int) int {
	i := sort.Search(len(bs), func(i int) bool { return bs[i].PromptLine > ln }) - 1
	if i >= 0 && ln < max(bs[i].Output[1], bs[i].PromptLine+1) {
		return i
	}
	return -1
}

// compute works out the rows on view. Without collapsed blocks (or on a full-screen
// program's screen) they are the screen's own view; with them, every kept line is laid out
// with each collapsed block folded into one summary row.
func (v *View) compute() display {
	sc := v.screen
	bs := v.blocksNow()
	folded := false
	if len(bs) > 0 && !sc.AltScreen() {
		for _, b := range bs {
			if v.collapsed()[b.Seq] && !b.Running {
				folded = true
				break
			}
		}
	}
	if !folded {
		vw := sc.View(v.back)
		d := display{back: vw.Back, total: vw.History + len(vw.Lines), cursorRow: -1}
		for i, l := range vw.Lines {
			d.rows = append(d.rows, row{abs: vw.First + i, line: l, summary: -1})
		}
		if vw.CursorVisible && vw.CursorY < len(vw.Lines) {
			d.cursorRow, d.cursorX, d.cursorVisible = vw.CursorY, vw.CursorX, true
		}
		return d
	}
	all := v.entries()
	end := sc.EndLine()
	rows := v.rows
	d := display{total: len(all), cursorRow: -1}
	d.back = max(0, min(v.back, len(all)-rows))
	start := max(0, len(all)-rows-d.back)
	vw := sc.View(0)
	cursorAbs := end - len(vw.Lines) + vw.CursorY
	for i := start; i < min(start+rows, len(all)); i++ {
		e := all[i]
		r := row{abs: e.abs, summary: e.summary}
		if e.summary < 0 {
			r.line, _ = sc.LineAt(e.abs)
			if e.abs == cursorAbs && vw.CursorVisible {
				d.cursorRow, d.cursorX, d.cursorVisible = i-start, vw.CursorX, true
			}
		}
		d.rows = append(d.rows, r)
	}
	return d
}

// entry is a row of the whole folded layout: an absolute line, or a collapsed block (summary).
type entry struct{ abs, summary int }

// entries lays out every kept line with each collapsed block folded into one row.
func (v *View) entries() []entry {
	sc, bs := v.screen, v.blocks
	var all []entry
	first, end := sc.FirstLine(), sc.EndLine()
	bi := 0
	for ln := first; ln < end; {
		for bi < len(bs) && bs[bi].Output[1] <= ln && bs[bi].PromptLine < ln {
			bi++
		}
		if bi < len(bs) && bs[bi].PromptLine == ln && v.collapsed()[bs[bi].Seq] && !bs[bi].Running {
			all = append(all, entry{ln, bi})
			ln = max(bs[bi].Output[1], ln+1)
			bi++
			continue
		}
		all = append(all, entry{ln, -1})
		ln++
	}
	return all
}

// statusColour is a block's bar: the accent while it runs, success or error once it ended,
// tertiary when the shell gave no status.
func statusColour(t theme.Tokens, b term.Block) color.NRGBA {
	switch {
	case b.Running:
		return t.Accent.Primary
	case b.Exit == 0:
		return t.Status.Success
	case b.Exit > 0:
		return t.Status.Error
	}
	return t.Text.Tertiary
}

// Summary is a collapsed block's line: "▸ make -j8 · 2.4 s · exit 2".
func Summary(b term.Block) string {
	s := "▸ " + b.Command + " · " + FormatDuration(b.Duration())
	if b.Exit >= 0 {
		s += fmt.Sprintf(" · exit %d", b.Exit)
	}
	return s
}

// FormatDuration is a command's duration as people read it: 0.4 s, 2.4 s, 1 m 05 s.
func FormatDuration(d time.Duration) string {
	switch {
	case d < 10*time.Second:
		return strconv.FormatFloat(d.Seconds(), 'f', 1, 64) + " s"
	case d < time.Minute:
		return strconv.Itoa(int(d.Seconds())) + " s"
	}
	return fmt.Sprintf("%d m %02d s", int(d.Minutes()), int(d.Seconds())%60)
}

// decorate draws the blocks' bars, hairlines, summaries and the sticky header over the rows.
func (v *View) decorate(objs []fyne.CanvasObject, t theme.Tokens, d display, width float32) []fyne.CanvasObject {
	bs := v.blocks
	if len(bs) == 0 {
		return objs
	}
	o := origin()
	gridW := width - o.X - Inset
	for i, r := range d.rows {
		y := o.Y + float32(i)*v.cellH
		bi := r.summary
		if bi < 0 {
			bi = blockAt(bs, r.abs)
		}
		if bi < 0 {
			continue
		}
		b := bs[bi]
		objs = append(objs, rect(statusColour(t, b), Inset, y, 3, v.cellH))
		if r.abs == b.PromptLine && i > 0 {
			objs = append(objs, rect(t.Border.Hairline, o.X, y, gridW, 1))
		}
		if r.summary >= 0 {
			tx := canvas.NewText(Summary(b), t.Text.Secondary)
			tx.TextSize, tx.TextStyle = v.textSize, fyne.TextStyle{Monospace: true}
			tx.Move(fyne.NewPos(o.X, y+1))
			objs = append(objs, tx)
		}
	}
	// The command stays at the top while its output scrolls under it.
	if len(d.rows) > 1 && d.rows[0].summary < 0 {
		if bi := blockAt(bs, d.rows[0].abs); bi >= 0 && d.rows[0].abs > bs[bi].PromptLine {
			b := bs[bi]
			prompt, _ := v.screen.LineText(b.PromptLine)
			objs = append(objs, rect(t.Surface.Raised, o.X-Gutter, o.Y, gridW+Gutter, v.cellH),
				rect(statusColour(t, b), Inset, o.Y, 3, v.cellH))
			tx := canvas.NewText(prompt, t.Text.Primary)
			tx.TextSize, tx.TextStyle = v.textSize, fyne.TextStyle{Monospace: true, Bold: true}
			tx.Move(fyne.NewPos(o.X, o.Y+1))
			objs = append(objs, tx, rect(t.Border.Strong, o.X-Gutter, o.Y+v.cellH, gridW+Gutter, 1))
			v.sticky = b.Seq
		} else {
			v.sticky = 0
		}
	}
	return objs
}

// Block actions, on the header that shows over a hovered block.

func (v *View) blockBySeq(seq uint64) (term.Block, bool) {
	for _, b := range v.blocks {
		if b.Seq == seq {
			return b, true
		}
	}
	return term.Block{}, false
}

// CopyCommand, CopyOutput, Rerun, ToggleCollapse and SendToProblems act on a block.
func (v *View) CopyCommand(b term.Block) { v.clip(b.Command) }
func (v *View) CopyOutput(b term.Block)  { v.clip(term.OutputText(v.screen, b)) }

// Rerun types the block's command again at the prompt.
func (v *View) Rerun(b term.Block) {
	if b.Command != "" {
		v.send([]byte(b.Command + "\r"))
	}
}

// ToggleCollapse folds a block into one line, or opens it again.
func (v *View) ToggleCollapse(b term.Block) {
	if b.Running {
		return
	}
	c := v.collapsed()
	c[b.Seq] = !c[b.Seq]
	if !c[b.Seq] {
		delete(c, b.Seq)
	}
	v.Refresh()
}

// Collapsed reports whether a block is folded.
func (v *View) Collapsed(b term.Block) bool { return v.collapsed()[b.Seq] }

func (v *View) clip(s string) {
	if a := fyne.CurrentApp(); a != nil && s != "" {
		a.Clipboard().SetContent(s)
	}
}

// Blocks are the commands on the screen shown (BlocksOn).
func (v *View) Blocks() []term.Block { return v.blocksNow() }

// header is the bar over a hovered block: its command, duration and exit status, and actions.
type header struct {
	widget.BaseWidget
	v       *View
	seq     uint64
	text    *kit.Text
	collBtn *kit.IconButton
	box     *fyne.Container
}

func newHeader(v *View) *header {
	h := &header{v: v, text: kit.NewText("", kit.Mono, kit.Secondary)}
	act := func(f func(term.Block)) func() {
		return func() {
			if b, ok := v.blockBySeq(h.seq); ok {
				f(b)
			}
		}
	}
	btn := func(icon icons.Name, label string, f func(term.Block)) *kit.IconButton {
		b := kit.NewIconButton(icon, label, act(f))
		b.Side = 22
		return b
	}
	h.collBtn = btn(icons.ChevronDown, "Collapse", v.ToggleCollapse)
	problems := btn(icons.TriangleAlert, "Send to Problems", func(b term.Block) {
		if v.OnSendToProblems != nil {
			v.OnSendToProblems(b, term.OutputText(v.screen, b))
		}
	})
	bg := canvas.NewRectangle(color.Transparent)
	h.box = container.NewStack(bg, container.New(layout.NewCustomPaddedHBoxLayout(theme.Space1),
		container.NewCenter(h.text),
		btn(icons.Copy, "Copy Command", v.CopyCommand),
		btn(icons.FileText, "Copy Output", v.CopyOutput),
		btn(icons.Refresh, "Rerun", v.Rerun),
		h.collBtn, problems))
	h.ExtendBaseWidget(h)
	return h
}

func (h *header) show(b term.Block) {
	h.seq = b.Seq
	s := b.Command
	if r := []rune(s); len(r) > 40 {
		s = string(r[:39]) + "…"
	}
	if !b.Running {
		s += " · " + FormatDuration(b.Duration())
		if b.Exit >= 0 {
			s += fmt.Sprintf(" · exit %d", b.Exit)
		}
	}
	h.text.SetText(s)
	h.collBtn.Label = "Collapse"
	h.collBtn.Icon = icons.ChevronDown
	if h.v.Collapsed(b) {
		h.collBtn.Label, h.collBtn.Icon = "Expand", icons.ChevronRight
	}
	h.collBtn.Refresh()
	t := theme.Current()
	bg := h.box.Objects[0].(*canvas.Rectangle)
	bg.FillColor, bg.StrokeColor, bg.StrokeWidth, bg.CornerRadius = t.Surface.Overlay, t.Border.Strong, 1, theme.RadiusControl
	bg.Refresh()
}

func (h *header) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(h.box) }

// Links: path:line[:col] in output, resolved by the caller (ResolveLink) against the block's
// folder and kept to the project; underlined on hover, opened with Ctrl+click.

var linkPattern = regexp.MustCompile(`(?:[A-Za-z]:)?(?:[\w.~-]+[/\\])*[\w.-]+\.[A-Za-z0-9]+:(\d+)(?::(\d+))?`)

// link is a resolved path:line on view.
type link struct {
	row, from, to int // its row and columns [from, to)
	path          string
	line, col     int
}

// rowText is a row's text with one rune per cell, so a rune index is a column.
func rowText(l vt.Line) []rune {
	out := make([]rune, len(l.Cells))
	for i, c := range l.Cells {
		switch {
		case c.Width == 0, c.R == 0:
			out[i] = ' '
		default:
			out[i] = c.R
		}
	}
	return out
}

// findLinks lists the links on the rows shown.
func (v *View) findLinks(d display) []link {
	if v.ResolveLink == nil {
		return nil
	}
	if v.linkCache == nil {
		v.linkCache = map[string]linkTarget{}
	}
	var out []link
	for i, r := range d.rows {
		if r.summary >= 0 {
			continue
		}
		text := string(rowText(r.line))
		cwd := v.screen.Cwd()
		if bi := blockAt(v.blocks, r.abs); bi >= 0 && v.blocks[bi].Cwd != "" {
			cwd = v.blocks[bi].Cwd
		}
		for _, m := range linkPattern.FindAllStringSubmatchIndex(text, -1) {
			whole := text[m[0]:m[1]]
			colon := strings.Index(whole[2:], ":") + 2 // past a drive letter's colon
			if colon < 2 {
				colon = strings.Index(whole, ":")
			}
			p := whole[:colon]
			key := cwd + "\x00" + p
			tgt, ok := v.linkCache[key]
			if !ok {
				tgt.path, tgt.ok = v.ResolveLink(cwd, p)
				v.linkCache[key] = tgt
			}
			if !tgt.ok {
				continue
			}
			ln, _ := strconv.Atoi(text[m[2]:m[3]])
			col := 0
			if m[4] >= 0 {
				col, _ = strconv.Atoi(text[m[4]:m[5]])
			}
			from := utf8.RuneCountInString(text[:m[0]])
			out = append(out, link{row: i, from: from, to: from + utf8.RuneCountInString(whole), path: tgt.path, line: ln, col: col})
		}
	}
	return out
}

type linkTarget struct {
	path string
	ok   bool
}

// Links are the resolved links on view; for tests.
func (v *View) Links() []string {
	var out []string
	for _, l := range v.links {
		out = append(out, fmt.Sprintf("%s:%d:%d", l.path, l.line, l.col))
	}
	return out
}

// Search in the output.

// match is a hit of the search: an absolute line and the columns [from, to).
type match struct{ line, from, to int }

// SetSearch finds query in every kept line (case-insensitively unless matchCase) and shows
// the last match, the one nearest the prompt. It returns the number of matches.
func (v *View) SetSearch(query string, matchCase bool) int {
	v.search, v.matches, v.matchAt = query, nil, -1
	if v.screen != nil && query != "" {
		q := query
		if !matchCase {
			q = strings.ToLower(q)
		}
		for ln := v.screen.FirstLine(); ln < v.screen.EndLine(); ln++ {
			text, _ := v.screen.LineText(ln)
			if !matchCase {
				text = strings.ToLower(text)
			}
			for off := 0; ; {
				i := strings.Index(text[off:], q)
				if i < 0 {
					break
				}
				from := utf8.RuneCountInString(text[:off+i])
				v.matches = append(v.matches, match{ln, from, from + utf8.RuneCountInString(q)})
				off += i + len(q)
			}
		}
		if len(v.matches) > 0 {
			v.matchAt = len(v.matches) - 1
			v.scrollToLine(v.matches[v.matchAt].line, false)
		}
	}
	v.Refresh()
	return len(v.matches)
}

// NextMatch moves to the next (1) or previous (-1) match and reports which it is, from 1, and
// how many there are.
func (v *View) NextMatch(step int) (int, int) {
	n := len(v.matches)
	if n == 0 {
		return 0, 0
	}
	v.matchAt = ((v.matchAt+step)%n + n) % n
	v.scrollToLine(v.matches[v.matchAt].line, false)
	v.Refresh()
	return v.matchAt + 1, n
}

// scrollToLine scrolls so absolute line ln shows: in the middle, or at the top.
func (v *View) scrollToLine(ln int, top bool) {
	if v.screen == nil {
		return
	}
	first, end := v.screen.FirstLine(), v.screen.EndLine()
	if v.hasFolded() {
		// Open a collapsed block the line is in, so it can show.
		if bi := blockAt(v.blocks, ln); bi >= 0 && ln > v.blocks[bi].PromptLine {
			delete(v.collapsed(), v.blocks[bi].Seq)
		}
		all := v.entries()
		for i, e := range all {
			if e.abs == ln && e.summary < 0 {
				v.setBackFor(i, len(all), top)
				return
			}
		}
		return
	}
	v.setBackFor(ln-first, end-first, top)
}

func (v *View) setBackFor(idx, total int, top bool) {
	want := v.rows / 2
	if top {
		want = 0
	}
	v.back = max(0, min(total-v.rows, total-idx-v.rows+want))
	v.lastTotal = total // a scroll asked for, not output arriving under the view
}

func (v *View) hasFolded() bool {
	for _, b := range v.blocks {
		if v.collapsed()[b.Seq] && !b.Running {
			return true
		}
	}
	return false
}

// JumpPrompt scrolls to the previous (-1) or next (1) command's prompt from the top of the view.
func (v *View) JumpPrompt(step int) bool {
	bs := v.blocksNow()
	if len(bs) == 0 || v.screen == nil {
		return false
	}
	d := v.compute()
	if len(d.rows) == 0 {
		return false
	}
	top := d.rows[0].abs
	target := -1
	if step < 0 {
		for i := len(bs) - 1; i >= 0; i-- {
			if bs[i].PromptLine < top {
				target = bs[i].PromptLine
				break
			}
		}
	} else {
		for _, b := range bs {
			if b.PromptLine > top {
				target = b.PromptLine
				break
			}
		}
	}
	if target < 0 {
		if step > 0 && v.back > 0 { // past the last prompt: back to the end
			v.back = 0
			v.Refresh()
			return true
		}
		return false
	}
	v.scrollToLine(target, true)
	v.Refresh()
	return true
}

// TopLine is the absolute line at the top of the view; for tests.
func (v *View) TopLine() int {
	d := v.compute()
	if len(d.rows) == 0 {
		return -1
	}
	return d.rows[0].abs
}
