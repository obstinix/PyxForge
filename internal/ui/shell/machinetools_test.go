package shell

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"
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
