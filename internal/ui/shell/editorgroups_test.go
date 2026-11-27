package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// groupTitles lists each group's tab titles, left to right.
func groupTitles(s *Shell) string {
	var out [][]string
	for _, g := range s.groups {
		var titles []string
		for _, it := range g.Items {
			titles = append(titles, it.Text)
		}
		out = append(out, titles)
	}
	return fmt.Sprint(out)
}

// TestEditorGroups: Split Right moves the selected file into a group of its own on the right
// (a file is in one group at a time), the active group follows the selection, commands act on
// it, opening a file that is open in another group goes there, a group closes with its last
// tab, and Join All Editor Groups brings everything back into one.
func TestEditorGroups(t *testing.T) {
	s, root, _ := newEditorShell(t)
	var files []string
	for _, n := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
		p := filepath.Join(root, n)
		if err := os.WriteFile(p, []byte(n+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
		s.OpenFile(p)
	}

	// d is selected: Split Right puts it in a second group, which becomes active.
	s.Commands().Run("view.splitRight")
	if got := groupTitles(s); got != "[[a.txt b.txt c.txt] [d.txt]]" {
		t.Fatalf("after Split Right: %s", got)
	}
	if s.editors != s.groups[1] || s.activeDoc() == nil || s.activeDoc().path != files[3] {
		t.Fatal("the new group is not the active one with d.txt")
	}
	if !s.groups[1].Accent() || s.groups[0].Accent() {
		t.Error("the accent is not on the active group alone")
	}

	// A single tab cannot split further: its file would only change sides.
	s.Commands().Run("view.splitRight")
	if got := groupTitles(s); got != "[[a.txt b.txt c.txt] [d.txt]]" {
		t.Fatalf("splitting a group's only tab moved it: %s", got)
	}

	// Opening a file already open in the left group selects it there and makes that group active.
	s.OpenFile(files[1])
	if s.editors != s.groups[0] || s.editors.Selected().Text != "b.txt" || len(s.allTabs()) != 4 {
		t.Fatalf("opening b.txt again: %s, active group %d", groupTitles(s), s.groupIndex(s.editors.Selected()))
	}
	// From the tab menu: move c to the right group, then back with Move to Left Group.
	c := s.tabFor(files[2])
	tabMenuItem(t, s.tabMenu(c), "Split Right").Action()
	if got := groupTitles(s); got != "[[a.txt b.txt] [d.txt c.txt]]" || s.editors != s.groups[1] {
		t.Fatalf("after moving c right: %s", got)
	}
	s.Commands().Run("view.moveToLeftGroup")
	if got := groupTitles(s); got != "[[a.txt b.txt c.txt] [d.txt]]" {
		t.Fatalf("after Move Editor to Left Group: %s", got)
	}
	// Focus Next Editor Group moves between groups.
	s.Commands().Run("view.nextGroup")
	if s.editors != s.groups[1] {
		t.Error("Focus Next Editor Group did not move to the right group")
	}
	// Closing the right group's last tab closes the group.
	s.closeEditor()
	if got := groupTitles(s); got != "[[a.txt b.txt c.txt]]" || s.editors != s.groups[0] {
		t.Fatalf("after closing d.txt: %s", got)
	}

	// Join All Editor Groups.
	s.OpenFile(files[3])
	s.Commands().Run("view.splitRight")
	s.selectTab(s.tabFor(files[0]), false)
	s.Commands().Run("view.splitRight")
	if got := groupTitles(s); got != "[[b.txt c.txt] [d.txt a.txt]]" {
		t.Fatalf("before joining: %s", got)
	}
	s.Commands().Run("view.joinGroups")
	if got := groupTitles(s); got != "[[b.txt c.txt d.txt a.txt]]" || s.editors.Selected().Text != "a.txt" {
		t.Fatalf("after Join All Editor Groups: %s, selected %s", got, s.editors.Selected().Text)
	}
	// Close All closes every group's tabs.
	s.Commands().Run("view.splitRight")
	s.Commands().Run("view.closeAll")
	if len(s.allTabs()) != 0 || len(s.groups) != 1 {
		t.Errorf("after Close All: %s", groupTitles(s))
	}
}

// TestEditorGroupsRestore: the groups, pins and the active file come back with the workspace.
func TestEditorGroupsRestore(t *testing.T) {
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
	s.Commands().Run("view.splitRight") // three.txt to the right
	s.groups[1].SetPinned(s.tabFor(files[2]), true)
	s.selectTab(s.tabFor(files[1]), false) // two.txt, left, is the active file
	s.saveState()
	s.stopEditor()

	r := NewWithOptions(a, root, Options{State: store, Editor: true, Dispatch: q.post})
	noToastExpiry(r)
	t.Cleanup(r.stopEditor)
	if got := groupTitles(r); got != "[[one.txt two.txt] [three.txt]]" {
		t.Fatalf("restored groups %s", got)
	}
	if !r.groups[1].Pinned(r.tabFor(files[2])) || r.activeFile() != files[1] {
		t.Errorf("pin %v, active %q", r.groups[1].Pinned(r.tabFor(files[2])), r.activeFile())
	}
}
