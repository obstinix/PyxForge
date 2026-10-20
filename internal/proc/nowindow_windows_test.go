package proc

import (
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// TestNoWindow: NoWindow adds CREATE_NO_WINDOW to the flags already set, and the program's
// output still reaches PyxForge's pipe.
func TestNoWindow(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo", "heard")
	cmd.SysProcAttr = &windows.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	NoWindow(cmd)
	want := uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW)
	if got := cmd.SysProcAttr.CreationFlags; got != want {
		t.Errorf("flags %#x, want %#x", got, want)
	}
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "heard" {
		t.Errorf("output %q, %v", out, err)
	}

	bare := exec.Command("cmd", "/c", "exit")
	NoWindow(bare)
	if bare.SysProcAttr == nil || bare.SysProcAttr.CreationFlags != windows.CREATE_NO_WINDOW {
		t.Errorf("without attributes: %+v", bare.SysProcAttr)
	}
}
