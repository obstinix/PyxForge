//go:build !windows

package proc

import "syscall"

func terminate(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }
