//go:build windows

package build

import (
	"os/exec"
	"strconv"

	"github.com/obstinix/PyxForge/internal/proc"
)

// killTree makes cancelling the build stop the tool and every process it started (make runs
// compilers as children): taskkill /T ends the tree, then the tool itself is killed in case
// taskkill could not run.
func killTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		proc.NoWindow(kill)
		_ = kill.Run()
		return cmd.Process.Kill()
	}
}
