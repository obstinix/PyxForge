package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// menuLabels lists a menu's item labels, separators as "-".
func menuLabels(m *fyne.Menu) []string {
	var out []string
	for _, it := range m.Items {
		if it.IsSeparator {
			out = append(out, "-")
			continue
		}
		out = append(out, it.Label)
	}
	return out
}

func tabMenuItem(t *testing.T, m *fyne.Menu, label string) *fyne.MenuItem {
	t.Helper()
	for _, it := range m.Items {
		if it.Label == label {
			return it
		}
	}
	t.Fatalf("no %q in the menu %v", label, menuLabels(m))
	return nil
}

// TestEditorTabManagement: a tab's right-click menu pins it and closes others in bulk; bulk
// closes leave pinned tabs open and ask once about the unsaved files among the tabs they close;
// Close Saved never asks; the palette moves the selected tab; Copy Path copies the tab's file.
func TestEditorTabManagement(t *testing.T) {
	s, root, _ := newEditorShell(t)
	var files []string
	for _, n := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"} {
		p := filepath.Join(root, n)
		if err := os.WriteFile(p, []byte(n+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
		s.OpenFile(p)
	}
	titles := func() string { return fmt.Sprint(tabTitles(s)) }

	// Pin c from its menu: it moves to the front and its menu now offers Unpin.
	c := s.tabFor(files[2])
	m := s.tabMenu(c)
	if got := fmt.Sprint(menuLabels(m)); got != "[Close Close Others Close to the Right Close Saved Close All - Pin - Split Right - Copy Path Copy Relative Path "+revealLabel()+"]" {
		t.Fatalf("tab menu %s", got)
	}
	tabMenuItem(t, m, "Pin").Action()
	if titles() != "[c.txt a.txt b.txt d.txt e.txt]" || !s.editors.Pinned(c) {
		t.Fatalf("after Pin: %s", titles())
	}
	if tabMenuItem(t, s.tabMenu(c), "Unpin") == nil {
		t.Fatal("a pinned tab's menu has no Unpin")
	}
	tabMenuItem(t, s.tabMenu(c), "Copy Relative Path").Action()
	if got := s.app.Clipboard().Content(); got != "c.txt" {
		t.Errorf("Copy Relative Path copied %q", got)
	}

	// Move the selected tab (e) left in the palette; it cannot pass the pinned c.
	s.selectTab(s.tabFor(files[4]), false)
	s.Commands().Run("view.moveTabLeft")
	if titles() != "[c.txt a.txt b.txt e.txt d.txt]" {
		t.Fatalf("after Move Editor Tab Left: %s", titles())
	}
	s.selectTab(s.tabFor(files[0]), false)
	s.Commands().Run("view.moveTabLeft")
	if titles() != "[c.txt a.txt b.txt e.txt d.txt]" {
		t.Fatalf("a tab moved before a pinned one: %s", titles())
	}

	// Close to the Right of b, with e unsaved: one question; Don't Save closes e and d.
	s.selectTab(s.tabFor(files[4]), false)
	test.Type(s.activeDoc().ed, "edit ")
	s.lastDialog = nil
	tabMenuItem(t, s.tabMenu(s.tabFor(files[1])), "Close to the Right").Action()
	if s.lastDialog == nil {
		t.Fatal("closing an unsaved tab did not ask")
	}
	tapInDialog(t, s, "Don't Save")
	if titles() != "[c.txt a.txt b.txt]" {
		t.Fatalf("after Close to the Right: %s", titles())
	}

	// Close Saved closes a and b without asking even with an unsaved pinned c; Close Others
	// keeps the tab it is run from and the pinned c, Close All keeps c.
	test.Type(s.docOf(c).ed, "x")
	s.lastDialog = nil
	s.Commands().Run("view.closeSaved")
	if s.lastDialog != nil || titles() != "[c.txt ●]" {
		t.Fatalf("Close Saved: %s, asked %v", titles(), s.lastDialog != nil)
	}
	s.OpenFile(files[3])
	s.OpenFile(files[4])
	s.Commands().Run("view.closeOthers") // from e: d goes, the pinned c stays
	if titles() != "[c.txt ● e.txt]" {
		t.Fatalf("Close Others from e: %s", titles())
	}
	s.Commands().Run("view.closeAll")
	if titles() != "[c.txt ●]" {
		t.Fatalf("Close All closed a pinned tab or left another: %s", titles())
	}
	// Close (by name) still closes a pinned tab, after asking.
	tabMenuItem(t, s.tabMenu(c), "Close").Action()
	tapInDialog(t, s, "Don't Save")
	if len(s.editors.Items) != 0 {
		t.Fatalf("Close left %s", titles())
	}
}

// TestPinnedTabsRestore: pinned tabs and the tab order come back with the workspace. Plain
// text files, so no language server starts, and UI work posted from goroutines runs on the
// test's own goroutine through the queue.
func TestPinnedTabsRestore(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	store := &workspace.StateStore{Dir: t.TempDir()}
	var files []string
	for _, n := range []string{"one.txt", "two.txt", "three.txt"} {
		p := filepath.Join(root, n)
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{State: store, Editor: true, Dispatch: q.post})
	noToastExpiry(s)
	for _, f := range files {
		s.OpenFile(f)
	}
	s.editors.SetPinned(s.tabFor(files[2]), true)
	s.editors.MoveTab(s.tabFor(files[0]), 2)
	s.saveState()
	s.stopEditor()

	r := NewWithOptions(a, root, Options{State: store, Editor: true, Dispatch: q.post})
	noToastExpiry(r)
	t.Cleanup(r.stopEditor)
	if got := fmt.Sprint(tabTitles(r)); got != "[three.txt two.txt one.txt]" {
		t.Fatalf("restored tabs %s", got)
	}
	if !r.editors.Pinned(r.tabFor(files[2])) || r.editors.Pinned(r.tabFor(files[1])) {
		t.Error("the pin did not come back, or came back on the wrong tab")
	}
}
