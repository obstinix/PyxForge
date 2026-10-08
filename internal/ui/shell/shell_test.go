package shell

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

func TestGitBranch(t *testing.T) {
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo := t.TempDir()
	write(filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/v3\n")
	if b, ok := gitBranch(repo); !ok || b != "v3" {
		t.Errorf("branch = %q, %v", b, ok)
	}

	detached := t.TempDir()
	write(filepath.Join(detached, ".git", "HEAD"), "9d273f5aa0bb1c2d3e4f5a6b7c8d9e0f1a2b3c4d\n")
	if b, ok := gitBranch(detached); !ok || b != "9d273f5" {
		t.Errorf("detached = %q, %v", b, ok)
	}

	// A linked worktree: .git is a file naming the real git directory.
	worktree := t.TempDir()
	gitdir := filepath.Join(t.TempDir(), "worktrees", "ref")
	write(filepath.Join(gitdir, "HEAD"), "ref: refs/heads/main\n")
	write(filepath.Join(worktree, ".git"), "gitdir: "+gitdir+"\n")
	if b, ok := gitBranch(worktree); !ok || b != "main" {
		t.Errorf("worktree = %q, %v", b, ok)
	}

	if _, ok := gitBranch(t.TempDir()); ok {
		t.Error("plain folder reported a branch")
	}
}

func newTestShell(t *testing.T) (*Shell, string) {
	t.Helper()
	a := test.NewTempApp(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "boot.asm"), []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(a, root), root
}

func TestCommandsAreReachable(t *testing.T) {
	s, _ := newTestShell(t)
	ids := map[string]bool{}
	for _, c := range s.Commands().All() {
		ids[c.ID] = true
	}
	for _, id := range []string{"view.commands", "go.file", "view.explorer", "view.panel", "view.inspector",
		"prefs.settings", "prefs.theme", "prefs.accent", "prefs.reset", "help.about",
		"prefs.theme.system", "prefs.theme.ink-glass", "prefs.accent.amber"} {
		if !ids[id] {
			t.Errorf("command %s not registered", id)
		}
	}
	s.ShowCommands()
	if !s.Palette().Visible() || len(s.Palette().Results()) != len(s.Commands().All()) {
		t.Errorf("palette shows %d of %d commands", len(s.Palette().Results()), len(s.Commands().All()))
	}
	test.Type(s.win.Canvas().Focused(), "toggle insp")
	if r := s.Palette().Results(); len(r) == 0 || r[0].Title != "Toggle Inspector" {
		t.Errorf("search for 'toggle insp' gave %v", r)
	}
	s.Palette().Hide()
}

// TestShellChordsLeaveKeysToNeovim enforces K1: the shell binds only Ctrl+Shift chords, so
// Ctrl+W, Ctrl+B, Ctrl+J, Ctrl+P and AltGr combinations reach Neovim.
func TestShellChordsLeaveKeysToNeovim(t *testing.T) {
	s, _ := newTestShell(t)
	if len(s.shortcuts) < 7 {
		t.Fatalf("only %d keybindings registered", len(s.shortcuts))
	}
	seen := map[string]bool{}
	for _, sc := range s.shortcuts {
		if !IsShellChord(sc) {
			t.Errorf("%s is not a shell chord; Neovim owns it", shortcutLabel(sc))
		}
		if seen[sc.ShortcutName()] {
			t.Errorf("%s is bound twice", shortcutLabel(sc))
		}
		seen[sc.ShortcutName()] = true
	}
	plainCtrl := &desktop.CustomShortcut{KeyName: fyne.KeyW, Modifier: fyne.KeyModifierShortcutDefault}
	if IsShellChord(plainCtrl) || IsShellChord(&fyne.ShortcutCopy{}) {
		t.Error("IsShellChord claims a key that belongs to Neovim")
	}
	// The empty state teaches the bindings as registered.
	c, _ := s.Commands().Get("go.file")
	found := false
	for _, o := range s.emptyKeys.Objects {
		if txt, ok := o.(*kit.Text); ok && txt.Text == c.Keys {
			found = true
		}
	}
	if c.Keys == "" || !found {
		t.Errorf("empty state does not show Go to File's binding %q", c.Keys)
	}
}

func TestTogglesAndRail(t *testing.T) {
	s, _ := newTestShell(t)
	s.ToggleExplorer()
	if s.bench.explorerOn || s.railExplorer.Selected {
		t.Error("explorer still on after toggle")
	}
	if !s.Commands().Run("view.explorer") || !s.bench.explorerOn || !s.railExplorer.Selected {
		t.Error("explorer command did not toggle it back")
	}
	s.ToggleDock()
	if s.bench.dockOn || s.railDock.Selected {
		t.Error("dock still on")
	}
}

func TestInspectorFloatsWhenNarrow(t *testing.T) {
	s, _ := newTestShell(t)
	s.win.Resize(fyne.NewSize(1440, 900))
	s.content.Refresh()
	if s.bench.inspector.floating {
		t.Error("inspector floats at 1440 px")
	}
	s.win.Resize(fyne.NewSize(1024, 640))
	s.content.Refresh()
	if !s.bench.inspector.floating {
		t.Error("inspector docked at 1024 px")
	}
	// Narrowing collapses it; reopening shows it floating over the editor.
	if s.bench.inspectorOn || s.railInspector.Selected {
		t.Error("inspector did not collapse when the window narrowed")
	}
	// Widening restores an automatically collapsed inspector, docked.
	s.win.Resize(fyne.NewSize(1440, 900))
	s.content.Refresh()
	if !s.bench.inspectorOn || s.bench.inspector.floating {
		t.Error("inspector not restored, docked, after widening")
	}
	// Reopened by the user below the breakpoint, it floats.
	s.win.Resize(fyne.NewSize(1024, 640))
	s.content.Refresh()
	s.ToggleInspector()
	if !s.bench.inspectorOn || !s.bench.inspector.Visible() || !s.bench.inspector.floating {
		t.Error("inspector did not reopen as an overlay")
	}
	// Closed by the user, it stays closed across resizes.
	s.ToggleInspector()
	s.win.Resize(fyne.NewSize(1440, 900))
	s.content.Refresh()
	if s.bench.inspectorOn {
		t.Error("a user-closed inspector reopened on resize")
	}
}

func TestEditorTabs(t *testing.T) {
	s, root := newTestShell(t)
	if s.editorPane.Visible() || !s.editorEmpty.Visible() {
		t.Error("empty state not shown with no tabs")
	}
	p := filepath.Join(root, "boot.asm")
	s.OpenFile(p)
	s.OpenFile(p) // a second open selects, it does not duplicate
	if len(s.editors.Items) != 1 || !s.editorPane.Visible() {
		t.Fatalf("%d tabs after opening one file twice", len(s.editors.Items))
	}
	s.OpenSettings()
	if len(s.editors.Items) != 2 || s.editors.Selected() != s.settings {
		t.Error("settings tab not opened and selected")
	}
	s.closeEditor()
	s.closeEditor()
	if len(s.editors.Items) != 0 || s.settings != nil || len(s.open) != 0 || !s.editorEmpty.Visible() {
		t.Error("closing every tab did not restore the empty state")
	}
	s.OpenFile(filepath.Join(root, "missing.asm"))
	if s.notes.Count() != 1 {
		t.Error("opening a missing file did not notify")
	}
}

func TestSelectionPersists(t *testing.T) {
	s, _ := newTestShell(t)
	if !s.Commands().Run("prefs.theme.verdigris-forge") || !s.Commands().Run("prefs.accent.amber") {
		t.Fatal("theme commands missing")
	}
	if got := theme.Current(); got.ID != theme.VerdigrisForge.ID || got.AccentID != theme.Amber.ID {
		t.Errorf("current tokens are %s/%s", got.ID, got.AccentID)
	}
	if got := loadSelection(s.app.Preferences()); got != s.Selection() {
		t.Errorf("persisted %v, shell has %v", got, s.Selection())
	}
	if s.statusTheme.text.Text != "Verdigris Forge · Amber" {
		t.Errorf("status bar says %q", s.statusTheme.text.Text)
	}
}

func TestGlassSetting(t *testing.T) {
	s, _ := newTestShell(t)
	if s.Selection().Glass || theme.Current().GlassOn {
		t.Fatal("glass is on by default")
	}
	if !s.Commands().Run("prefs.glass") || !theme.Current().GlassOn || theme.Current().Glass.Blur == 0 {
		t.Error("the glass command did not turn on translucent, blurred overlays")
	}
	if !loadSelection(s.app.Preferences()).Glass {
		t.Error("the glass setting was not saved")
	}
	s.Commands().Run("prefs.glass")
	if theme.Current().GlassOn {
		t.Error("the glass command did not turn glass off again")
	}
}
