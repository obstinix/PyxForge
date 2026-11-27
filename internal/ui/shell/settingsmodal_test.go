package shell

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/ui/kit"
)

// TestSettingsOpensAndCloses: Settings opens as a modal over the workspace (D22) from the
// command and its chord, and closes with Esc (whatever control has focus, but a search field
// with text clears first), the close button and a click on the scrim.
func TestSettingsOpensAndCloses(t *testing.T) {
	s, _ := newTestShell(t)
	c := s.win.Canvas()
	esc := &fyne.KeyEvent{Name: fyne.KeyEscape}
	open := func() *settingsModal {
		t.Helper()
		if !s.Commands().Run("prefs.settings") || s.settingsUI == nil {
			t.Fatal("Open Settings did not open it")
		}
		if c.Overlays().Top() != s.settingsUI.layer || c.Focused() != s.settingsUI.search {
			t.Fatal("Settings is not the overlay in front with focus in its search field")
		}
		return s.settingsUI
	}
	if cmd, _ := s.Commands().Get("prefs.settings"); cmd.Keys != "Ctrl+Shift+," {
		t.Errorf("Open Settings is on %q (K2 keeps it on Ctrl+Shift+,)", cmd.Keys)
	}

	m := open()
	test.Type(m.search, "font")
	s.settingsKey(esc) // the window sees Esc: with text in the field, it is the field's
	m.search.TypedKey(esc)
	if s.settingsUI == nil || m.search.Text != "" {
		t.Fatal("Esc in a search field with text closed Settings or kept the text")
	}
	m.search.TypedKey(esc) // empty now: Esc closes
	if s.settingsUI != nil || c.Overlays().Top() != nil {
		t.Fatal("Esc in the empty search field did not close Settings")
	}

	m = open()
	c.Focus(m.nav)
	s.settingsKey(esc) // focus on another control: the window's key-down hook closes it
	if s.settingsUI != nil {
		t.Fatal("Esc with focus in the page list did not close Settings")
	}

	m = open()
	test.Tap(m.close)
	if s.settingsUI != nil {
		t.Fatal("the close button did not close Settings")
	}

	m = open()
	test.Tap(m.layer.Objects[0].(*scrim)) // the scrim
	if s.settingsUI != nil || len(s.modals) != 0 {
		t.Fatalf("a click on the scrim did not close Settings (modals %d)", len(s.modals))
	}
}

// TestSettingsKeyboard: Tab goes from the search field to the page list; Down and Enter
// choose a page; Tab then reaches the page's controls, and Space operates a switch.
func TestSettingsKeyboard(t *testing.T) {
	s, _ := newTestShell(t)
	c := s.win.Canvas()
	s.Commands().Run("prefs.settings")
	m := s.settingsUI
	c.FocusNext()
	if c.Focused() != m.nav {
		t.Fatalf("Tab from the search field went to %T", c.Focused())
	}
	for range 2 { // General → Editor → Appearance
		m.nav.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	}
	m.nav.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if m.current != "appearance" || m.title.Text != "Appearance" {
		t.Fatalf("Down, Down, Enter chose %q", m.current)
	}
	var motion *kit.Switch
	for range 40 {
		c.FocusNext()
		if sw, ok := c.Focused().(*kit.Switch); ok {
			motion = sw
			break
		}
	}
	if motion == nil || motion.Label != "Reduce motion" {
		t.Fatal("Tab never reached the Reduce motion switch")
	}
	was := ReduceMotion(s.app)
	motion.TypedRune(' ')
	if ReduceMotion(s.app) == was {
		t.Error("Space on Reduce motion did not change it")
	}
}

// TestSettingsSearch: one query finds rows on several pages, grouped under their pages; Show
// opens the row's page.
func TestSettingsSearch(t *testing.T) {
	s, _ := newTestShell(t)
	s.Commands().Run("prefs.settings")
	m := s.settingsUI
	pagesFor := func(q string) []string {
		test.Type(m.search, q)
		defer m.search.SetText("")
		var out []string
		for _, o := range m.results.Objects {
			if g, ok := o.(*kit.SettingGroup); ok {
				out = append(out, g.Heading)
			}
		}
		return out
	}
	if got := pagesFor("text size"); !slices.Equal(got, []string{"Editor", "Terminal"}) {
		t.Errorf("\"text size\" found pages %v", got)
	}
	if got := pagesFor("update"); !slices.Contains(got, "General") {
		t.Errorf("\"update\" found pages %v", got)
	}
	if got := pagesFor("pwsh"); !slices.Equal(got, []string{"Terminal"}) { // by keyword
		t.Errorf("\"pwsh\" found pages %v", got)
	}
	if got := pagesFor("accent"); !slices.Contains(got, "Appearance") {
		t.Errorf("\"accent\" found pages %v", got)
	}
	test.Type(m.search, "zzz no such thing")
	if !strings.Contains(m.title.Text, "0 settings") || len(m.results.Objects) != 1 {
		t.Errorf("no match: title %q, %d results", m.title.Text, len(m.results.Objects))
	}
	m.search.SetText("")
	if m.title.Text != "General" || !m.scroll.Visible() {
		t.Error("clearing the search did not return to the page")
	}

	test.Type(m.search, "shell")
	group := m.results.Objects[0].(*kit.SettingGroup)
	show := group.Rows[0].Control.(*kit.Button)
	test.Tap(show)
	if m.current != "terminal" || m.search.Text != "" {
		t.Errorf("Show went to %q, search %q", m.current, m.search.Text)
	}
}

// TestEverySettingHasARow: every preference the shell keeps for the user has a row in
// Settings. A new preference fails here until it has a row, or is listed below as state the
// shell keeps for itself.
func TestEverySettingHasARow(t *testing.T) {
	state := map[string]string{ // not settings: what the shell remembers by itself
		"prefRecentFiles": "recent files", "prefRecentFolders": "recent folders",
		"prefSetupDone": "the wizard was finished", "prefSetupProfile": "the wizard's last answer",
		"prefSetupOffered": "the toolchain offer was made", "prefUpdateLastCheck": "when updates were last checked",
		"prefBuildPick":     "the build profile chosen in the Build tab",
		"prefPinnedFolders": "the folders pinned on the Welcome page",
	}
	decl := regexp.MustCompile(`(?m)^\s*((?:p|P)ref[A-Za-z]+)\s*=\s*"([^"]+)"`)
	files, _ := filepath.Glob("*.go")
	keys := map[string]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, _ := os.ReadFile(f)
		for _, m := range decl.FindAllStringSubmatch(string(data), -1) {
			keys[m[1]] = m[2]
		}
	}
	if len(keys) < 10 {
		t.Fatalf("found only %d preference constants: %v", len(keys), keys)
	}
	s, _ := newTestShell(t)
	s.OpenSettings()
	for name, key := range keys {
		if _, ok := state[name]; ok {
			continue
		}
		if s.settingsUI.rows[key] == nil {
			t.Errorf("%s (%q) has no row in Settings", name, key)
		}
	}
}

// TestShellSetting: Terminal > Shell lists the shells found and saves the one chosen, which new
// sessions then run (TestTerminalUsesTheChosenShell).
func TestShellSetting(t *testing.T) {
	s, _ := newTestShell(t)
	s.openSettingsAt("terminal")
	row := s.settingsUI.rows[prefTerminalShell]
	d, ok := row.Control.(*kit.Dropdown)
	if !ok {
		t.Fatalf("the Shell row's control is %T", row.Control)
	}
	shells := pty.Shells()
	if len(d.Options) != len(shells) || d.Selected != s.terminalShell().Name {
		t.Fatalf("options %v, selected %q", d.Options, d.Selected)
	}
	last := shells[len(shells)-1]
	d.SetSelected(last.Name)
	if got := s.app.Preferences().String(prefTerminalShell); got != last.Argv[0] {
		t.Errorf("saved %q, want %q", got, last.Argv[0])
	}
}
