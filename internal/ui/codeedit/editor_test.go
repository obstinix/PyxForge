package codeedit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/textbuf"
)

func open(t *testing.T, name, text string) (*Editor, string, fyne.Window) {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := textbuf.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	e := New(b, p)
	w := test.NewTempWindow(t, e)
	w.Resize(fyne.NewSize(800, 400))
	w.Canvas().Focus(e)
	return e, p, w
}

func texts(e *Editor) []string {
	var out []string
	for _, o := range test.WidgetRenderer(e).Objects() {
		if tx, ok := o.(*canvas.Text); ok {
			out = append(out, tx.Text)
		}
	}
	return out
}

// TestEditAndSave edits a C file through the keyboard as a user would and saves it.
func TestEditAndSave(t *testing.T) {
	test.NewTempApp(t)
	e, p, _ := open(t, "main.c", "int main(void) {\n}\n")
	if e.Language() != "C" || e.LineComment != "//" {
		t.Errorf("language %q, comment %q", e.Language(), e.LineComment)
	}
	e.SetCursor(Pos{Line: 0, Col: 16}, false)
	test.Type(e, "")
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	test.Type(e, "return 0;")
	if got := e.Buf.Text(); got != "int main(void) {\n    return 0;\n}\n" {
		t.Fatalf("after typing: %q", got)
	}
	e.TypedShortcut(&fyne.ShortcutUndo{})
	if strings.Contains(e.Buf.Text(), "return") {
		t.Errorf("undo left %q", e.Buf.Text())
	}
	e.TypedShortcut(&fyne.ShortcutRedo{})
	if !e.Buf.Modified() {
		t.Fatal("not modified")
	}
	if err := e.Buf.Save(p); err != nil {
		t.Fatal(err)
	}
	disk, _ := os.ReadFile(p)
	if string(disk) != e.Buf.Text() {
		t.Errorf("on disk %q", disk)
	}
	// Selection with Shift and the clipboard.
	e.SetCursor(Pos{Line: 1, Col: 4}, false)
	e.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
	e.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	e.TypedShortcut(&fyne.ShortcutCut{})
	if got := fyne.CurrentApp().Clipboard().Content(); got != "return 0;" {
		t.Errorf("cut %q", got)
	}
	e.TypedShortcut(&fyne.ShortcutPaste{})
	if !strings.Contains(e.Buf.Text(), "    return 0;") {
		t.Errorf("paste: %q", e.Buf.Text())
	}
}

// TestDrawsColouredText: the frame holds the line numbers and the text in runs.
func TestDrawsColouredText(t *testing.T) {
	test.NewTempApp(t)
	e, _, _ := open(t, "a.py", "def f(x):\n\treturn x  # one\n")
	e.Refresh()
	all := strings.Join(texts(e), "|")
	for _, want := range []string{"1", "2", "def", "return", "# one"} {
		if !strings.Contains(all, want) {
			t.Errorf("no %q in the frame: %s", want, all)
		}
	}
	e.SetDiagnostics([]Diagnostic{{Range: Range{Start: Pos{Line: 1, Col: 1}, End: Pos{Line: 1, Col: 7}}, Severity: SevError, Message: "bad"}})
	e.ShowInfo("hover text")
	if !strings.Contains(strings.Join(texts(e), "|"), "hover text") {
		t.Error("the information box is not drawn")
	}
}

// TestCompletionList: the list opens from Complete's answer, the arrows move through it and
// Return puts the entry in place of the word typed so far.
func TestCompletionList(t *testing.T) {
	test.NewTempApp(t)
	e, _, _ := open(t, "a.c", "int total;\n")
	asked := 0
	e.Complete = func(at Pos) {
		asked++
		e.ShowCompletions([]Completion{{Label: "toast"}, {Label: "total", Detail: "int"}})
	}
	e.SetCursor(Pos{Line: 1, Col: 0}, false)
	test.Type(e, "to")
	if asked == 0 || len(e.completes) != 2 {
		t.Fatalf("asked %d, list %v", asked, e.completes)
	}
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if string(e.Buf.Line(1)) != "total" || len(e.completes) != 0 {
		t.Errorf("accepted: %q, list %v", string(e.Buf.Line(1)), e.completes)
	}
}

// TestMouseAndFolding: a click places the cursor, a double click selects a word, a click on
// the fold marker of a block folds it.
func TestMouseAndFolding(t *testing.T) {
	test.NewTempApp(t)
	e, _, _ := open(t, "a.c", "void f() {\n\tone();\n\ttwo();\n}\nint x;\n")
	gw, cell := e.gutterWidth(), e.CellSize()
	at := fyne.NewPos(gw+cell.Width*5.2, cell.Height*1.5) // line 2, "\tone": display col 5 is 'n'
	e.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	if e.Cur.Line != 1 {
		t.Fatalf("click went to %v", e.Cur)
	}
	e.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	if e.SelectedText() != "one" {
		t.Errorf("double click selected %q", e.SelectedText())
	}
	time.Sleep(450 * time.Millisecond)
	marker := fyne.NewPos(gw-cell.Width, cell.Height*0.5)
	e.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: marker}, Button: desktop.MouseButtonPrimary})
	if _, folded := e.Folded(0); !folded {
		t.Fatal("the click on the fold marker did not fold")
	}
	if got := e.VisibleLines(0, 10); len(got) < 2 || got[1] != 3 {
		t.Errorf("visible lines %v", got)
	}
}

// TestLargeFileFrameTime measures how long a frame of a 10,000-line file takes to build, at
// the top, the middle and after an edit, so the feasibility record has numbers.
func TestLargeFileFrameTime(t *testing.T) {
	test.NewTempApp(t)
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&b, "static int f%d(int x) {\n\t/* step %d */\n\treturn x * %d + 1;\n}\n\n", i, i, i)
	}
	start := time.Now()
	e, _, w := open(t, "big.c", b.String())
	w.Resize(fyne.NewSize(1400, 900))
	e.Refresh()
	first := time.Since(start)
	r := test.WidgetRenderer(e)
	frame := func() time.Duration {
		s := time.Now()
		r.Refresh()
		return time.Since(s)
	}
	var worst, total time.Duration
	const n = 100
	for i := 0; i < n; i++ {
		e.scrollBy(40)
		d := frame()
		total += d
		worst = max(worst, d)
	}
	e.Reveal(Pos{Line: 5000, Col: 0})
	test.Type(e, "x")
	edit := frame()
	t.Logf("open and first frame %v; scrolling frames avg %v, worst %v; frame after an edit %v", first, total/n, worst, edit)
	if total/n > 50*time.Millisecond {
		t.Errorf("an average frame took %v", total/n)
	}
}
