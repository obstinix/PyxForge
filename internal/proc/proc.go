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

// StartDetached starts a process that outlives PyxForge, such as another PyxForge window: it
// is not tied to PyxForge's lifetime the way every other child is.
func StartDetached(path string, args ...string) error { return startDetached(path, args) }

// NoWindow keeps a console program from opening a console window. PyxForge.exe is a windowed
// program without a console, so on Windows every console program it starts (git, compilers,
// GDB, QEMU, Neovim) would otherwise get a console of its own, which Windows shows as a new
// window, in Windows Terminal by default (PXF-DEF-028). The program still has a console, so its
// children share it, and its output still reaches the pipes PyxForge reads. Call it before
// cmd.Start, after anything else that sets cmd.SysProcAttr. It does nothing elsewhere.
func NoWindow(cmd *exec.Cmd) { noWindow(cmd) }

// Alive reports whether a process with this ID is still running.
func Alive(pid int) bool { return alive(pid) }

// Descendants returns the processes pid started, and those they started, running at the time
// of the call (Windows: a Toolhelp snapshot; Linux: /proc). It is empty elsewhere.
func Descendants(pid int) []int { return descendants(pid) }

// Terminate ends a process at once, without letting it clean up. It is for processes left
// behind by one that has already exited.
func Terminate(pid int) error { return terminate(pid) }
