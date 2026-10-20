//go:build linux

package proc

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
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

// descendants reads every process's parent from /proc/<pid>/stat (the field after the
// command name, which may itself hold spaces and parentheses, so it is found from the last ')').
func descendants(pid int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	kids := map[int][]int{}
	for _, e := range entries {
		child, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue // ended meanwhile
		}
		i := strings.LastIndexByte(string(b), ')')
		f := strings.Fields(string(b[i+1:]))
		if i < 0 || len(f) < 2 {
			continue
		}
		if parent, err := strconv.Atoi(f[1]); err == nil {
			kids[parent] = append(kids[parent], child)
		}
	}
	var out []int
	seen := map[int]bool{pid: true}
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
