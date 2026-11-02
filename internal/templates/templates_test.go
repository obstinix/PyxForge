package templates

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/cli"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

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

// create makes a project from the preset and checks its pyxforge.toml loads, with the name given.
func create(t *testing.T, id string) string {
	t.Helper()
	p, ok := ByID(id)
	if !ok {
		t.Fatalf("no preset %q", id)
	}
	dir := filepath.Join(t.TempDir(), "my first os") // quotes go in the name only: Windows allows none in folder names
	if err := Create(dir, `my "first" os`, p); err != nil {
		t.Fatal(err)
	}
	if id != "empty" {
		c, err := config.Load(dir)
		if err != nil || c.Validate() != nil || c.Project.Name != `my "first" os` {
			t.Fatalf("%s: pyxforge.toml: %+v, %v", id, c, err)
		}
	}
	return dir
}

func cliIn(dir string, out, errOut *syncBuffer, ctx context.Context, args ...string) cli.Result {
	env := cli.Env{Ctx: ctx, Stdout: out, Stderr: errOut, Getwd: func() (string, error) { return dir, nil }, Detect: toolchain.DetectFresh}
	return cli.Run(args, env)
}

// TestPresetsBuild: each preset makes a project that builds as it is; the boot sector boots in
// QEMU and prints over its serial port, and the C program runs (#72).
func TestPresetsBuild(t *testing.T) {
	if entries, _ := os.ReadDir(create(t, "empty")); len(entries) != 0 {
		t.Errorf("the empty preset wrote %v", entries)
	}
	if err := Create(t.TempDir(), "x", Presets[0]); err == nil {
		t.Error("a project was made in a folder that exists")
	}

	t.Run("boot sector", func(t *testing.T) {
		for _, tool := range []string{"nasm", "qemu-system-x86_64"} {
			if _, err := toolchain.LookPath(tool); err != nil {
				t.Skip(tool + " is not installed")
			}
		}
		dir := create(t, "boot-sector")
		var out, errOut syncBuffer
		if r := cliIn(dir, &out, &errOut, context.Background(), "build"); r.Exit != cli.ExitOK {
			t.Fatalf("build: %+v\n%s%s", r, out.String(), errOut.String())
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan cli.Result, 1)
		go func() { done <- cliIn(dir, &out, &errOut, ctx, "run") }()
		deadline := time.After(20 * time.Second)
		for !strings.Contains(out.String(), "PyxForge boot sector OK") {
			select {
			case <-deadline:
				t.Fatalf("no serial output.\nstdout: %s\nstderr: %s", out.String(), errOut.String())
			case r := <-done:
				t.Fatalf("run ended early: %+v\n%s", r, errOut.String())
			case <-time.After(20 * time.Millisecond):
			}
		}
		cancel()
		<-done
	})

	t.Run("C program", func(t *testing.T) {
		if _, err := toolchain.LookPath("gcc"); err != nil {
			t.Skip("gcc is not installed")
		}
		dir := create(t, "c-program")
		var out, errOut syncBuffer
		if r := cliIn(dir, &out, &errOut, context.Background(), "build"); r.Exit != cli.ExitOK {
			t.Fatalf("build: %+v\n%s%s", r, out.String(), errOut.String())
		}
		exe := filepath.Join(dir, "build", "hello")
		if _, err := os.Stat(exe + ".exe"); err == nil {
			exe += ".exe"
		}
		got, err := exec.Command(exe).CombinedOutput()
		if err != nil || strings.TrimSpace(string(got)) != "hello from PyxForge: 42" {
			t.Errorf("the program printed %q (%v)", got, err)
		}
	})
}
