package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/toolchain"
)

// sector writes "OK" to COM1 and halts (the image the qemu tests boot).
func sector() []byte {
	b := make([]byte, 512)
	copy(b, []byte{0xBA, 0xF8, 0x03, 0xB0, 'O', 0xEE, 0xB0, 'K', 0xEE, 0xB0, 0x0A, 0xEE, 0xFA, 0xF4, 0xEB, 0xFD})
	b[510], b[511] = 0x55, 0xAA
	return b
}

// syncBuffer is a bytes.Buffer safe for QEMU's reader goroutine and the test to share.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func TestRunBootsTheImage(t *testing.T) {
	if _, err := toolchain.LookPath("qemu-system-x86_64"); err != nil {
		t.Skip("qemu-system-x86_64 is not installed")
	}
	root := buildProject(t, "[qemu]\nmemory = \"16M\"\nboot_image = \"boot.bin\"\nextra_args = [\"-serial\", \"stdio\", \"-display\", \"none\"]\n")
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), sector(), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errOut syncBuffer
	env := Env{Ctx: ctx, Stdout: &out, Stderr: &errOut, Getwd: func() (string, error) { return root, nil }}
	done := make(chan Result, 1)
	go func() { done <- Run([]string{"run"}, env) }()
	deadline := time.After(15 * time.Second)
	for !strings.Contains(out.String(), "OK") {
		select {
		case <-deadline:
			t.Fatalf("no serial output.\nstdout: %s\nstderr: %s", out.String(), errOut.String())
		case r := <-done:
			t.Fatalf("run ended early: %+v\n%s", r, errOut.String())
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel() // Ctrl+C
	select {
	case r := <-done:
		if r.Exit != ExitOK || !strings.Contains(errOut.String(), "QEMU stopped.") {
			t.Errorf("after Ctrl+C: %+v\n%s", r, errOut.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("QEMU did not stop")
	}
}

func TestRunNeedsSomethingToBoot(t *testing.T) {
	root := buildProject(t, "")
	if r := do(t, root, nil, "run"); r.res.Exit != ExitFailure || !strings.Contains(r.stderr, "no [qemu] table") {
		t.Errorf("no [qemu]: %+v %q", r.res, r.stderr)
	}
	root = buildProject(t, "[qemu]\nboot_image = \"build/boot.bin\"\n")
	if r := do(t, root, nil, "run", "--no-build"); r.res.Exit != ExitFailure || !strings.Contains(r.stderr, "build the project first") {
		t.Errorf("no image: %+v %q", r.res, r.stderr)
	}
}

func TestInspect(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "boot.bin")
	if err := os.WriteFile(img, sector(), 0o644); err != nil {
		t.Fatal(err)
	}
	r := do(t, dir, nil, "inspect", img, "--disasm")
	for _, want := range []string{"signature 55 aa at 510: a BIOS boots it", "use 16 of the sector's 510 bytes; 494 are free",
		"00000000  ba f8 03 b0 4f ee", "7c00:  ba f8 03", "mov dx, 0x3f8",
		"000-00f   16 B  unclassified Code and data", "1fe-1ff    2 B  known        Boot signature"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("inspect lacks %q:\n%s", want, r.stdout)
		}
	}
	r = do(t, dir, nil, "inspect", "--json", img)
	var got struct{ Boot struct{ Signature bool } }
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || !got.Boot.Signature {
		t.Errorf("--json: %v %s", err, r.stdout)
	}
	short := filepath.Join(dir, "short.bin")
	if err := os.WriteFile(short, []byte{0xf4}, 0o644); err != nil {
		t.Fatal(err)
	}
	if r := do(t, dir, nil, "inspect", short); !strings.Contains(r.stdout, "shorter than a 512-byte sector") {
		t.Errorf("short image:\n%s", r.stdout)
	}
	elf, _ := filepath.Abs("../inspect/testdata/kernel.elf")
	if r := do(t, dir, nil, "inspect", elf, "--disasm"); !strings.Contains(r.stdout, "ELF32 ET_EXEC EM_386, entry 0x100000") ||
		!strings.Contains(r.stdout, "label  kmain") || !strings.Contains(r.stdout, "mov esp, 0x102000") {
		t.Errorf("ELF:\n%s", r.stdout)
	}
	if r := do(t, dir, nil, "inspect"); r.res.Exit != ExitUsage {
		t.Errorf("no file: %+v", r.res)
	}
}

func TestInspectOtherFormats(t *testing.T) {
	dir := t.TempDir()
	pe := filepath.Join(dir, "tool.exe")
	if err := os.WriteFile(pe, append([]byte("MZ"), make([]byte, 600)...), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := do(t, dir, nil, "inspect", pe); r.res.Exit != ExitOK || !strings.Contains(r.stdout, "is a PE (Windows executable) file") ||
		strings.Contains(r.stdout, "BIOS") {
		t.Errorf("PE file:\n%s", r.stdout)
	}
	img := filepath.Join(dir, "disk.img")
	data := make([]byte, 4096)
	copy(data, sector())
	if err := os.WriteFile(img, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if r := do(t, dir, nil, "inspect", img); !strings.Contains(r.stdout, "The first sector of 4096 bytes:") ||
		!strings.Contains(r.stdout, "a BIOS boots it") {
		t.Errorf("disk image:\n%s", r.stdout)
	}
}
