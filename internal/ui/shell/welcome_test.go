package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/templates"
	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/workspace"
)

func TestEmptyStartupAndPreference(t *testing.T) {
	for _, show := range []bool{true, false} {
		t.Run(fmt.Sprint(show), func(t *testing.T) {
			a := test.NewTempApp(t)
			if !show {
				a.Preferences().SetBool(prefWelcome, false)
			}
			// A real config and file in cwd must never be read as a workspace.
			cwd := t.TempDir()
			t.Chdir(cwd)
			if err := os.WriteFile(filepath.Join(cwd, config.FileName), []byte("[project]\nname = 'unrelated'\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			store := &workspace.StateStore{Dir: t.TempDir()}
			s := NewWithOptions(a, "", Options{State: store, WatchFiles: true})
			noToastExpiry(s)
			t.Cleanup(s.Shutdown)
			if s.root != "" || s.explorer.Root != "" || s.win.Title() != "PyxForge" || s.workspaceStatus != workspaceEmpty {
				t.Fatal("startup selected a workspace")
			}
			if (s.welcome != nil) != show {
				t.Fatalf("welcome = %v, preference = %v", s.welcome != nil, show)
			}
			if files, _ := s.explorer.Files(10); len(files) != 0 {
				t.Fatalf("scanned cwd: %v", files)
			}
			if _, err := config.Load(""); err != config.ErrNotFound {
				t.Fatalf("read cwd config: %v", err)
			}
			if s.term.sess != nil || len(s.term.sessions) != 0 || s.ed != nil || s.gitp.repo != nil || s.snaps.store != nil {
				t.Fatal("empty startup initialized project services")
			}
			s.saveState()
			entries, _ := os.ReadDir(store.Dir)
			if len(entries) != 0 {
				t.Fatal("empty startup persisted an implicit workspace")
			}
			if !strings.Contains(strings.Join(texts(s.explorerFrame.Content), "\n"), "No Folder Open") {
				t.Fatal("missing explorer empty state")
			}
			s.Commands().Run("help.welcome")
			if s.welcome == nil || s.editors.Selected() != s.welcome {
				t.Fatal("Help > Welcome did not open")
			}
			page := strings.Join(texts(s.welcomeRecent), "\n")
			if !strings.Contains(page, "No recent projects") {
				t.Fatalf("empty recent list: %s", page)
			}
			s.editors.CloseIntercept(s.welcome)
			if s.welcome != nil || !s.editorEmpty.Visible() {
				t.Fatal("Welcome is not closable")
			}
		})
	}
}

func TestWorkspaceTransitionsPreserveWindowAndLayout(t *testing.T) {
	a := test.NewTempApp(t)
	q := make(queue, 1024)
	s := NewWithOptions(a, "", Options{Dispatch: q.post, WatchFiles: true})
	noToastExpiry(s)
	t.Cleanup(s.Shutdown)
	s.bench.explorerW, s.bench.dockH, s.bench.inspectorW = 210, 180, 300
	win := s.win
	root, err := createProject(t.TempDir(), "my os ü", templates.Presets[0])
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(file, []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(file)
	if s.root != "" || s.activeFile() != file {
		t.Fatal("opening a file selected its folder")
	}
	s.openFolderAt(root)
	if s.workspaceStatus != workspaceLoading {
		t.Fatal("no loading state")
	}
	q.pumpUntil(t, "workspace open", func() bool { return s.group.currentShell() != s })
	r := s.group.currentShell()
	noToastExpiry(r)
	if r.win != win || r.root != root || r.workspaceStatus != workspaceOpen || r.explorer.Root != root {
		t.Fatal("folder was not adopted by the window")
	}
	if r.bench.explorerW != 210 || r.bench.dockH != 180 || r.bench.inspectorW != 300 {
		t.Fatal("panel sizes changed")
	}
	files, _ := r.explorer.Files(10)
	if !slices.Contains(files, "boot.asm") {
		t.Fatalf("explorer did not load folder: %v", files)
	}
	r.openFolderAt(filepath.Join(root, "missing"))
	q.pumpUntil(t, "failed workspace", func() bool { return r.workspaceStatus == workspaceFailed })
	if r.root != root || r.workspaceError == nil || r.group.currentShell() != r {
		t.Fatal("failed open disturbed workspace")
	}
	r.closeWorkspace()
	q.pumpUntil(t, "workspace closed", func() bool { return r.group.currentShell() != r })
	empty := r.group.currentShell()
	if empty == nil || empty.root != "" || empty.win != win || empty.welcome == nil || empty.win.Title() != "PyxForge" {
		t.Fatal("closing workspace did not return to Welcome")
	}
	if empty.term.sess != nil {
		t.Fatal("close started terminal")
	}
	if got := a.Preferences().StringList(prefRecentFolders); !slices.Equal(got, []string{root}) {
		t.Fatalf("recent projects = %v", got)
	}
}

func TestWelcomeHistoryAndPreferenceSurviveNewShell(t *testing.T) {
	a := test.NewTempApp(t)
	s := NewWithOptions(a, "", Options{})
	noToastExpiry(s)
	for i := range 12 {
		s.addRecent(prefRecentFolders, filepath.Join(t.TempDir(), fmt.Sprint(i)))
	}
	newest := a.Preferences().StringList(prefRecentFolders)[0]
	s.addRecent(prefRecentFolders, newest)
	list := a.Preferences().StringList(prefRecentFolders)
	if len(list) != 10 || list[0] != newest {
		t.Fatalf("history order/cap: %v", list)
	}
	// Exercise the actual checkbox rather than directly writing its preference.
	body := s.welcome.Content.(*container.Scroll).Content.(*fyne.Container).Objects[0].(*fyne.Container)
	check := body.Objects[5].(*widget.Check)
	test.Tap(check)
	if a.Preferences().BoolWithFallback(prefWelcome, true) {
		t.Fatal("checkbox did not save")
	}
	s.Shutdown()
	r := NewWithOptions(a, "", Options{})
	noToastExpiry(r)
	t.Cleanup(r.Shutdown)
	if r.welcome != nil || !slices.Equal(a.Preferences().StringList(prefRecentFolders), list) {
		t.Fatal("preferences were not reused")
	}
	r.showWelcome()
	r.openRecent(prefRecentFolders, newest)
	if slices.Contains(a.Preferences().StringList(prefRecentFolders), newest) {
		t.Fatal("invalid recent entry was not removed")
	}
}

func TestCreateProjectDoesNotOverwriteOrEscape(t *testing.T) {
	parent := t.TempDir()
	path, err := createProject(parent, "Kernel", templates.Presets[0])
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
		t.Fatal("project not created")
	}
	for _, name := range []string{"Kernel", "", ".", "..", "../escape", `a\b`, "C:bad", "CON", "nul.txt", "COM1", "lpt9.log"} {
		if _, err := createProject(parent, name, templates.Presets[0]); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}

func TestWorkspaceChangeAsksBeforeStoppingWork(t *testing.T) {
	a := test.NewTempApp(t)
	q := make(queue, 1024)
	s := NewWithOptions(a, "", Options{Dispatch: q.post})
	noToastExpiry(s)
	t.Cleanup(s.Shutdown)
	root, err := createProject(t.TempDir(), "next", templates.Presets[0])
	if err != nil {
		t.Fatal(err)
	}
	stopped := false
	s.buildp.running = true
	s.buildp.cancel = func() { stopped = true }
	s.buildp.done = make(chan struct{})
	close(s.buildp.done)

	s.openFolderAt(root)
	q.pumpUntil(t, "stop-work confirmation", func() bool { return len(s.modals) > 0 })
	if stopped || s.group.currentShell() != s {
		t.Fatal("workspace switch stopped work before confirmation")
	}
	s.modals[len(s.modals)-1].hide()
	if stopped || s.group.currentShell() != s {
		t.Fatal("cancelling the prompt stopped work or switched workspace")
	}
}

func TestStandaloneFileOffersProjectAndBuildExplainsMissingWorkspace(t *testing.T) {
	a := test.NewTempApp(t)
	q := make(queue, 1024)
	s := NewWithOptions(a, "", Options{Dispatch: q.post})
	noToastExpiry(s)
	t.Cleanup(s.Shutdown)
	root, err := createProject(t.TempDir(), "boot project", templates.Presets[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, config.FileName), []byte("[project]\nname = 'boot'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(file, []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(file)
	q.pumpUntil(t, "project offer", func() bool { return len(s.modals) > 0 })
	if s.root != "" {
		t.Fatal("opening a file silently selected its project")
	}
	s.buildp.start()
	if !strings.Contains(strings.Join(s.logLines, "\n"), "Open a project folder first") {
		t.Fatal("Build did not explain that a workspace is required")
	}
}

func TestFolderPickerIsSingleAndOwnedModal(t *testing.T) {
	s, _ := newTestShell(t)
	t.Cleanup(s.Shutdown)
	s.pickFolder("Open Folder", func(string) {})
	s.pickFolder("Open Another Folder", func(string) {})
	if !s.folderPickerOpen || len(s.modals) != 1 {
		t.Fatal("folder picker was not guarded as a single modal")
	}
	s.modals[0].hide()
	if s.folderPickerOpen {
		t.Fatal("closing the folder picker left it marked open")
	}
}

func TestWelcomeColumnsMinimumDoesNotDependOnLayoutHistory(t *testing.T) {
	columns := &welcomeColumns{}
	objects := []fyne.CanvasObject{widget.NewLabel("Start"), widget.NewLabel("Guide")}
	minimum := columns.MinSize(objects)
	columns.Layout(objects, fyne.NewSize(300, 240))
	want := objects[0].MinSize().Height + objects[1].MinSize().Height + 3*theme.Space4
	if minimum.Height < want {
		t.Fatalf("minimum height %.1f is less than stacked height %.1f", minimum.Height, want)
	}
}

func TestStandaloneNeovimAndWorkspaceCloseUnsaved(t *testing.T) {
	initial, root, q := newEditorShell(t)
	initial.replaceWorkspace("")
	q.pumpUntil(t, "standalone shell", func() bool { return initial.group.currentShell() != initial })
	s := initial.group.currentShell()
	noToastExpiry(s)
	t.Cleanup(initial.Shutdown)
	file := filepath.Join(root, "standalone.txt")
	if err := os.WriteFile(file, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.OpenFile(file)
	q.pumpUntil(t, "standalone buffer", func() bool { return s.activeFile() == file })
	if s.root != "" || s.explorer.Root != "" || s.ed == nil || s.ed.sess == nil {
		t.Fatal("standalone file created a workspace or failed to start Neovim")
	}
	s.ed.sess.Input("iunsaved <Esc>")
	q.pumpUntil(t, "modified standalone buffer", func() bool { return s.activeBuffer() != nil && s.activeBuffer().modified })
	s.openFolderAt(root)
	q.pumpUntil(t, "unsaved confirmation", func() bool { return s.lastDialog != nil })
	if s.group.currentShell() != s {
		t.Fatal("workspace changed before unsaved confirmation")
	}
	s.lastDialog.Hide() // Cancel
	if s.group.currentShell() != s {
		t.Fatal("cancel replaced workspace")
	}
	s.showWelcome()
	if s.win.Canvas().Focused() == s.ed.view {
		t.Fatal("Welcome left hidden editor focused")
	}
	s.openFolderAt(root)
	q.pumpUntil(t, "second confirmation", func() bool { return len(s.modals) > 0 })
	s.modals[len(s.modals)-1].confirm() // Save, using the actual confirmation's default action
	q.pumpUntil(t, "workspace after save", func() bool { return s.group.currentShell() != s })
	opened := s.group.currentShell()
	noToastExpiry(opened)
	if b, err := os.ReadFile(file); err != nil || !strings.Contains(string(b), "unsaved") {
		t.Fatalf("unsaved buffer lost: %q %v", b, err)
	}
	opened.closeWorkspace()
	q.pumpUntil(t, "workspace closed", func() bool { return opened.group.currentShell() != opened })
	empty := opened.group.currentShell()
	noToastExpiry(empty)
	if empty == nil || empty.welcome == nil || empty.term.sess != nil {
		t.Fatal("close did not restore idle Welcome")
	}
	if empty.nvimOptions().Dir != empty.processFolder() || empty.nvimOptions().Dir == root {
		t.Fatal("non-project terminal inherits the previous workspace")
	}
	empty.term.newSession()
	if empty.term.sess == nil || len(empty.term.sessions) != 1 || empty.root != "" {
		t.Fatal("explicit non-project terminal failed")
	}
}

// findSelect finds a drop-down under o.
func findSelect(o fyne.CanvasObject) *widget.Select {
	if o == nil || !o.Visible() {
		return nil
	}
	switch w := o.(type) {
	case *widget.Select:
		return w
	case *fyne.Container:
		for _, c := range w.Objects {
			if s := findSelect(c); s != nil {
				return s
			}
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(w).Objects() {
			if s := findSelect(c); s != nil {
				return s
			}
		}
	}
	return nil
}

// TestNewProjectTemplates: New Project offers the presets, an empty folder first; a project made
// from the boot-sector preset has the example's files and its own name (#72).
func TestNewProjectTemplates(t *testing.T) {
	a := test.NewTempApp(t)
	s := NewWithOptions(a, "", Options{Dispatch: func(f func()) { f() }})
	s.newProject()
	sel := findSelect(s.win.Canvas().Overlays().Top())
	if sel == nil || len(sel.Options) != len(templates.Presets) || sel.Selected != "Empty folder" {
		t.Fatalf("the Template choice: %+v", sel)
	}
	s.lastDialog.Hide()

	boot, _ := templates.ByID("boot-sector")
	root, err := createProject(t.TempDir(), "my os", boot)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"boot.asm", "pyxforge.toml", ".gitignore", "README.md"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	if c, err := config.Load(root); err != nil || c.Project.Name != "my os" || c.Qemu == nil {
		t.Errorf("pyxforge.toml: %+v, %v", c, err)
	}
}
