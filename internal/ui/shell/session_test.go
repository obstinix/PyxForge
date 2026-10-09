package shell

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// TestWorkspaceStateRestores closes one shell and opens another on the same folder: the
// layout, open files, explorer folders and build profile come back.
func TestWorkspaceStateRestores(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	store := &workspace.StateStore{Dir: t.TempDir()}
	var files []string
	for _, f := range []string{"boot/boot.asm", "kernel/main.c", "notes.md"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}

	s := NewWithOptions(a, root, Options{State: store})
	for _, f := range files {
		s.OpenFile(f)
	}
	s.OpenFile(files[1]) // the active tab
	s.ToggleInspector()
	s.showDockTab(slices.Index(s.dock.Items, s.buildp.tab))
	s.explorer.Expand([]string{"kernel"})
	s.buildp.pick.Options = []string{defaultTargets, "boot"}
	s.buildp.pick.SetSelected("boot")
	s.win.Resize(fyne.NewSize(1200, 760))
	s.saveState()

	r := NewWithOptions(a, root, Options{State: store})
	var titles []string
	for _, it := range r.editors.Items {
		titles = append(titles, it.Text)
	}
	if want := []string{"boot.asm", "main.c", "notes.md"}; !slices.Equal(titles, want) {
		t.Errorf("tabs %v, want %v", titles, want)
	}
	if got := r.activeFile(); got != files[1] {
		t.Errorf("active file %q, want %q", got, files[1])
	}
	if r.bench.inspectorOn || !r.bench.dockOn || !r.bench.explorerOn {
		t.Errorf("panels: explorer %v, dock %v, inspector %v", r.bench.explorerOn, r.bench.dockOn, r.bench.inspectorOn)
	}
	if r.dock.Selected() != r.buildp.tab {
		t.Errorf("panel tab %q, want Build", r.dock.Selected().Text)
	}
	if got := r.explorer.Expanded(); !slices.Equal(got, []string{"kernel"}) {
		t.Errorf("expanded %v", got)
	}
	if got := a.Preferences().String(prefBuildPick); got != "boot" {
		t.Errorf("build profile %q", got)
	}
	if size := r.win.Canvas().Size(); size.Width != 1200 {
		t.Errorf("window %v", size)
	}

	// New File creates the file in the active file's folder and opens it.
	r.newEntry(false)
	r.lastDialog.Hide()
	p, err := r.explorer.Create("kernel/vga.c", false)
	if err != nil {
		t.Fatal(err)
	}
	r.OpenFile(p)
	if r.activeFile() != p {
		t.Errorf("the new file is not the active tab")
	}
}

func TestReopenedFilesKeepTheirOrderInNeovim(t *testing.T) {
	s, root, q := newEditorShell(t)
	var files []string
	for _, name := range []string{"a.asm", "b.c", "c.ld", "d.md"} {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	s.openFiles(files[:3], files[1])
	s.OpenFile(files[3]) // a file named on the command line opens after them, and is active
	q.pumpUntil(t, "four tabs with d.md selected", func() bool {
		return len(s.editors.Items) == 4 && s.editors.Selected().Text == "d.md"
	})
	var titles []string
	for _, it := range s.editors.Items {
		titles = append(titles, it.Text)
	}
	if want := []string{"a.asm", "b.c", "c.ld", "d.md"}; !slices.Equal(titles, want) {
		t.Errorf("tabs %v, want %v", titles, want)
	}
}
