package shell

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/neovim"
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

	// An editor breakpoint in the project: a flat boot sector has no line information, so GDB
	// cannot set it, and the GDB tab says so; one outside the project is not GDB's business.
	s.breakpoints = []neovim.Breakpoint{{File: filepath.Join(root, "boot.asm"), Lines: []int{3}},
		{File: filepath.Join(t.TempDir(), "other.c"), Lines: []int{1}}}
	m.start(true)
	q.pumpUntil(t, "the stop at 0x7c00", func() bool {
		return m.dbg != nil && m.paused && m.pc == 0x7c00 && !s.mviews.busy && len(s.mviews.regs.rows) > 0
	})
	notSet := func(at string) int {
		n := 0
		for _, l := range m.gdbOut.Lines() {
			if strings.HasPrefix(l, "Breakpoint not set at "+at) {
				n++
			}
		}
		return n
	}
	if notSet("boot.asm:3") != 1 || notSet("") != 1 {
		t.Errorf("editor breakpoints at attach: GDB tab %q", m.gdbOut.Lines())
	}
	// Set while paused: passed on at once.
	s.setBreakpoints([]neovim.Breakpoint{{File: filepath.Join(root, "boot.asm"), Lines: []int{3, 5}}})
	q.pumpUntil(t, "the new breakpoint tried", func() bool { return notSet("boot.asm:5") == 1 })
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
	noToastExpiry(s)
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

// TestViewsFollowTheProcessorMode: the inspector decodes in the mode the processor is in, not
// the one [gdb] architecture implies. The bytes are PyxisOS's Multiboot entry, which runs in
// 32-bit protected mode in a kernel debugged as i386:x86-64; decoded as 64-bit, the store's
// address took eight bytes and read 0xf0041d890013f000.
func TestViewsFollowTheProcessorMode(t *testing.T) {
	for _, c := range []struct {
		regs map[string]uint64
		want int
	}{
		{map[string]uint64{"cr0": 0x10}, 16},
		{map[string]uint64{"cr0": 0x11, "efer": 0}, 32},
		{map[string]uint64{"cr0": 0x80000011, "efer": 0x500}, 64},
		{map[string]uint64{"cr0": 0x80000011, "efer": 0x100}, 32}, // LME set, paging not yet on: not active
		{map[string]uint64{"eax": 1}, 64},                         // no control registers: the configured mode
	} {
		if got := cpuMode(c.regs, 64); int(got) != c.want {
			t.Errorf("cpuMode(%v) = %d, want %d", c.regs, got, c.want)
		}
	}
	s, _ := newTestShell(t)
	entry := []byte{0xfa, 0xfc, 0xa3, 0x00, 0xf0, 0x13, 0x00, 0x89, 0x1d, 0x04, 0xf0, 0x13, 0x00, 0xbc, 0x00, 0xf0, 0x13, 0x00}
	regs := map[string]uint64{"rip": 0x100028, "rax": 0x2badb002, "cr0": 0x11, "efer": 0, "cs": 8}
	s.mviews.apply(machineRead{mode: cpuMode(regs, 64), regs: regs, code: entry, start: 0x100028}, 0x100028)
	if !strings.HasPrefix(s.mviews.regs.head.Text, "Protected mode") || !strings.Contains(rowsText(s.mviews.regs), "eax=2badb002;") {
		t.Errorf("registers: %q %s", s.mviews.regs.head.Text, rowsText(s.mviews.regs))
	}
	if got := rowsText(s.mviews.disasm); !strings.Contains(got, "0010002a=mov dword ptr [0x13f000], eax;0010002f=mov dword ptr [0x13f004], ebx;00100035=mov esp, 0x13f000;") {
		t.Errorf("disassembly: %s", got)
	}
}

// TestBreakpointInABootSector: an editor breakpoint on a line of a flat boot sector is set at
// its address from the NASM listing the build profile writes, QEMU stops there on Continue,
// and the stop maps back to that line (#61).
func TestBreakpointInABootSector(t *testing.T) {
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
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "boot-sector", "boot.asm"))
	if err != nil {
		t.Fatal(err)
	}
	toml := fmt.Sprintf("[project]\nname = \"boot\"\n[profiles.boot]\ntool = \"nasm\"\n"+
		"args = [\"-f\", \"bin\", \"-l\", \"build/boot.lst\", \"boot.asm\", \"-o\", \"build/boot.bin\"]\n"+
		"[qemu]\nmemory = \"16M\"\nboot_image = \"build/boot.bin\"\nextra_args = [\"-display\", \"none\"]\n"+
		"[qemu.debug]\ngdb_port = %d\n", port)
	for name, body := range map[string][]byte{"pyxforge.toml": []byte(toml), "boot.asm": src} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o644); err != nil {
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
	const lodsb = 18 // "lodsb", at 0x7c10
	if lines := strings.Split(string(src), "\n"); !strings.Contains(lines[lodsb-1], "lodsb") {
		t.Fatalf("line %d of the example is %q; the test expects lodsb there", lodsb, lines[lodsb-1])
	}

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
	s.breakpoints = []neovim.Breakpoint{{File: asm, Lines: []int{lodsb, 1}}} // line 1 is a comment
	m.start(true)
	q.pumpUntil(t, "the stop at 0x7c00", func() bool { return m.dbg != nil && m.paused && m.pc == 0x7c00 && !s.mviews.busy })
	q.pumpUntil(t, "the breakpoint set", func() bool {
		return slices.ContainsFunc(m.gdbOut.Lines(), func(l string) bool {
			return strings.HasPrefix(l, "Breakpoint ") && strings.HasSuffix(l, fmt.Sprintf("at boot.asm:%d (from the editor)", lodsb))
		})
	})
	if !slices.ContainsFunc(m.gdbOut.Lines(), func(l string) bool { return strings.HasPrefix(l, "Breakpoint not set at boot.asm:1:") }) {
		t.Errorf("a comment line was not reported as not set: %q", m.gdbOut.Lines())
	}
	m.cont()
	q.pumpUntil(t, "the stop at the lodsb", func() bool { return m.dbg != nil && m.paused && m.pc == 0x7c10 })
	if file, line, ok := m.listingLine(m.pc); !ok || !sameFile(file, asm) || line != lodsb {
		t.Errorf("0x7c10 maps to %s:%d (%v)", file, line, ok)
	}
	// The Stack view names the frame's line from the listing, as GDB has none for it.
	q.pumpUntil(t, "the call stack", func() bool {
		return !s.mviews.busy && len(s.mviews.stack.rows) > 0 && strings.Contains(s.mviews.stack.rows[0].value, "0x7c10")
	})
	if top := s.mviews.stack.rows[0].value; !strings.Contains(top, fmt.Sprintf("boot.asm:%d", lodsb)) {
		t.Errorf("the top frame reads %q", top)
	}
	// The lodsb runs once a character: Continue stops there again.
	// The second stop is GDB's second report of the breakpoint: paused at 0x7c10 alone is still
	// true of the first stop until the Continue reaches GDB.
	hits := func() int {
		n := 0
		for _, l := range m.gdbOut.Lines() {
			if strings.HasPrefix(l, "Breakpoint 2, 0x") {
				n++
			}
		}
		return n
	}
	m.cont()
	q.pumpUntil(t, "the second stop at the lodsb", func() bool {
		return m.dbg != nil && m.paused && m.pc == 0x7c10 && !s.mviews.busy && hits() == 2
	})
	// Cleared while paused, it is deleted in GDB (#48): Continue then runs on to the hlt.
	s.setBreakpoints(nil)
	q.pumpUntil(t, "the breakpoint deleted", func() bool { return len(m.bpNums) == 0 })
	m.runGDB("info breakpoints")
	var listed []string // GDB's list after the command; PyxForge's own stop at 0x7c00 stays
	q.pumpUntil(t, "GDB's list", func() bool {
		lines := m.gdbOut.Lines()
		i := slices.IndexFunc(lines, func(l string) bool { return strings.HasSuffix(l, "info breakpoints") })
		listed = lines[i+1:]
		return i >= 0 && slices.ContainsFunc(listed, func(l string) bool { return strings.Contains(l, "0x0000000000007c00") })
	})
	if slices.ContainsFunc(listed, func(l string) bool { return strings.Contains(l, "7c10") }) {
		t.Errorf("the cleared breakpoint is still in GDB:\n%s", strings.Join(listed, "\n"))
	}
	m.cont()
	q.pumpUntil(t, "running", func() bool { return !m.paused })
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); {
		select {
		case f := <-q:
			f()
		case <-time.After(20 * time.Millisecond):
		}
		if m.paused {
			t.Fatalf("stopped again at %#x after the breakpoint was cleared: %+v", m.pc, m.lastStop)
		}
	}
	m.halt()
}

// kernelAsm is a multiboot kernel small enough to read: GDB knows its lines from NASM's DWARF.
const kernelAsm = `bits 32
section .text
align 4
    dd 0x1BADB002            ; multiboot magic
    dd 0                     ; flags
    dd -0x1BADB002           ; checksum
global _start
_start:
    mov eax, 1
    mov ebx, 2
    add eax, ebx
.halt:
    hlt
    jmp .halt
`

// elfLinker finds a linker that writes 32-bit x86 ELF: GNU ld built for it, or LLVM's ld.lld.
func elfLinker() []string {
	if p, err := exec.LookPath("ld.lld"); err == nil {
		return []string{p, "-m", "elf_i386"}
	}
	if out, err := exec.Command("ld", "-V").Output(); err == nil && strings.Contains(string(out), "elf_i386") {
		return []string{"ld", "-m", "elf_i386"}
	}
	return nil
}

// TestBreakpointInAnELFKernel: in a kernel with DWARF line information, an editor breakpoint is
// set by file and line in GDB, QEMU stops there, the stop names that source line, and clearing
// the breakpoint while paused deletes it (#48, #58).
func TestBreakpointInAnELFKernel(t *testing.T) {
	for _, tool := range []string{"qemu-system-x86_64", "gdb", "nasm"} {
		if _, err := toolchain.LookPath(tool); err != nil {
			t.Skip(tool + " is not installed")
		}
	}
	link := elfLinker()
	if link == nil {
		t.Skip("no linker for 32-bit ELF (GNU ld with elf_i386, or ld.lld)")
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	root := t.TempDir()
	asm := filepath.Join(root, "kernel.asm")
	toml := fmt.Sprintf("[project]\nname = \"kernel\"\n[qemu]\nmemory = \"16M\"\nkernel = \"kernel.elf\"\n"+
		"extra_args = [\"-display\", \"none\"]\n[qemu.debug]\ngdb_port = %d\n[gdb]\narchitecture = \"i386\"\n", port)
	script := "ENTRY(_start)\nSECTIONS { . = 0x100000; .text : { *(.text) } }\n" // GNU ld and lld both read it
	for name, body := range map[string]string{"pyxforge.toml": toml, "kernel.asm": kernelAsm, "link.ld": script} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range [][]string{{"nasm", "-f", "elf32", "-g", "-F", "dwarf", "kernel.asm", "-o", "kernel.o"},
		append(link, "-T", "link.ld", "kernel.o", "-o", "kernel.elf")} {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c, err, out)
		}
	}
	const movEbx = 10 // "mov ebx, 2"

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
	s.breakpoints = []neovim.Breakpoint{{File: asm, Lines: []int{movEbx}}}
	m.start(true)
	// The entry is the first stop at a breakpoint: attaching reports one at the reset vector
	// first, which under load the test took for the entry.
	q.pumpUntil(t, "the stop at the entry", func() bool {
		return m.dbg != nil && m.paused && m.lastStop.Reason == "breakpoint-hit" && !s.mviews.busy
	})
	entry := m.pc
	if len(m.bpNums) != 1 {
		t.Fatalf("the editor breakpoint was not set by its line: %v", m.bpNums)
	}
	m.cont()
	q.pumpUntil(t, "the stop at the line", func() bool { return m.dbg != nil && m.paused && m.pc != entry })
	if stopped := m.lastStop; !sameFile(stopped.File, asm) || stopped.Line != movEbx {
		t.Errorf("stopped at %s:%d (%#x), want kernel.asm:%d", stopped.File, stopped.Line, m.pc, movEbx)
	}
	// The Stack view shows the frame with its function and source line.
	q.pumpUntil(t, "the call stack", func() bool { return !s.mviews.busy && len(s.mviews.stack.rows) > 0 })
	if top := s.mviews.stack.rows[0]; !strings.Contains(top.value, "_start") || !strings.Contains(top.value, fmt.Sprintf("kernel.asm:%d", movEbx)) {
		t.Errorf("the top frame reads %q", top.value)
	}
	s.setBreakpoints(nil)
	q.pumpUntil(t, "the breakpoint deleted", func() bool { return len(m.bpNums) == 0 })
	m.halt()
}
