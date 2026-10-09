//go:build !windows

package build

import (
	"os/exec"
	"syscall"
)

// killTree makes cancelling the build stop the tool and every process it started: the tool
// leads its own process group, and the whole group is killed.
func killTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
