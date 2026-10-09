package neovim

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// startReal starts the real Neovim, or skips when it is not installed. CI installs it so these
// tests run there.
func startReal(t *testing.T, events chan<- Event) *Session {
	t.Helper()
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s, err := Start(ctx, Options{Dir: t.TempDir(), Width: 60, Height: 20, OnEvent: func(e Event) {
		if events != nil {
			select {
			case events <- e:
			default:
			}
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, ok := s.WaitFlush(0, 5*time.Second); !ok {
		t.Fatal("no first redraw from Neovim")
	}
	return s
}

// waitGrid waits until some row contains want.
func waitGrid(t *testing.T, s *Session, want string) Snapshot {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		g := s.Grid()
		for r := range g.Rows {
			if strings.Contains(g.Line(r), want) {
				return g
			}
		}
		if time.Now().After(deadline) {
			var rows []string
			for r := range g.Rows {
				rows = append(rows, g.Line(r))
			}
			t.Fatalf("grid never showed %q:\n%s", want, strings.Join(rows, "\n"))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitEvent(t *testing.T, events <-chan Event, kind string, ok func(Event) bool) Event {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Kind == kind && (ok == nil || ok(e)) {
				return e
			}
		case <-timeout:
			t.Fatalf("no %s event", kind)
		}
	}
}

func TestEditSaveAndEvents(t *testing.T) {
	events := make(chan Event, 64)
	s := startReal(t, events)
	if rows, cols := s.Grid().Size(); rows != 20 || cols != 60 {
		t.Fatalf("grid is %dx%d, want 20x60", rows, cols)
	}

	file := filepath.Join(t.TempDir(), "boot.asm")
	if err := os.WriteFile(file, []byte("org 0x7c00\nmov ax, 0x0003\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Open(file); err != nil {
		t.Fatal(err)
	}
	waitGrid(t, s, "mov ax, 0x0003")
	waitEvent(t, events, "BufEnter", func(e Event) bool { return filepath.Base(e.Name) == "boot.asm" })

	// Keys go in as Vim keys: insert a line at the top.
	s.Input("ggOcli<Esc>")
	waitGrid(t, s, "cli")
	waitEvent(t, events, "BufModifiedSet", func(e Event) bool { return e.Modified })
	if m, err := s.Modified(); err != nil || !m {
		t.Fatalf("Modified() = %v, %v", m, err)
	}

	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, "BufWritePost", func(e Event) bool { return !e.Modified })
	data, err := os.ReadFile(file)
	if err != nil || !strings.HasPrefix(string(data), "cli\norg 0x7c00") {
		t.Fatalf("file on disk: %q, %v", data, err)
	}

	// Diagnostics set by any source (LSP later) reach PyxForge with their positions.
	if err := s.ExecLua(`local ns = vim.api.nvim_create_namespace("pyxforge-test")
vim.diagnostic.set(ns, 0, {{ lnum = 2, col = 4, severity = vim.diagnostic.severity.ERROR, message = "bad operand", source = "nasm" }})`, nil); err != nil {
		t.Fatal(err)
	}
	e := waitEvent(t, events, "DiagnosticChanged", func(e Event) bool { return len(e.Diagnostics) == 1 })
	if d := e.Diagnostics[0]; d.Line != 2 || d.Col != 4 || d.Severity != 1 || d.Message != "bad operand" || d.Source != "nasm" {
		t.Errorf("diagnostic = %+v", d)
	}
}

func TestTerminalResizeAndExit(t *testing.T) {
	s := startReal(t, nil)
	// :terminal runs a job on a pseudo-console (ConPTY on Windows) and draws it in the grid.
	cmd := `terminal echo PYXTERM-OK`
	if runtime.GOOS == "windows" {
		cmd = `terminal cmd.exe /c echo PYXTERM-OK`
	}
	if err := s.Command(cmd); err != nil {
		t.Fatal(err)
	}
	waitGrid(t, s, "PYXTERM-OK")

	if err := s.Resize(40, 10); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if rows, cols := s.Grid().Size(); rows == 10 && cols == 40 {
			break
		}
		if time.Now().After(deadline) {
			r, c := s.Grid().Size()
			t.Fatalf("grid is %dx%d after resize, want 10x40", r, c)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Closing ends the process; nothing survives the session.
	var once sync.Once
	exited := make(chan struct{})
	go func() {
		for !s.Exited() {
			time.Sleep(10 * time.Millisecond)
		}
		once.Do(func() { close(exited) })
	}()
	if err := s.Close(); err != nil && !strings.Contains(err.Error(), "closed") {
		t.Logf("close: %v", err)
	}
	select {
	case <-exited:
	case <-time.After(15 * time.Second):
		t.Fatal("Neovim did not exit after Close")
	}
}

func TestNotInstalled(t *testing.T) {
	_, err := Start(context.Background(), Options{Path: filepath.Join(t.TempDir(), "no-nvim-here")})
	if err == nil {
		t.Fatal("a missing executable started")
	}
}
