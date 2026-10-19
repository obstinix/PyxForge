//go:build !windows && !linux

package proc

import (
	"os/exec"
	"syscall"
)

func startDetached(path string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
