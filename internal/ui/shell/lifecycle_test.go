package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// waitPid reads the process ID a child wrote to file.
func waitPid(t *testing.T, file string) int {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(file); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no process ID in %s", file)
	return 0
}

// waitGone fails the test unless the process ends within the time a clean shutdown takes.
func waitGone(t *testing.T, what string, pid int) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for proc.Alive(pid) {
		if time.Now().After(deadline) {
			t.Errorf("%s (pid %d) still runs after Shutdown", what, pid)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestShutdownStopsARunningBuild closes the shell while a build tool runs: the tool must not
// outlive the window. (On Linux the kernel ends only PyxForge's direct children when PyxForge
// exits, so a build left running would leave the tool's own children behind.)
func TestShutdownStopsARunningBuild(t *testing.T) {
	root := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(root, "tool.pid")
	toml := fmt.Sprintf("[project]\nname = \"demo\"\n[profiles.slow]\ntool = %q\nargs = [%q, \"sleep\"]\nenv = { PYX_HELPER = \"1\" }\n",
		self, "pid:"+pidFile)
	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	q := make(queue, 1024)
	s := NewWithOptions(test.NewTempApp(t), root, Options{Dispatch: q.post})
	if !s.buildp.reload() {
		t.Fatal("pyxforge.toml was not loaded")
	}
	s.buildp.start()
	if !s.buildp.running {
		t.Fatal("the build did not start")
	}
	pid := waitPid(t, pidFile)
	s.Shutdown()
	waitGone(t, "the build tool", pid)
}

// TestShutdownWhileQEMUStarts closes the shell after QEMU has started but before the window
// heard back from the launch, the moment a window closed during startup hits: that QEMU must
// be stopped too.
func TestShutdownWhileQEMUStarts(t *testing.T) {
	if _, err := toolchain.LookPath("qemu-system-x86_64"); err != nil {
		t.Skip("qemu-system-x86_64 is not installed")
	}
	root := t.TempDir()
	pidFile := filepath.Join(root, "qemu.pid")
	toml := fmt.Sprintf("[project]\nname = \"boot\"\n[qemu]\nmemory = \"16M\"\nboot_image = \"boot.bin\"\n"+
		"extra_args = [\"-display\", \"none\", \"-pidfile\", %q]\n[qemu.debug]\nenabled = false\n", filepath.ToSlash(pidFile))
	for name, body := range map[string][]byte{"pyxforge.toml": []byte(toml), "boot.bin": testSector()} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	q := make(queue, 1024) // never pumped: the launch's report does not reach the window
	s := NewWithOptions(test.NewTempApp(t), root, Options{Dispatch: q.post})
	s.mach.start(false)
	pid := waitPid(t, pidFile)
	s.Shutdown()
	waitGone(t, "QEMU", pid)
}

// TestHeldProcessesStopAtShutdown covers the hand-over itself: a process held for the UI
// thread is stopped by shutdown unless it was taken over, and one that arrives after shutdown
// is stopped at once.
func TestHeldProcessesStopAtShutdown(t *testing.T) {
	s, _ := newTestShell(t)
	m := s.mach
	var stopped []string
	stop := func(name string) func() { return func() { stopped = append(stopped, name) } }
	_, ok1 := m.hold(stop("waiting"))
	taken, ok2 := m.hold(stop("taken"))
	if !ok1 || !ok2 {
		t.Fatal("hold refused before shutdown")
	}
	taken() // the UI thread owns this one now and stops it itself
	m.shutdown()
	if _, ok := m.hold(stop("late")); ok {
		t.Error("hold accepted a process after shutdown")
	}
	if want := "waiting,late"; strings.Join(stopped, ",") != want {
		t.Errorf("stopped %v, want %s", stopped, want)
	}
	if m.life.Err() == nil {
		t.Error("shutdown did not call off launches in progress")
	}
	m.shutdown() // twice is harmless
}
