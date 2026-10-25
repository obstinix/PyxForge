package shell

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

func draft() machineDraft {
	return machineDraft{Image: "build/boot.bin", QEMU: "qemu-system-i386", Machine: "pc", Memory: "16M", Port: "1234", Arch: "i8086", GDB: "gdb"}
}

// TestMachineFile: the tables Set Up Machine writes make a valid pyxforge.toml, new or added
// to an existing one without changing what it had; a file with [qemu] already, or one that does
// not parse, is left alone; the form's rules hold.
func TestMachineFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my os")
	text, err := machineFile(root, nil, draft())
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Parse(text)
	if err != nil || c.Validate() != nil {
		t.Fatalf("new file does not load (%v):\n%s", err, text)
	}
	if c.Project.Name != "my os" || c.Qemu.BootImage != "build/boot.bin" || c.Qemu.Executable != "qemu-system-i386" ||
		c.Qemu.Memory != "16M" || c.Qemu.Debug.GdbPort != 1234 || !c.Qemu.Debug.Enabled || c.Gdb.Architecture != "i8086" {
		t.Errorf("new file:\n%s\nparsed %+v %+v", text, c.Qemu, c.Gdb)
	}

	old := "# my build\n[project]\nname = \"kernel\"\n\n[profiles.build]\ntool = \"make\"\n\n[gdb]\nexecutable = \"gdb-multiarch\"\narchitecture = \"i386\"\n"
	k := draft()
	k.Kernel, k.Image = true, filepath.Join(root, "out", "kernel.elf") // a path picked with Browse…
	text, err = machineFile(root, []byte(old), k)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, old) || strings.Count(text, "[gdb]") != 1 {
		t.Errorf("the existing file was changed, or [gdb] repeated:\n%s", text)
	}
	if c, _ := config.Parse(text); c == nil || c.Qemu.Kernel != "out/kernel.elf" || c.Qemu.BootImage != "" || c.Gdb.Executable != "gdb-multiarch" {
		t.Errorf("kernel setup:\n%s", text)
	}

	if _, err := machineFile(root, []byte(text), draft()); err == nil || !strings.Contains(err.Error(), "already has a [qemu] table") {
		t.Errorf("a second setup: %v", err)
	}
	if _, err := machineFile(root, []byte("[project\nname="), draft()); err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("a broken file: %v", err)
	}
	for name, change := range map[string]func(*machineDraft){
		"no file":      func(d *machineDraft) { d.Image = " " },
		"outside":      func(d *machineDraft) { d.Image = "../elsewhere/boot.bin" },
		"memory":       func(d *machineDraft) { d.Memory = "lots" },
		"port":         func(d *machineDraft) { d.Port = "70000" },
		"architecture": func(d *machineDraft) { d.Arch = "z80" },
	} {
		d := draft()
		change(&d)
		if _, err := machineFile(root, nil, d); err == nil {
			t.Errorf("%s: accepted %+v", name, d)
		}
	}
	if m, a := defaultsFor("qemu-system-arm"); m != "virt" || a != "arm" {
		t.Errorf("defaults for ARM: %s, %s", m, a)
	}
}

// TestSetUpMachine: a project without a machine shows Set Up Machine… in the QEMU tab, and Run
// asks to set one up instead of failing; the form writes pyxforge.toml, after which Debug boots
// the sector and stops at 0x7c00. A pyxforge.toml that does not load is offered in the editor.
func TestSetUpMachine(t *testing.T) {
	root := t.TempDir()
	a := test.NewTempApp(t)
	q := make(queue, 4096)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	m := s.mach
	t.Cleanup(m.shutdown)
	if !m.setupView.Visible() || findButton(m.setupView, "Set Up Machine…") == nil {
		t.Fatal("no Set Up Machine… in the QEMU tab of a project without a machine")
	}
	s.lastDialog = nil
	m.start(false)
	if s.lastDialog == nil || m.starting || m.inst != nil {
		t.Fatal("Run without a machine did not offer to set one up")
	}
	s.lastDialog.Hide()

	m.setupForm()
	fields := entries(s.win.Canvas().Overlays().Top()) // file, memory, port
	if len(fields) != 3 || fields[1].Text != "128M" || fields[2].Text != "1234" {
		t.Fatalf("Set Up Machine fields: %d", len(fields))
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	// With no file to boot, Save is off from the start and does nothing (#26).
	save := findButton(s.win.Canvas().Overlays().Top(), "Save")
	if save == nil || !save.Disabled() {
		t.Fatal("Save is enabled while the file to boot is empty")
	}
	test.Tap(save)
	if _, err := os.Stat(filepath.Join(root, config.FileName)); err == nil {
		t.Fatal("Save wrote pyxforge.toml without a file to boot")
	}
	fields[0].SetText("../outside.bin")
	if !save.Disabled() {
		t.Error("Save is enabled for a file outside the project")
	}
	fields[0].SetText("boot.bin")
	if save.Disabled() {
		t.Fatal("Save stayed off with a valid file to boot")
	}
	fields[1].SetText("16M")
	fields[2].SetText(fmt.Sprint(port))
	tapInDialog(t, s, "Save")
	file := filepath.Join(root, config.FileName)
	c, err := config.Load(root)
	if err != nil || c.Qemu == nil || c.Qemu.BootImage != "boot.bin" || int(c.Qemu.Debug.GdbPort) != port {
		t.Fatalf("after Save: %+v, %v", c, err)
	}
	if m.setupView.Visible() || !logged(s, "Machine set up in pyxforge.toml") || !logged(s, "boot.bin does not exist yet") {
		t.Errorf("after Save: setup still shown %v; log %v", m.setupView.Visible(), s.logLines)
	}

	// Boot it, with no display window in a test.
	for _, tool := range []string{c.Qemu.Executable, "gdb"} {
		if _, err := toolchain.LookPath(tool); err != nil {
			t.Skip(tool + " is not installed")
		}
	}
	data, _ := os.ReadFile(file)
	withArgs := strings.Replace(string(data), "[qemu]\n", "[qemu]\nextra_args = [\"-display\", \"none\"]\n", 1)
	for name, body := range map[string][]byte{config.FileName: []byte(withArgs), "boot.bin": testSector()} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m.start(true)
	q.pumpUntil(t, "the stop at 0x7c00", func() bool { return m.dbg != nil && m.paused && m.pc == 0x7c00 })
	m.halt()
	q.pumpUntil(t, "QEMU stopped", func() bool { return m.inst == nil && !m.starting })

	// A pyxforge.toml that does not load: Run says why and offers the file.
	if err := os.WriteFile(file, []byte("[project]\nname = \"x\"\n[qemu]\nmemory = \"16M\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.lastDialog = nil
	m.start(false)
	if s.lastDialog == nil || findButton(s.win.Canvas().Overlays().Top(), "Open pyxforge.toml") == nil {
		t.Error("Run with an invalid pyxforge.toml did not offer to open it")
	}
}

// TestCommandLines: QEMU's command line is shown one option to a line, values beside them.
func TestCommandLines(t *testing.T) {
	got := commandLines("qemu-system-i386", []string{"-machine", "pc", "-m", "16M", "-drive", "file=boot.bin,format=raw", "-S", "-gdb", "tcp::1234"})
	want := []string{"==> qemu-system-i386", "      -machine pc", "      -m 16M", "      -drive file=boot.bin,format=raw", "      -S", "      -gdb tcp::1234"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("commandLines:\n%s", strings.Join(got, "\n"))
	}
}
