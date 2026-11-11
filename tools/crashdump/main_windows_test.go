package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func build(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "crashdump.exe")
	if out, err := exec.Command("go", "build", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return exe
}

// TestPassesThrough: the program gets crashdump's standard streams and arguments, and
// crashdump exits with its status.
func TestPassesThrough(t *testing.T) {
	exe := build(t)
	out, err := exec.Command(exe, "cmd.exe", "/c", "echo hello & exit 7").Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("exit: %v", err)
	}
	if strings.TrimSpace(string(out)) != "hello" {
		t.Errorf("output %q", out)
	}
}

// TestCatchesACrash: a program that writes through a null pointer dies as it would, and leaves
// a minidump and a report naming the exception, the module and the faulting address.
func TestCatchesACrash(t *testing.T) {
	cc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc is not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "crasher.c")
	if err := os.WriteFile(src, []byte("int main(void) { *(volatile int *)0 = 1; return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	crasher := filepath.Join(dir, "crasher.exe")
	if out, err := exec.Command(cc, "-O0", "-o", crasher, src).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v\n%s", err, out)
	}
	dumps := filepath.Join(dir, "dumps")
	cmd := exec.Command(build(t), crasher)
	cmd.Env = append(os.Environ(), "PYXFORGE_CRASHDUMP_DIR="+dumps)
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || uint32(exit.ExitCode()) != 0xC0000005 {
		t.Fatalf("exit: %v, want 0xC0000005", err)
	}
	reports, _ := filepath.Glob(filepath.Join(dumps, "crasher-*.txt"))
	dmps, _ := filepath.Glob(filepath.Join(dumps, "crasher-*.dmp"))
	if len(reports) != 1 || len(dmps) != 1 {
		t.Fatalf("reports %v, dumps %v", reports, dmps)
	}
	if fi, err := os.Stat(dmps[0]); err != nil || fi.Size() < 1024 {
		t.Errorf("minidump: %v, %v", fi, err)
	}
	text, _ := os.ReadFile(reports[0])
	for _, want := range []string{"exception 0xC0000005 at crasher.exe+0x", "access violation writing 0x0", "minidump", ": written", "rip ", "modules:"} {
		if !strings.Contains(string(text), want) {
			t.Errorf("the report has no %q:\n%s", want, text)
		}
	}
}
