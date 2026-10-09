package shell

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/kit"
)

// testSector writes "OK" to COM1 and halts: mov dx, 0x3f8 / mov al, 'O' / out / … / hlt.
func testSector() []byte {
	b := make([]byte, 512)
	copy(b, []byte{0xBA, 0xF8, 0x03, 0xB0, 'O', 0xEE, 0xB0, 'K', 0xEE, 0xB0, 0x0A, 0xEE, 0xFA, 0xF4, 0xEB, 0xFD})
	b[510], b[511] = 0x55, 0xAA
	return b
}

func rowsText(v *rowsView) string {
	var b strings.Builder
	for _, r := range v.rows {
		fmt.Fprintf(&b, "%s=%s;", r.name, r.value)
	}
	return b.String()
}

// TestDebugSessionInTheShell boots a sector paused, lets GDB stop it at 0x7c00, checks every
// inspector view, steps, uses the QEMU monitor and the GDB console, then runs without GDB.
func TestDebugSessionInTheShell(t *testing.T) {
	for _, tool := range []string{"qemu-system-x86_64", "gdb"} {
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
	toml := fmt.Sprintf("[project]\nname = \"boot\"\n[qemu]\nmemory = \"16M\"\nboot_image = \"boot.bin\"\n"+
		"extra_args = [\"-serial\", \"stdio\", \"-display\", \"none\"]\n[qemu.debug]\ngdb_port = %d\n", port)
	for name, body := range map[string][]byte{"pyxforge.toml": []byte(toml), "boot.bin": testSector()} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := test.NewTempApp(t)
	q := make(queue, 4096)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	m := s.mach
	t.Cleanup(m.shutdown)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("QEMU:\n%s\nGDB:\n%s\nLog:\n%s", strings.Join(m.qemuOut.Lines(), "\n"),
				strings.Join(m.gdbOut.Lines(), "\n"), strings.Join(s.logLines, "\n"))
		}
	})

	// Before a session the image is already readable.
	s.mviews.loadImage()
	if !strings.Contains(s.mviews.hex.head.Text, "signature 55 aa · 16 of 510 bytes used") ||
		!strings.Contains(rowsText(s.mviews.disasm), "mov dx, 0x3f8") {
		t.Errorf("image views: %q / %s", s.mviews.hex.head.Text, rowsText(s.mviews.disasm))
	}

	m.start(true)
	q.pumpUntil(t, "the stop at 0x7c00", func() bool {
		return m.dbg != nil && m.paused && m.pc == 0x7c00 && !s.mviews.busy && len(s.mviews.regs.rows) > 0
	})
	if s.dock.Selected() != m.qemuTab || !strings.HasPrefix(m.state.Text, "Paused at 0x7c00") || s.statusMachine.text.Text != m.state.Text {
		t.Errorf("state %q, status %q", m.state.Text, s.statusMachine.text.Text)
	}
	regs := rowsText(s.mviews.regs)
	if !strings.Contains(regs, "ip=7c00;") || !strings.Contains(regs, "cs=0000;") {
		t.Errorf("registers: %s", regs)
	}
	marked := slices.IndexFunc(s.mviews.disasm.rows, func(r row) bool { return r.mark })
	if marked < 0 || s.mviews.disasm.rows[marked].name != "00007c00" || s.mviews.disasm.rows[marked].value != "mov dx, 0x3f8" {
		t.Errorf("disassembly: %s", rowsText(s.mviews.disasm))
	}
	if len(s.mviews.flags.rows) < 9 || len(s.mviews.memory.rows) == 0 || !strings.HasPrefix(s.mviews.memory.head.Text, "Memory at the stack") {
		t.Errorf("flags %d rows, memory %q", len(s.mviews.flags.rows), s.mviews.memory.head.Text)
	}

	// One instruction: DX becomes 0x3f8 and is marked as changed.
	m.stepInstruction()
	q.pumpUntil(t, "the step", func() bool {
		return m.paused && m.pc == 0x7c03 && !s.mviews.busy && strings.Contains(rowsText(s.mviews.regs), "dx=03f8;")
	})
	for _, r := range s.mviews.regs.rows {
		if r.name == "dx" && r.role != kit.Warning {
			t.Errorf("dx not marked as changed: %+v", r)
		}
	}

	// The QEMU monitor and the GDB console.
	m.runMonitor("info registers")
	q.pumpUntil(t, "the monitor's answer", func() bool {
		return slices.ContainsFunc(m.qemuOut.Lines(), func(l string) bool { return strings.Contains(l, "EIP=") })
	})
	m.runGDB("info registers rdx")
	q.pumpUntil(t, "the GDB console's answer", func() bool {
		return slices.ContainsFunc(m.gdbOut.Lines(), func(l string) bool { return strings.Contains(l, "0x3f8") })
	})

	// Continue runs to the hlt; Stop ends everything.
	m.cont()
	q.pumpUntil(t, "running", func() bool { return !m.paused })
	m.halt()
	if m.inst != nil || m.dbg != nil || s.statusMachine.Visible() || len(s.mviews.regs.rows) != 0 {
		t.Errorf("after Stop: inst %v, dbg %v, status visible %v", m.inst != nil, m.dbg != nil, s.statusMachine.Visible())
	}

	// Run without GDB: the serial port reaches the QEMU tab.
	m.start(false)
	q.pumpUntil(t, "the serial output", func() bool {
		return m.inst != nil && slices.Contains(m.qemuOut.Lines(), "OK")
	})
	if m.state.Text != "Running" {
		t.Errorf("state %q", m.state.Text)
	}
	m.halt()
}
