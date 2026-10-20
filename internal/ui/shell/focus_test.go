package shell

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
)

// TestOpeningFromTheExplorerFocusesTheEditor replays what a tap on a tree row does in Fyne:
// the open handler runs, then the tree takes keyboard focus, then queued work runs. The editor
// must end up with focus, showing the file under its own tab (PXF-DEF-005); a preview (Space)
// must leave focus in the tree; and the file shown is the row the explorer draws selected.
func TestOpeningFromTheExplorerFocusesTheEditor(t *testing.T) {
	s, root, q := newEditorShell(t)
	a, b := filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte(filepath.Base(p)+" body\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := s.win.Canvas()
	tree := s.explorer.Widget().(fyne.Focusable)
	current := func() string {
		var name string
		_ = s.ed.sess.ExecLua(`return vim.fn.expand("%:t")`, &name)
		return name
	}

	s.explorer.OnOpen(a)
	c.Focus(tree) // Fyne's treeNode.Tapped focuses the tree after OnSelected
	q.pumpUntil(t, "a.txt in the editor with focus", func() bool {
		sel := s.editors.Selected()
		return sel != nil && sel.Text == "a.txt" && c.Focused() == s.ed.view
	})
	if s.active != regionEditor || s.explorer.Active() != "a.txt" || current() != "a.txt" {
		t.Fatalf("after opening a.txt: active %v, explorer row %q, Neovim buffer %q", s.active, s.explorer.Active(), current())
	}

	// Space in the tree previews b.txt: its tab shows, the tree keeps focus.
	c.Focus(tree)
	s.explorer.OnPreview(b)
	q.pumpUntil(t, "b.txt's tab", func() bool { sel := s.editors.Selected(); return sel != nil && sel.Text == "b.txt" })
	if c.Focused() != tree || s.active != regionExplorer || current() != "b.txt" || s.explorer.Active() != "b.txt" {
		t.Errorf("preview: focused %T, active %v, buffer %q, row %q", c.Focused(), s.active, current(), s.explorer.Active())
	}

	// Opening a.txt again, already open, selects its tab and moves focus to the editor.
	s.explorer.OnOpen(a)
	c.Focus(tree)
	q.pumpUntil(t, "a.txt selected again with focus", func() bool {
		return s.editors.Selected().Text == "a.txt" && c.Focused() == s.ed.view && current() == "a.txt"
	})
	if len(s.editors.Items) != 2 {
		t.Errorf("%d tabs after reopening a file", len(s.editors.Items))
	}
}
