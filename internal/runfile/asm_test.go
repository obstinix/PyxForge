package runfile

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/toolchain"
)

const bootAsm = "bits 16\norg 0x7c00\nstart: hlt\n    times 510 - ($ - $$) db 0\n    dw 0xaa55\n"

// TestAssemblyPlans: a GNU .s file builds like C; a NASM boot sector assembles to a flat image
// and boots in QEMU with its serial port in the terminal; any other NASM file, or one whose
// boot-sector lines are commented out, needs a pyxforge.toml; a missing tool says which.
func TestAssemblyPlans(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	s := write(t, filepath.Join(root, "hello.s"), ".globl main\nmain:\n  xorl %eax, %eax\n  ret\n")
	p, err := For(s, out, installed("gcc"))
	if err != nil || p.Language != "Assembly" || len(p.Build) != 1 || p.Build[0].Tool != "gcc" ||
		!slices.Contains(p.Build[0].Args, s) || p.Out != outFor(out, s) {
		t.Errorf("hello.s: %+v, %v", p, err)
	}

	dir := filepath.Join(root, "boot, sector") // a comma, which QEMU's -drive option doubles
	boot := write(t, filepath.Join(dir, "boot.asm"), bootAsm)
	p, err = For(boot, out, installed("nasm", "qemu-system-x86_64"))
	if err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(outFor(out, boot), "boot.bin")
	if p.Language != "Assembly (boot sector)" || p.Out != filepath.Dir(image) ||
		!slices.Equal(p.Build[0].Args, []string{"-f", "bin", boot, "-o", image}) || p.Build[0].Tool != "nasm" {
		t.Errorf("boot sector build: %+v", p)
	}
	drive := "format=raw,file=" + strings.ReplaceAll(image, ",", ",,")
	if p.Run.Tool != "qemu-system-x86_64" || !slices.Equal(p.Run.Args, []string{"-drive", drive, "-serial", "stdio"}) {
		t.Errorf("boot sector run: %+v", p.Run)
	}
	if p, _ := For(boot, out, installed("nasm", "qemu-system-i386", "qemu-system-x86_64")); p.Run.Tool != "qemu-system-i386" {
		t.Errorf("QEMU %q, want qemu-system-i386 when both are installed", p.Run.Tool)
	}

	for name, body := range map[string]string{
		"program.asm":   "section .text\nglobal _start\n_start: mov eax, 60\n",
		"commented.asm": "; org 0x7c00\n; dw 0xaa55\nmov ax, 1\n",
	} {
		if _, err := For(write(t, filepath.Join(root, name), body), out, installed("nasm", "qemu-system-i386")); !errors.Is(err, ErrNotBootSector) {
			t.Errorf("%s: %v, want ErrNotBootSector", name, err)
		}
	}
	var missing *MissingError
	if _, err := For(boot, out, installed("qemu-system-i386")); !errors.As(err, &missing) || missing.Language != "Assembly" {
		t.Errorf("without nasm: %v", err)
	}
	if _, err := For(boot, out, installed("nasm")); !errors.As(err, &missing) || missing.Language != "QEMU" {
		t.Errorf("without QEMU: %v", err)
	}
	if _, err := ForDebug(boot, out, installed("nasm", "qemu-system-i386"), nil); !errors.Is(err, ErrNoDebugger) {
		t.Errorf("Debug Current File on a boot sector: %v, want ErrNoDebugger (Debug in QEMU debugs it)", err)
	}
}

// TestAssemblyForReal builds and runs both kinds with the real tools: a GNU assembly program
// that exits 0, and the repository's boot-sector example, which says so on its serial port.
func TestAssemblyForReal(t *testing.T) {
	t.Run("GNU", func(t *testing.T) {
		dir, out := t.TempDir(), t.TempDir()
		s := write(t, filepath.Join(dir, "zero.s"), ".text\n.globl main\nmain:\n  xorl %eax, %eax\n  ret\n")
		p, err := For(s, out, toolchain.LookPath)
		var missing *MissingError
		if errors.As(err, &missing) {
			t.Skip(missing.Error())
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(p.Out, 0o755); err != nil {
			t.Fatal(err)
		}
		run(t, p.Build[0])
		run(t, p.Run) // fails the test unless it exits 0
	})

	t.Run("boot sector", func(t *testing.T) {
		src, err := os.ReadFile(filepath.Join("..", "..", "examples", "boot-sector", "boot.asm"))
		if err != nil {
			t.Fatal(err)
		}
		dir, out := t.TempDir(), t.TempDir()
		boot := write(t, filepath.Join(dir, "boot.asm"), string(src))
		p, err := For(boot, out, toolchain.LookPath)
		var missing *MissingError
		if errors.As(err, &missing) {
			t.Skip(missing.Error())
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(p.Out, 0o755); err != nil {
			t.Fatal(err)
		}
		run(t, p.Build[0])
		image := p.Build[0].Args[len(p.Build[0].Args)-1]
		if data, err := os.ReadFile(image); err != nil || len(data) != 512 || data[510] != 0x55 || data[511] != 0xaa {
			t.Fatalf("image %s: %d bytes, %v", image, len(data), err)
		}
		qemu, err := toolchain.LookPath(p.Run.Tool)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(qemu, append(p.Run.Args, "-display", "none")...) // no window in a test
		cmd.Dir = p.Run.Dir
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
		said := make(chan bool, 1)
		go func() {
			sc := bufio.NewScanner(stdout)
			for sc.Scan() {
				if strings.Contains(sc.Text(), "PyxForge boot sector OK") {
					said <- true
					return
				}
			}
			said <- false
		}()
		select {
		case ok := <-said:
			if !ok {
				t.Error("QEMU's serial port ended without the boot sector's message")
			}
		case <-time.After(30 * time.Second):
			t.Error("no message on the serial port within 30 s")
		}
	})
}
