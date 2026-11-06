//go:build darwin

package proc

import (
	"os/exec"

	"golang.org/x/sys/unix"
)

// containChildren does nothing on macOS: it has no parent-death signal and no job objects, so
// a child outlives a PyxForge killed by force (D10 holds on Windows and Linux; #40). PyxForge
// still ends its children when it closes normally.
func containChildren() error { return nil }

func bind(*exec.Cmd) {}

func alive(pid int) bool { return signalZero(pid) }

// descendants reads every process's parent from the kernel's process table
// (sysctl kern.proc.all), as ps does.
func descendants(pid int) []int {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil
	}
	kids := map[int][]int{}
	for _, p := range procs {
		parent := int(p.Eproc.Ppid)
		kids[parent] = append(kids[parent], int(p.Proc.P_pid))
	}
	var out []int
	seen := map[int]bool{pid: true} // guards against a cycle made by a reused process ID
	for queue := []int{pid}; len(queue) > 0; queue = queue[1:] {
		for _, k := range kids[queue[0]] {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
				queue = append(queue, k)
			}
		}
	}
	return out
}
