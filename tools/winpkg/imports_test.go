package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestNeovimNeedsTheVCRuntime: the fault the clean-machine check found (0xC0000135). A copy of
// an installed Neovim's programs, without the runtime, is reported as unable to start; with
// the runtime from Visual Studio staged beside them, nothing is missing. It needs Neovim's
// official Windows build and Visual Studio or its Build Tools, as a release build does.
func TestNeovimNeedsTheVCRuntime(t *testing.T) {
	nvim, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("nvim is not installed")
	}
	rt, err := findVCRuntime()
	if err != nil {
		t.Skip(err)
	}
	stage := t.TempDir()
	bin := filepath.Join(stage, "nvim", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(nvim))
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || (ext != ".exe" && ext != ".dll") || strings.HasPrefix(strings.ToLower(e.Name()), "vcruntime") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(filepath.Dir(nvim), e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, e.Name()), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	missing, err := missingImports(stage)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range missing {
		found = found || strings.EqualFold(m, "nvim/bin/nvim.exe needs VCRUNTIME140.dll")
	}
	if !found {
		t.Skipf("this Neovim build doesn't load the Visual C++ runtime: %v", missing)
	}
	if err := checkImports(stage); err == nil || !strings.Contains(err.Error(), "0xC0000135") {
		t.Errorf("checkImports passed a Neovim without its runtime: %v", err)
	}
	copied, err := bundleVCRuntime(stage, rt)
	if err != nil || len(copied) == 0 || copied[0] != "nvim/bin/vcruntime140.dll" {
		t.Fatalf("bundled %v, %v", copied, err)
	}
	if err := checkImports(stage); err != nil {
		t.Errorf("after bundling the runtime: %v", err)
	}
}
