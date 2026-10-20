//go:build linux

package proc

import (
	"os/exec"
	"syscall"
)

func containChildren() error { return nil } // Bind covers each child

func bind(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// Sent when the thread that started the child exits. Go keeps its threads for the life of
	// the process unless a goroutine locked to one ends, which PyxForge does not do.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}

func alive(pid int) bool { return signalZero(pid) }

func startDetached(path string, args []string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // no Pdeathsig, its own session
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func descendants(int) []int { return nil }
