package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/kit"
)

func menuItem(m *fyne.Menu, label string) *fyne.MenuItem {
	for _, it := range m.Items {
		if it.Label == label {
			return it
		}
	}
	return nil
}

// TestMenus: the menus hold the registered commands with their shortcuts shown, grey out what
// cannot run, and open from the menu bar.
func TestMenus(t *testing.T) {
	s, root := newTestShell(t)
	file := s.menu("File", menus[0].items)
	save := menuItem(file, "Save")
	if save == nil || save.Shortcut == nil || shortcutLabel(save.Shortcut.(*desktop.CustomShortcut)) != "Ctrl+S" {
		t.Fatalf("File > Save: %+v", save)
	}
	if !save.Disabled {
		t.Error("Save is available with no file open")
	}
	for _, m := range menus {
		for _, id := range m.items {
			if id == menuSep {
				continue
			}
			if _, ok := s.cmds.Get(id); !ok {
				t.Errorf("%s menu lists %s, which is not a command", m.title, id)
			}
		}
	}
	if rec := menuItem(file, "Open Recent…"); rec == nil || !rec.Disabled {
		t.Error("Open Recent is available with nothing recent")
	}

	// A file opened is offered again, in the palette; one gone since is dropped with a word.
	p := filepath.Join(root, "boot.asm")
	s.OpenFile(p)
	if rec := menuItem(s.menu("File", menus[0].items), "Open Recent…"); rec == nil || rec.Disabled {
		t.Fatal("Open Recent is greyed out after opening a file")
	}
	s.chooseRecent()
	got := s.Palette().Results()
	if len(got) != 2 || got[0].Title != "boot.asm" || got[1].Title != "Clear Recent" {
		t.Fatalf("Open Recent lists %+v", got)
	}
	s.Palette().Hide()
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	got[0].Run()
	if left := s.app.Preferences().StringList(prefRecentFiles); len(left) != 0 {
		t.Errorf("a missing file stayed on the recent list: %v", left)
	}

	// The menu bar opens its menus, on PyxForge's overlay material.
	test.Tap(s.menuBar.Objects[0].(*widget.Button))
	if s.openMenu == nil || !s.openMenu.Visible() || s.win.Canvas().Overlays().Top() == nil {
		t.Fatal("tapping File opened no menu")
	}
	if _, ok := s.openMenu.Content.(*kit.Glass); !ok {
		t.Errorf("the menu sits on %T, not PyxForge's overlay material", s.openMenu.Content)
	}
}

// TestEditMenuInTheEditor: Edit > Select All and Copy put the file's text on the system
// clipboard through Neovim's clipboard register; Undo takes back an edit.
func TestEditMenuInTheEditor(t *testing.T) {
	s, root, q := newEditorShell(t)
	f := filepath.Join(root, "a.txt")
	if err := os.WriteFile(f, []byte("hello menus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(f)
	q.pumpUntil(t, "the editor focused", func() bool { return s.win.Canvas().Focused() == s.ed.view })
	if !s.available("edit.copy") {
		t.Fatal("Copy is greyed out in the editor")
	}
	s.editAction("selectAll")
	s.editAction("copy")
	q.pumpUntil(t, "the clipboard", func() bool { return strings.Contains(s.app.Clipboard().Content(), "hello menus") })

	s.ed.sess.Input("Oadded<Esc>")
	q.pumpUntil(t, "the edit", func() bool { return s.editors.Selected().Text == "a.txt ●" })
	s.editAction("undo")
	q.pumpUntil(t, "undone", func() bool { return s.editors.Selected().Text == "a.txt" })
}
