//go:build !windows

package proc

import "os/exec"

func noWindow(*exec.Cmd) {} // only Windows gives a console program a window of its own
