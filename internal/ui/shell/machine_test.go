package shell

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/qemu"
	"github.com/obstinix/PyxForge/internal/snapshot"
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

	// GDB dying on its own is reported, and QEMU keeps running without it.
	gdbProc, err := os.FindProcess(m.dbg.Pid())
	if err != nil {
		t.Fatal(err)
	}
	_ = gdbProc.Kill()
	q.pumpUntil(t, "the GDB exit report", func() bool { return m.dbg == nil && strings.HasPrefix(m.state.Text, "GDB exited") })
	if m.inst == nil || m.debugOn {
		t.Errorf("after GDB died: inst %v, debugOn %v", m.inst != nil, m.debugOn)
	}

	// Debug again at once: the old QEMU is stopped and the new one gets the same GDB port.
	m.start(true)
	m.start(true) // a second press while starting is ignored
	q.pumpUntil(t, "the second session's stop", func() bool {
		return m.dbg != nil && m.paused && m.pc == 0x7c00 && !s.mviews.busy && len(s.mviews.regs.rows) > 0
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

// TestSectorMapView maps the boot image in the inspector, seeks the hex view from the map, and
// follows a rebuilt image.
func TestSectorMapView(t *testing.T) {
	root := t.TempDir()
	toml := "[project]\nname = \"boot\"\n[qemu]\nboot_image = \"boot.bin\"\n"
	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), testSector(), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewWithOptions(test.NewTempApp(t), root, Options{})
	v := s.mviews
	v.loadImage()
	var got []string
	for _, r := range v.sector.m.Regions {
		got = append(got, fmt.Sprintf("%s[%d,%d)", r.Kind, r.Start, r.End))
	}
	if strings.Join(got, " ") != "code[0,16) padding[16,510) signature[510,512)" || !strings.Contains(v.sector.head.Text, "bootable") {
		t.Errorf("map %v, head %q", got, v.sector.head.Text)
	}

	// The signature's legend row seeks the hex view to its line.
	v.sector.legend.OnSelected(2)
	if s.inspectorTabs.Selected().Text != "Hex" || !v.hex.rows[31].mark || v.hex.rows[0].mark {
		t.Errorf("seek: tab %q, row 31 marked %v", s.inspectorTabs.Selected().Text, v.hex.rows[31].mark)
	}

	// A rebuilt image with a broken signature is shown when the Map tab is selected again.
	bad := testSector()
	bad[511] = 0
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, it := range s.inspectorTabs.Items {
		if it.Text == "Map" {
			s.inspectorTabs.Select(it)
		}
	}
	if !strings.Contains(v.sector.head.Text, "not bootable") || v.sector.m.SignatureOK {
		t.Errorf("after the rebuild: %q", v.sector.head.Text)
	}

	// Disassemble As… decodes the same bytes in another mode.
	if !strings.Contains(v.disasm.head.Text, "Real mode") || v.disasm.rows[0].value != "mov dx, 0x3f8" {
		t.Errorf("real-mode listing: %q %+v", v.disasm.head.Text, v.disasm.rows[0])
	}
	v.mode = 32
	v.loadImage()
	if !strings.Contains(v.disasm.head.Text, "Protected mode") || v.disasm.rows[0].value == "mov dx, 0x3f8" {
		t.Errorf("protected-mode listing: %q %+v", v.disasm.head.Text, v.disasm.rows[0])
	}

	// A Windows executable is named, not judged as a boot sector.
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), append([]byte("MZ"), make([]byte, 700)...), 0o644); err != nil {
		t.Fatal(err)
	}
	v.loadImage()
	if !strings.Contains(v.hex.head.Text, "a PE executable") || len(v.sector.m.Regions) != 0 {
		t.Errorf("PE file: %q, map %+v", v.hex.head.Text, v.sector.m.Regions)
	}
}

// TestSnapshotsInTheShell captures and compares diagnostic snapshots, and saves and restores a
// machine state, against a real QEMU booted through its snapshot overlay with GDB attached.
func TestSnapshotsInTheShell(t *testing.T) {
	for _, tool := range []string{"qemu-system-x86_64", "qemu-img", "gdb"} {
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
	toml := fmt.Sprintf("[project]\nname = \"boot\"\n[qemu]\nmemory = \"16M\"\nboot_image = \"boot.bin\"\nsnapshots = true\n"+
		"extra_args = [\"-display\", \"none\"]\n[qemu.debug]\ngdb_port = %d\n", port)
	for name, body := range map[string][]byte{"pyxforge.toml": []byte(toml), "boot.bin": testSector()} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	q := make(queue, 4096)
	s := NewWithOptions(test.NewTempApp(t), root, Options{Dispatch: q.post})
	m, p := s.mach, s.snaps
	p.store = &snapshot.Store{Dir: t.TempDir()}
	if dir, err := qemu.OverlayDir(root); err == nil {
		t.Cleanup(func() { _ = os.RemoveAll(dir) }) // the overlay lives in the user cache folder
	}
	t.Cleanup(m.shutdown)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Snapshots:\n%s\nQEMU:\n%s", strings.Join(p.out.Lines(), "\n"), strings.Join(m.qemuOut.Lines(), "\n"))
		}
	})
	paused := func(pc uint64) func() bool {
		return func() bool {
			return m.dbg != nil && m.paused && m.pc == pc && !s.mviews.busy && len(s.mviews.regs.rows) > 0
		}
	}

	m.start(true)
	q.pumpUntil(t, "the stop at 0x7c00", paused(0x7c00))
	if !slices.ContainsFunc(m.qemuOut.Lines(), func(l string) bool { return strings.Contains(l, "format=qcow2") }) {
		t.Fatalf("QEMU was not booted through the overlay")
	}

	// Two diagnostic snapshots around one instruction, then their comparison.
	p.captureNamed("start")
	q.pumpUntil(t, "the first capture", func() bool { return !p.busy && len(p.diag) == 1 })
	m.stepInstruction()
	q.pumpUntil(t, "the step", paused(0x7c03))
	p.captureNamed("after mov dx")
	q.pumpUntil(t, "the second capture", func() bool { return !p.busy && len(p.diag) == 2 })
	if p.diag[0].Name != "after mov dx" || p.diag[0].PC != 0x7c03 || p.diag[1].ImageHash == "" || len(p.diag[1].Listing) == 0 {
		t.Fatalf("snapshots %+v", p.diag)
	}
	p.showCompare(&p.diag[1], &p.diag[0])
	text := strings.Join(p.out.Lines(), "\n")
	if !strings.Contains(text, "start (pc 7c00) → after mov dx (pc 7c03)") || !regexpMatch(`rdx +\w*?0 → \w*3f8`, text) {
		t.Errorf("comparison:\n%s", text)
	}

	// A machine state, saved at 0x7c03, brings the machine back after another step.
	p.saveMachineNamed("at-7c03")
	q.pumpUntil(t, "the saved state", func() bool { return !p.busy && len(p.machine) == 1 && p.machine[0].Tag == "at-7c03" })
	m.stepInstruction()
	q.pumpUntil(t, "another step", paused(0x7c05))
	p.restoreMachine("at-7c03")
	q.pumpUntil(t, "the restored machine", func() bool {
		return m.pc == 0x7c03 && slices.Contains(p.out.Lines(), "==> Restored machine state at-7c03")
	})
	p.saveMachineNamed("bad name")
	q.pumpUntil(t, "the refused name", func() bool {
		return slices.ContainsFunc(p.out.Lines(), func(l string) bool { return strings.HasPrefix(l, "Save machine state failed") })
	})
	m.halt()
	if len(p.machBox.Objects) != 2 { // the title and the "not running" hint
		t.Errorf("machine list after Stop shows %d rows", len(p.machBox.Objects))
	}
}

func regexpMatch(pattern, text string) bool { return regexp.MustCompile(pattern).MatchString(text) }
