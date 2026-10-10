//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

const createNewConsole = 0x00000010

// TestReleaseOwnConsole starts the test binary with a console of its own, as Explorer would,
// and checks the console is released; in a console shared with this test, it is kept.
func TestReleaseOwnConsole(t *testing.T) {
	if out := os.Getenv("PYX_CONSOLE_OUT"); out != "" {
		result := "kept"
		if releaseOwnConsole() {
			result = "released"
		}
		_ = os.WriteFile(out, []byte(result), 0o644)
		return
	}
	out := filepath.Join(t.TempDir(), "result")
	cmd := exec.Command(os.Args[0], "-test.run=^TestReleaseOwnConsole$")
	cmd.Env = append(os.Environ(), "PYX_CONSOLE_OUT="+out)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole, HideWindow: true}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(out); string(got) != "released" {
		t.Errorf("own console: %q", got)
	}
	if releaseOwnConsole() {
		t.Error("released the console this test shares with its runner")
	}
}
