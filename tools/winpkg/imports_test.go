package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPartOfWindows(t *testing.T) {
	for dll, want := range map[string]bool{
		"KERNEL32.dll": true, "api-ms-win-crt-stdio-l1-1-0.dll": true, "OPENGL32.dll": true, "dbghelp.dll": true,
		"WINSPOOL.DRV": true, "VCRUNTIME140.dll": false, "MSVCP140.dll": false, "lua51.dll": false, "libcurl-4.dll": false,
	} {
		if got := partOfWindows(dll); got != want {
			t.Errorf("partOfWindows(%s) = %v, want %v", dll, got, want)
		}
	}
}

// TestStagedProgramStarts: a program that needs only what Windows has passes the check (the
// test's own binary, built like PyxForge with MinGW).
func TestStagedProgramStarts(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	if err := os.WriteFile(filepath.Join(stage, "PyxForge.exe"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkImports(stage); err != nil {
		t.Errorf("checkImports refused a program that runs: %v", err)
	}
}
