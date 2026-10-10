//go:build !windows

package proc

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// signalZero probes a process with signal 0. A zombie still answers, so on Linux its state in
// /proc decides.
func signalZero(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		if i := strings.LastIndexByte(string(b), ')'); i >= 0 && i+2 < len(b) && b[i+2] == 'Z' {
			return false
		}
	}
	return true
}
