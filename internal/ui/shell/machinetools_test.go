package shell

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/ui/editor"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// machineToml makes root a project that boots in QEMU.
func machineToml(t *testing.T, root string) {
	t.Helper()
	toml := "[project]\nname = \"boot\"\n[profiles.boot]\ntool = \"nasm\"\nargs = [\"boot.asm\"]\n" +
		"[qemu]\nexecutable = \"qemu-system-x86_64\"\nboot_image = \"build/boot.bin\"\n"
	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tabNames(s *Shell) []string {
	var names []string
	for _, it := range s.dock.Items {
		names = append(names, it.Text)
	}
	return names
}

// TestEditorFirstByDefault: a project that boots nothing opens as a code editor: no register
// Inspector, and a panel of Terminal, Build, Problems, Git and Log. The machine tools are one
// command away, and a project with a [qemu] table starts with them.
func TestEditorFirstByDefault(t *testing.T) {
	s, _ := newTestShell(t)
	if s.bench.inspectorOn {
		t.Error("the Inspector is open in a project without QEMU")
	}
	if got, want := tabNames(s), []string{"Terminal", "Build", "Problems", "Git", "Log"}; !slices.Equal(got, want) {
		t.Errorf("panel tabs %v, want %v", got, want)
	}
	for _, id := range []string{"panel.qemu", "panel.gdb", "panel.snapshots"} {
		if _, ok := s.Commands().Get(id); !ok {
			t.Errorf("no %s command while the machine tools are out of the panel", id)
		}
	}
	s.dock.Select(s.gitp.tab)
	if !s.Commands().Run("panel.gdb") {
		t.Fatal("panel.gdb did not run")
	}
	if got, want := tabNames(s), []string{"Terminal", "Build", "Problems", "Git", "QEMU", "GDB", "Snapshots", "Log"}; !slices.Equal(got, want) {
		t.Errorf("after Show GDB: panel tabs %v, want %v", got, want)
	}
	if s.dock.Selected().Text != "GDB" {
		t.Errorf("Show GDB selected %q", s.dock.Selected().Text)
	}
	s.showMachineTools() // again: nothing doubles
	if n := len(s.dock.Items); n != 8 {
		t.Errorf("%d tabs after showing the machine tools twice", n)
	}

	root := t.TempDir()
	machineToml(t, root)
	m := NewWithOptions(test.NewTempApp(t), root, Options{})
	noToastExpiry(m)
	if !m.bench.inspectorOn || !slices.Contains(tabNames(m), "QEMU") || m.dock.Selected() != m.term.tab {
		t.Errorf("a QEMU project: inspector %v, tabs %v, selected %q", m.bench.inspectorOn, tabNames(m), m.dock.Selected().Text)
	}
}

// TestGlassSurvivesRestartAndThemeChange: turning glass off is kept when the theme or the
// accent changes and when PyxForge starts again; so is turning it back on.
func TestGlassSurvivesRestartAndThemeChange(t *testing.T) {
	a := test.NewTempApp(t)
	s := NewWithOptions(a, t.TempDir(), Options{})
	noToastExpiry(s)
	s.setGlass(false)
	s.setPalette("verdigris-forge")
	sel := s.Selection()
	sel.AccentID = "amber"
	s.SetSelection(sel)
	if s.Selection().Glass || s.Selection().PaletteID != "verdigris-forge" {
		t.Fatalf("after changing the theme and accent: %+v", s.Selection())
	}
	again := NewWithOptions(a, t.TempDir(), Options{})
	noToastExpiry(again)
	if got := again.Selection(); got.Glass || got.PaletteID != "verdigris-forge" || got.AccentID != "amber" {
		t.Errorf("after a restart: %+v", got)
	}
	again.setGlass(true)
	third := NewWithOptions(a, t.TempDir(), Options{})
	noToastExpiry(third)
	if got := third.Selection(); !got.Glass || got.PaletteID != "verdigris-forge" {
		t.Errorf("glass turned on again, after a restart: %+v", got)
	}
}

// TestTextSizes: the editor and the terminal have sizes of their own, kept within limits,
// applied to an open view and saved for the next start.
func TestTextSizes(t *testing.T) {
	a := test.NewTempApp(t)
	s := NewWithOptions(a, t.TempDir(), Options{})
	noToastExpiry(s)
	if s.textSize(prefEditorTextSize) != theme.TextCode || s.textSize(prefTerminalTextSize) != theme.TextCode {
		t.Fatal("the default sizes are not the scale's code size")
	}
	if got := s.setTextSize(prefEditorTextSize, 16); got != 16 || s.textSize(prefTerminalTextSize) != theme.TextCode {
		t.Errorf("the editor's size is %v and the terminal's %v after setting the editor's to 16", got, s.textSize(prefTerminalTextSize))
	}
	if got := s.setTextSize(prefTerminalTextSize, 99); got != editor.MaxTextSize {
		t.Errorf("a size of 99 became %v, want the limit %v", got, editor.MaxTextSize)
	}
	if got := s.setTextSize(prefTerminalTextSize, 1); got != editor.MinTextSize {
		t.Errorf("a size of 1 became %v, want the limit %v", got, editor.MinTextSize)
	}
	again := NewWithOptions(a, t.TempDir(), Options{})
	noToastExpiry(again)
	if again.textSize(prefEditorTextSize) != 16 || again.textSize(prefTerminalTextSize) != editor.MinTextSize {
		t.Errorf("after a restart: editor %v, terminal %v", again.textSize(prefEditorTextSize), again.textSize(prefTerminalTextSize))
	}
	v := editor.NewView()
	before := v.CellSize()
	v.SetTextSize(20)
	if after := v.CellSize(); v.TextSize() != 20 || after.Height <= before.Height || after.Width <= before.Width {
		t.Errorf("a view at 20 has cells of %v, was %v", after, before)
	}
	a.Preferences().SetFloat(prefEditorTextSize, 400) // a value from a damaged or hand-edited file
	if s.textSize(prefEditorTextSize) != theme.TextCode {
		t.Error("a size out of range is not replaced by the default")
	}
}
