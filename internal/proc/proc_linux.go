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
