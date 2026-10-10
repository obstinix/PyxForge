// Package proc makes sure the processes PyxForge starts (QEMU, GDB, Neovim, shells, build
// tools) end with it, even when PyxForge itself is killed and cannot run its own cleanup.
//
// On Windows, ContainChildren puts PyxForge in a Job Object that kills every process in it
// when the last handle closes, which the system does when PyxForge exits for any reason;
// children inherit the job. On Linux, Bind asks the kernel to SIGKILL a child when PyxForge
// dies (PR_SET_PDEATHSIG). Elsewhere both are no-ops and the normal shutdown path applies.
package proc

import "os/exec"

// ContainChildren arranges for every process PyxForge starts from now on to end with it. Call
// it once, early in main. An error means the platform refused (Windows 7 inside another job,
// for example); PyxForge still runs and relies on its normal shutdown.
func ContainChildren() error { return containChildren() }

// Bind ties one child to PyxForge's lifetime where the platform supports it per process. Call
// it before cmd.Start.
func Bind(cmd *exec.Cmd) { bind(cmd) }

// Alive reports whether a process with this ID is still running.
func Alive(pid int) bool { return alive(pid) }
