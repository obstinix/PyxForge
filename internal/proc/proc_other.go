//go:build !windows && !linux

package proc

import "os/exec"

func containChildren() error { return nil }

func bind(*exec.Cmd) {}

func alive(pid int) bool { return signalZero(pid) }
