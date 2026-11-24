package pty

import (
	"bytes"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// prog is a command for each platform: Windows' cmd.exe, a Unix sh.
func prog(win, unix string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd.exe", "/d", "/c", win}
	}
	return []string{"/bin/sh", "-c", unix}
}

// reader collects a PTY's output until io.EOF.
type reader struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	done chan struct{}
}

func readAll(p *PTY) *reader {
	r := &reader{done: make(chan struct{})}
	go func() {
		defer close(r.done)
		b := make([]byte, 4096)
		for {
			n, err := p.Read(b)
			r.mu.Lock()
			r.buf.Write(b[:n])
			r.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return r
}

func (r *reader) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

func (r *reader) waitFor(t *testing.T, s string) {
	t.Helper()
	end := time.Now().Add(20 * time.Second)
	for !strings.Contains(r.String(), s) {
		if time.Now().After(end) {
			t.Fatalf("no %q in the output: %q", s, r.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (r *reader) waitEOF(t *testing.T) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(20 * time.Second):
		t.Fatalf("the output did not end: %q", r.String())
	}
}

// TestProgramOutputAndStatus: a program's whole output is read, then io.EOF, and its exit
// status is reported, a failing one's included.
func TestProgramOutputAndStatus(t *testing.T) {
	for _, c := range []struct {
		argv   []string
		status int
	}{
		{prog("echo PYXTERM-OK", "echo PYXTERM-OK"), 0},
		{prog("echo PYXTERM-OK & exit 3", "echo PYXTERM-OK; exit 3"), 3},
	} {
		p, err := Start(Options{Argv: c.argv, Dir: t.TempDir(), Cols: 80, Rows: 24})
		if err != nil {
			t.Fatal(err)
		}
		r := readAll(p)
		if got := p.Wait(); got != c.status {
			t.Errorf("%v: status %d, want %d", c.argv, got, c.status)
		}
		r.waitEOF(t)
		if !strings.Contains(r.String(), "PYXTERM-OK") {
			t.Errorf("%v: output %q", c.argv, r.String())
		}
		if err := p.Resize(40, 10); err != ErrClosed {
			t.Errorf("Resize after the end: %v", err)
		}
		_ = p.Close()
	}
}

// TestTypingIntoAShell: what is written reaches the shell, whose answer is read; Close ends it.
func TestTypingIntoAShell(t *testing.T) {
	argv := []string{"/bin/sh"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/d", "/k"}
	}
	p, err := Start(Options{Argv: argv, Dir: t.TempDir(), Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	r := readAll(p)
	// The answer is not in what was typed, so its echo cannot pass for it.
	typed, answer := "echo PYX-$((6*7))\r", "PYX-42"
	if runtime.GOOS == "windows" {
		typed, answer = "echo PYX-%COMSPEC:~-7%\r", "PYX-cmd.exe"
	}
	if _, err := p.Write([]byte(typed)); err != nil {
		t.Fatal(err)
	}
	r.waitFor(t, answer)
	if err := p.Resize(60, 12); err != nil {
		t.Errorf("Resize: %v", err)
	}
	_ = p.Close()
	select {
	case <-p.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the shell outlived Close")
	}
	r.waitEOF(t)
	if _, err := p.Write([]byte("x")); err == nil {
		t.Error("Write after Close succeeded")
	}
}

// TestResizeRightAfterExit is #80's window: a resize just as a program ends, then close. Only
// a crash of PyxForge fails it.
func TestResizeRightAfterExit(t *testing.T) {
	for round := range 6 {
		p, err := Start(Options{Argv: prog("echo PYXTERM-OK", "echo PYXTERM-OK"), Cols: 80, Rows: 24})
		if err != nil {
			t.Fatal(err)
		}
		r := readAll(p)
		gap := time.Duration([]int{5, 0, 2, 10, 1, 20}[round]) * time.Millisecond
		r.waitFor(t, "PYXTERM-OK")
		_ = p.Resize(40, 10)
		time.Sleep(gap)
		_ = p.Close()
		_ = p.Resize(30, 8)
		p.Wait()
		r.waitEOF(t)
	}
}

func TestDefaultShellAndIsShell(t *testing.T) {
	if len(DefaultShell()) == 0 || DefaultShell()[0] == "" {
		t.Fatal("no default shell")
	}
	for argv, want := range map[string]string{
		`C:\Windows\System32\cmd.exe`: "cmd", "/bin/bash": "ctrl-l", "pwsh.exe": "ctrl-l",
		"/usr/bin/python3": "", "": "", "/bin/sh": "",
	} {
		if got := IsShell([]string{argv}); got != want {
			t.Errorf("IsShell(%q) = %q, want %q", argv, got, want)
		}
	}
	var _ io.ReadWriter = (*PTY)(nil)
}
