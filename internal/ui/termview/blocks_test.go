package termview

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/term"
	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/vt"
)

// command writes one integrated command onto a screen; exit < 0 leaves it running.
func command(s *vt.Screen, cmd, out string, exit int) {
	fmt.Fprintf(s, "\x1b]133;A\x07$ \x1b]133;B\x07%s\r\n\x1b]133;C\x07%s", cmd, out)
	if exit >= 0 {
		fmt.Fprintf(s, "\x1b]133;D;%d\x07", exit)
	}
}

func rects(v *View) []*canvas.Rectangle {
	var out []*canvas.Rectangle
	for _, o := range test.WidgetRenderer(v).Objects() {
		if r, ok := o.(*canvas.Rectangle); ok {
			out = append(out, r)
		}
	}
	return out
}

func joined(v *View) string {
	var b []string
	for _, tx := range texts(v) {
		b = append(b, tx.Text)
	}
	return strings.Join(b, "|")
}

// TestBlockBars: each block gets a bar in the gutter, success, error or accent while it runs.
func TestBlockBars(t *testing.T) {
	v, scr, _, _ := setup(t)
	v.BlocksOn = true
	command(scr, "true", "", 0)
	command(scr, "false", "", 1)
	command(scr, "sleep", "", -1)
	v.Refresh()
	tk := theme.Current()
	seen := map[string]bool{}
	for _, r := range rects(v) {
		if r.Position().X == Inset && r.Size().Width == 3 {
			switch r.FillColor {
			case tk.Status.Success:
				seen["ok"] = true
			case tk.Status.Error:
				seen["failed"] = true
			case tk.Accent.Primary:
				seen["running"] = true
			}
		}
	}
	if len(seen) != 3 || len(v.Blocks()) != 3 {
		t.Errorf("bars %v, blocks %d", seen, len(v.Blocks()))
	}
	v.BlocksOn = false
	v.Refresh()
	for _, r := range rects(v) {
		if r.Position().X == Inset && r.Size().Width == 3 {
			t.Fatal("a bar is drawn with blocks off")
		}
	}
}

// TestCollapseAndHeader: hovering a block shows its header; Collapse folds it into one summary
// line and Expand opens it; Copy Command and Copy Output copy; Rerun types the command again.
func TestCollapseAndHeader(t *testing.T) {
	v, scr, in, a := setup(t)
	v.BlocksOn = true
	command(scr, "make", "one\r\ntwo\r\n", 2)
	command(scr, "ls", "", -1)
	v.Refresh()
	cell := v.CellSize()
	v.HoverAt(origin().AddXY(cell.Width, cell.Height/2)) // the first row: make's prompt
	b, ok := v.HoveredBlock()
	if !ok || b.Command != "make" || !v.hdr.Visible() {
		t.Fatalf("hovered %+v %v, header shown %v", b, ok, v.hdr.Visible())
	}
	if !strings.Contains(v.hdr.text.Text, "make") || !strings.Contains(v.hdr.text.Text, "exit 2") {
		t.Errorf("header %q", v.hdr.text.Text)
	}
	v.CopyCommand(b)
	if a.Clipboard().Content() != "make" {
		t.Errorf("Copy Command: %q", a.Clipboard().Content())
	}
	v.CopyOutput(b)
	if a.Clipboard().Content() != "one\ntwo" {
		t.Errorf("Copy Output: %q", a.Clipboard().Content())
	}
	v.ToggleCollapse(b)
	if got := joined(v); !strings.Contains(got, "▸ make · ") || !strings.Contains(got, "exit 2") || strings.Contains(got, "one") {
		t.Errorf("collapsed: %s", got)
	}
	if v.TopLine() != b.PromptLine {
		t.Errorf("top line %d", v.TopLine())
	}
	v.ToggleCollapse(b)
	if !strings.Contains(joined(v), "one") {
		t.Error("Expand did not open the block")
	}
	v.Rerun(b)
	if (*in)[len(*in)-1] != "make\r" {
		t.Errorf("Rerun typed %q", (*in)[len(*in)-1])
	}
	var sent string
	v.OnSendToProblems = func(b term.Block, out string) { sent = b.Command + ":" + out }
	h := v.hdr
	h.seq = b.Seq
	for _, o := range test.LaidOutObjects(h) {
		if btn, ok := o.(interface{ Tapped(*fyne.PointEvent) }); ok {
			if lb, ok := o.(interface{ AccessibilityLabel() string }); ok && lb.AccessibilityLabel() == "Send to Problems" {
				btn.Tapped(&fyne.PointEvent{})
			}
		}
	}
	if sent != "make:one\ntwo" {
		t.Errorf("Send to Problems sent %q", sent)
	}
}

// TestStickyHeader: scrolled into a long block's output, its command stays pinned at the top.
func TestStickyHeader(t *testing.T) {
	v, scr, _, _ := setup(t)
	v.BlocksOn = true
	var out strings.Builder
	for i := range 30 {
		fmt.Fprintf(&out, "line %d\r\n", i)
	}
	command(scr, "seq 30", out.String(), 0)
	command(scr, "x", "", -1)
	v.ScrollBy(10)
	if v.sticky == 0 || !strings.Contains(joined(v), "$ seq 30") {
		t.Errorf("no pinned command: sticky %d, %s", v.sticky, joined(v))
	}
}

// TestJumpPrompt: Ctrl+Shift+Up and Down move between the commands' prompts.
func TestJumpPrompt(t *testing.T) {
	v, scr, _, _ := setup(t)
	v.BlocksOn = true
	for i := range 4 {
		command(scr, fmt.Sprintf("c%d", i), "a\r\nb\r\nc\r\n", 0)
	}
	command(scr, "now", "", -1)
	bs := v.Blocks()
	above := func(ln int) int { // the last prompt above line ln
		p := -1
		for _, b := range bs {
			if b.PromptLine < ln {
				p = b.PromptLine
			}
		}
		return p
	}
	start := v.TopLine()
	first := above(start)
	if !v.JumpPrompt(-1) || v.TopLine() != first {
		t.Fatalf("up from line %d: top %d, want %d", start, v.TopLine(), first)
	}
	v.JumpPrompt(-1)
	if want := above(first); v.TopLine() != want {
		t.Errorf("up twice: top %d, want %d", v.TopLine(), want)
	}
	v.JumpPrompt(1)
	if v.TopLine() != first {
		t.Errorf("down: top %d, want %d", v.TopLine(), first)
	}
}

// TestSearchOutput: matches across the history, counted, highlighted, stepped through.
func TestSearchOutput(t *testing.T) {
	v, scr, _, _ := setup(t)
	for i := range 20 {
		fmt.Fprintf(scr, "row %d error here\r\n", i)
	}
	if n := v.SetSearch("ERROR", false); n != 20 {
		t.Fatalf("%d matches", n)
	}
	if v.SetSearch("ERROR", true) != 0 {
		t.Error("Match case found ERROR in lower case")
	}
	v.SetSearch("row 3 ", false)
	if at, n := v.NextMatch(1); at != 1 || n != 1 {
		t.Errorf("match %d of %d", at, n)
	}
	if v.TopLine() > v.matches[0].line || v.TopLine()+6 <= v.matches[0].line {
		t.Errorf("the match on line %d is not on view (top %d)", v.matches[0].line, v.TopLine())
	}
}

// TestLinks: path:line in output is a link when the resolver accepts it; Ctrl+click opens it.
func TestLinks(t *testing.T) {
	v, scr, _, _ := setup(t)
	v.BlocksOn = true
	fmt.Fprintf(scr, "\x1b]7;file://h/proj\x07")
	command(scr, "cc", "src/main.c:12:5: error\r\n/etc/passwd:1: no\r\n", 1)
	var cwds []string
	v.ResolveLink = func(cwd, p string) (string, bool) {
		cwds = append(cwds, cwd)
		return "/proj/" + p, p == "src/main.c"
	}
	var opened string
	v.OnOpenLink = func(p string, line, col int) { opened = fmt.Sprintf("%s:%d:%d", p, line, col) }
	v.Refresh()
	if got := v.Links(); len(got) != 1 || got[0] != "/proj/src/main.c:12:5" {
		t.Fatalf("links %v", got)
	}
	if len(cwds) == 0 || cwds[0] == "" {
		t.Errorf("the resolver was not given the block's folder: %v", cwds)
	}
	l := v.links[0]
	cell := v.CellSize()
	at := origin().AddXY(float32(l.from)*cell.Width+1, float32(l.row)*cell.Height+1)
	v.HoverAt(at)
	if v.Cursor() != desktop.PointerCursor {
		t.Error("no pointer over a link")
	}
	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary, Modifier: fyne.KeyModifierControl})
	if opened != "/proj/src/main.c:12:5" {
		t.Errorf("Ctrl+click opened %q", opened)
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[string]string{"400ms": "0.4 s", "2.4s": "2.4 s", "42s": "42 s", "65s": "1 m 05 s"} {
		dd, _ := parseDur(d)
		if got := FormatDuration(dd); got != want {
			t.Errorf("%s: %q", d, got)
		}
	}
}

func parseDur(s string) (d time.Duration, err error) { return time.ParseDuration(s) }
