package toolchain

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestToolThatCannotStartIsBroken: a program that is there but cannot load a DLL it needs is
// not "found": Doctor says it cannot start. A release's bundled Neovim was reported as found
// on a machine without the Visual C++ runtime, where Windows refused to start it (0xC0000135).
// The test builds such a program for real: one linked against a DLL that is then removed.
func TestToolThatCannotStartIsBroken(t *testing.T) {
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc is not installed")
	}
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("dep.c", "__declspec(dllexport) int dep(void) { return 7; }\n")
	write("tool.c", "#include <stdio.h>\n__declspec(dllimport) int dep(void);\nint main(void) { printf(\"tool %d.0\\n\", dep()); return 0; }\n")
	for _, args := range [][]string{{"-shared", "-o", "dep.dll", "dep.c"}, {"-o", "tool.exe", "tool.c", "dep.dll"}} {
		cmd := exec.Command(gcc, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("gcc %v: %v\n%s", args, err, out)
		}
	}
	tool := Tool{ID: "tool", Label: "Tool", Candidates: []string{"pyx-broken-tool"}, VersionArgs: []string{"--version"}}
	d := New()
	d.LookPath = func(string) (string, error) { return filepath.Join(dir, "tool.exe"), nil }
	if st := d.Detect(context.Background(), []Tool{tool})[0]; !st.Usable() || st.Version != "7.0" {
		t.Fatalf("with its DLL: %+v", st)
	}
	if err := os.Remove(filepath.Join(dir, "dep.dll")); err != nil {
		t.Fatal(err)
	}
	st := New()
	st.LookPath = d.LookPath
	got := st.Detect(context.Background(), []Tool{tool})[0]
	if !got.Found() || got.Usable() || !got.Broken || got.Err == nil || !strings.Contains(got.Err.Error(), "0xC0000135") {
		t.Errorf("without its DLL: %+v", got)
	}
}
