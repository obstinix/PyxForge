package shell

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// spinAsm is a boot sector that runs one loop for ever: the guest passes line 5 again and again
// and never reaches the hlt on line 7.
const spinAsm = `org 0x7c00
    cli
    xor ax, ax
spin:
    inc ax                  ; line 5
    jmp spin
    hlt                     ; line 7: never reached
    times 510 - ($ - $$) db 0
    dw 0xaa55
`

// TestBreakpointWhileRunning: a breakpoint set in the editor while the machine runs takes effect
// at once (#20). PyxForge pauses the machine, sets it and continues: one on a line the guest
// never reaches leaves it running, one on its loop stops it there. A pause the user asked for
// stays a pause, even with a breakpoint change made at it.
func TestBreakpointWhileRunning(t *testing.T) {
	for _, tool := range []string{"qemu-system-x86_64", "gdb", "nasm"} {
		if _, err := toolchain.LookPath(tool); err != nil {
			t.Skip(tool + " is not installed")
		}
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	root := t.TempDir()
	toml := fmt.Sprintf("[project]\nname = \"spin\"\n[profiles.boot]\ntool = \"nasm\"\n"+
		"args = [\"-f\", \"bin\", \"-l\", \"build/boot.lst\", \"boot.asm\", \"-o\", \"build/boot.bin\"]\n"+
		"[qemu]\nmemory = \"16M\"\nboot_image = \"build/boot.bin\"\nextra_args = [\"-display\", \"none\"]\n"+
		"[qemu.debug]\ngdb_port = %d\n", port)
	for name, body := range map[string]string{"pyxforge.toml": toml, "boot.asm": spinAsm} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	nasm := exec.Command("nasm", "-f", "bin", "-l", "build/boot.lst", "boot.asm", "-o", "build/boot.bin")
	nasm.Dir = root
	if out, err := nasm.CombinedOutput(); err != nil {
		t.Fatalf("nasm: %v\n%s", err, out)
	}
	asm := filepath.Join(root, "boot.asm")

	a := test.NewTempApp(t)
	q := make(queue, 4096)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	m := s.mach
	t.Cleanup(m.shutdown)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("GDB:\n%s\nLog:\n%s", strings.Join(m.gdbOut.Lines(), "\n"), strings.Join(s.logLines, "\n"))
		}
	})
	// stays checks the machine keeps running (or stays paused) for a while.
	stays := func(paused bool, what string) {
		t.Helper()
		for end := time.Now().Add(1500 * time.Millisecond); time.Now().Before(end); {
			select {
			case f := <-q:
				f()
			case <-time.After(20 * time.Millisecond):
			}
			if m.paused != paused {
				t.Fatalf("%s: paused=%v at %#x (%+v)", what, m.paused, m.pc, m.lastStop)
			}
		}
	}

	m.start(true)
	q.pumpUntil(t, "the stop at 0x7c00", func() bool { return m.dbg != nil && m.paused && m.pc == 0x7c00 && !s.mviews.busy })
	m.cont()
	q.pumpUntil(t, "running", func() bool { return !m.paused })
	stays(false, "spinning")

	// A breakpoint the guest never reaches: set at once, and the machine runs on.
	s.setBreakpoints([]neovim.Breakpoint{{File: asm, Lines: []int{7}}})
	q.pumpUntil(t, "the breakpoint on the hlt set", func() bool { return len(m.bpNums) == 1 && !m.bpResume })
	stays(false, "after setting a breakpoint the guest never reaches")

	// One on its loop: the machine stops there, at the breakpoint.
	s.setBreakpoints([]neovim.Breakpoint{{File: asm, Lines: []int{5, 7}}})
	q.pumpUntil(t, "the stop on the loop", func() bool {
		return m.dbg != nil && m.paused && m.lastStop.Reason == "breakpoint-hit"
	})
	if file, line, ok := m.listingLine(m.pc); !ok || !sameFile(file, asm) || line != 5 {
		t.Errorf("stopped at %#x, %s:%d (%v), want boot.asm:5", m.pc, file, line, ok)
	}

	// The user's pause, with a breakpoint change pending at it, stays paused.
	s.setBreakpoints([]neovim.Breakpoint{{File: asm, Lines: []int{7}}}) // paused: applied at once
	q.pumpUntil(t, "the loop's breakpoint cleared", func() bool { return len(m.bpNums) == 1 })
	m.cont()
	q.pumpUntil(t, "running again", func() bool { return !m.paused })
	s.setBreakpoints(nil)
	m.togglePause()
	q.pumpUntil(t, "the user's pause", func() bool { return m.paused })
	q.pumpUntil(t, "the breakpoints cleared", func() bool { return len(m.bpNums) == 0 })
	stays(true, "after the user's pause")
	m.halt()
}
