package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// TestFindBar: Ctrl+F in the editor opens the find bar with the cursor in its field; typing
// finds as it goes and Enter goes to the next match, with a count; Ctrl+H adds the replace row,
// and Replace All changes every match; Esc closes the bar and returns to the editor.
func TestFindBar(t *testing.T) {
	s, root, q := newEditorShell(t)
	f := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(f, []byte("mov ax, 1\nmov bx, 2\nint 0x10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(f)
	q.pumpUntil(t, "the editor focused", func() bool { return s.win.Canvas().Focused() == s.ed.view })

	s.ed.view.TypedShortcut(ctrl(fyne.KeyF))
	if !s.find.box.Visible() || s.find.replaceRow.Visible() || s.win.Canvas().Focused() != s.find.find {
		t.Fatalf("Ctrl+F: bar %v, replace row %v, focused %T", s.find.box.Visible(), s.find.replaceRow.Visible(), s.win.Canvas().Focused())
	}
	test.Type(s.find.find, "mov")
	q.pumpUntil(t, "1 of 2", func() bool { return s.find.count.Text == "1 of 2" })
	s.find.find.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	q.pumpUntil(t, "2 of 2", func() bool { return s.find.count.Text == "2 of 2" })
	s.find.find.SetText("nothing like it")
	q.pumpUntil(t, "No results", func() bool { return s.find.count.Text == "No results" })

	// Ctrl+H: replace every match.
	s.ed.view.TypedShortcut(ctrl(fyne.KeyH))
	if !s.find.replaceRow.Visible() {
		t.Fatal("Ctrl+H did not show the replace row")
	}
	s.find.find.SetText("mov")
	s.find.repl.SetText("lea")
	test.Tap(findButton(s.find.box, "Replace All"))
	q.pumpUntil(t, "Replaced 2", func() bool { return s.find.count.Text == "Replaced 2" })
	var lines []string
	_ = s.ed.sess.ExecLua(`return vim.api.nvim_buf_get_lines(0, 0, -1, false)`, &lines)
	if got := strings.Join(lines, "\n"); got != "lea ax, 1\nlea bx, 2\nint 0x10" {
		t.Errorf("after Replace All:\n%s", got)
	}

	// Esc closes the bar and gives the editor its focus back.
	s.find.find.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if s.find.box.Visible() || s.win.Canvas().Focused() != s.ed.view {
		t.Errorf("Esc: bar %v, focused %T", s.find.box.Visible(), s.win.Canvas().Focused())
	}
}
