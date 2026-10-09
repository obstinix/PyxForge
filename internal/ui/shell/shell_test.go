package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

func newTestShell(t *testing.T) (*Shell, string) {
	t.Helper()
	a := test.NewTempApp(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "boot.asm"), []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewWithOptions(a, root, Options{}), root
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
	s.OpenSettings()
	hooks := len(s.settingsHooks)
	s.closeEditor()
	s.OpenSettings()
	if len(s.settingsHooks) != hooks {
		t.Errorf("reopening Settings left %d appearance hooks, want %d", len(s.settingsHooks), hooks)
	}
	s.closeEditor()
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

// TestAccentFollowsTheActiveRegion enforces A1: exactly one tab bar, the active region's, is
// drawn in the accent, and keyboard commands move it.
func TestAccentFollowsTheActiveRegion(t *testing.T) {
	s, root := newTestShell(t)
	accented := func() []region {
		var out []region
		for r, ov := range s.tabThemes {
			if ov.Theme.(tabTheme).accent {
				out = append(out, r)
			}
		}
		return out
	}
	only := func(want region) {
		t.Helper()
		if got := accented(); len(got) != 1 || got[0] != want || s.active != want {
			t.Errorf("accented %v, active %v; want only %v", got, s.active, want)
		}
	}
	only(regionEditor)

	s.Commands().Run("panel.log")
	only(regionPanel)
	if s.dock.Selected() != s.logTab || s.win.Canvas().Focused() != s.logList || !s.logFrame.Focused() {
		t.Error("Show Log did not focus the log list with a visible ring")
	}

	s.Commands().Run("view.focusExplorer")
	if len(accented()) != 0 || s.active != regionExplorer || !s.explorerFrame.Focused() || s.logFrame.Focused() {
		t.Error("Focus Explorer did not move focus and its ring to the explorer")
	}
	s.explorer.OnOpen(filepath.Join(root, "boot.asm"))
	if s.active != regionExplorer || len(s.editors.Items) != 1 {
		t.Error("opening a file from the focused tree moved the user out of the tree")
	}

	s.Commands().Run("view.focusEditor")
	only(regionEditor)
	if s.explorerFrame.Focused() {
		t.Error("explorer ring still showing after Focus Editor")
	}

	s.Commands().Run("inspector.hex")
	only(regionInspector)
	s.Commands().Run("view.nextTab")
	if got := s.inspectorTabs.Selected().Text; got != "Disasm" {
		t.Errorf("Next Tab in the inspector selected %q", got)
	}
	s.Commands().Run("view.previousTab")
	s.Commands().Run("view.previousTab")
	if got := s.inspectorTabs.Selected().Text; got != "Flags" {
		t.Errorf("Previous Tab twice selected %q", got)
	}
	s.ToggleInspector()
	only(regionEditor)
}

// TestKeyboardReachesEveryControl is the P0 keyboard contract: Tab reaches every interactive
// control in Settings and the status bar, and Space or Enter operates it.
func TestKeyboardReachesEveryControl(t *testing.T) {
	s, _ := newTestShell(t)
	c := s.win.Canvas()
	s.Commands().Run("prefs.settings")
	if len(s.themeCards) != 6 || c.Focused() != s.themeCards[0] || !s.themeCards[0].focused {
		t.Fatal("Open Settings did not put keyboard focus on the first theme card")
	}

	c.Focus(s.themeCards[3])
	test.Type(s.themeCards[3], " ")
	if s.Selection().PaletteID != theme.InkGlass.ID {
		t.Errorf("Space on the Ink & Glass card selected %q", s.Selection().PaletteID)
	}

	seen := map[fyne.Focusable]bool{}
	c.Unfocus()
	for range 100 {
		c.FocusNext()
		seen[c.Focused()] = true
	}
	for _, card := range s.themeCards {
		if !seen[card] {
			t.Errorf("Tab never reaches the %s card", card.name)
		}
	}
	accents := 0
	for f := range seen {
		if a, ok := f.(*accentCard); ok {
			accents++
			if a.a.ID == theme.Amber.ID {
				c.Focus(a)
				a.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			}
		}
	}
	if accents != len(theme.Accents) || s.Selection().AccentID != theme.Amber.ID {
		t.Errorf("Tab reached %d accent cards; Enter on Amber gave %q", accents, s.Selection().AccentID)
	}
	if !seen[s.statusTheme] || !seen[s.railExplorer] {
		t.Error("Tab never reaches the status-bar theme item or the rail")
	}

	s.closeEditor()
	c.Focus(s.statusTheme)
	if !s.statusTheme.focused {
		t.Error("the status-bar item draws no focus ring")
	}
	s.statusTheme.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnter})
	if s.settings == nil || s.editors.Selected() != s.settings {
		t.Error("Enter on the status-bar theme item did not open Settings")
	}
}

// TestToolchainReport checks that the desktop app reports the same detection as the CLI's
// doctor: Neovim's version in the status bar, every tool in the Log, missing tools on demand.
func TestToolchainReport(t *testing.T) {
	s, _ := newTestShell(t)
	nvim := toolchain.Tool{ID: "nvim", Label: "Neovim"}
	qemu := toolchain.Tool{ID: "qemu-x86_64", Label: "QEMU (x86-64)", Hint: map[string]string{runtime.GOOS: "get qemu"}}
	arm := toolchain.Tool{ID: "qemu-arm", Label: "QEMU (ARM)", Optional: true}
	st := []toolchain.Status{{Tool: nvim, Path: "nvim", Version: "0.12.5"}, {Tool: qemu}, {Tool: arm}}

	s.applyTools(st, false)
	if got := s.statusEditor.text.Text; got != "Neovim 0.12.5: not attached" {
		t.Errorf("status bar says %q", got)
	}
	log := strings.Join(s.logLines, "\n")
	for _, want := range []string{"Neovim 0.12.5: nvim", "QEMU (x86-64): not found. Install: get qemu", "QEMU (ARM): not found (optional)"} {
		if !strings.Contains(log, want) {
			t.Errorf("Log is missing %q:\n%s", want, log)
		}
	}
	if s.notes.Count() != 0 {
		t.Error("the startup check posted a notification")
	}
	s.applyTools(st, true) // Tools: Check Toolchain
	if s.notes.Count() != 1 {
		t.Error("an explicit check did not report missing tools")
	}
	s.applyTools([]toolchain.Status{{Tool: nvim}}, false)
	if got := s.statusEditor.text.Text; got != "Neovim: not found" {
		t.Errorf("status bar says %q without Neovim", got)
	}
	if _, ok := s.Commands().Get("tools.check"); !ok {
		t.Error("Check Toolchain is not a command")
	}
}
