package neovim

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/proc"
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
	return waitEventWithin(t, 10*time.Second, events, kind, ok)
}

// waitEventWithin is waitEvent with its own deadline, for events behind a process start.
func waitEventWithin(t *testing.T, d time.Duration, events <-chan Event, kind string, ok func(Event) bool) Event {
	t.Helper()
	timeout := time.After(d)
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

	if saved, err := s.Save(); err != nil || !saved {
		t.Fatalf("Save() = %v, %v", saved, err)
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

// TestCloseWaitsForChildProcesses: Close returns only once the processes Neovim started (a
// terminal's shell and, on Windows, its console host) have ended, so nothing keeps running in,
// or holds open, their working folder afterwards.
func TestCloseWaitsForChildProcesses(t *testing.T) {
	s := startReal(t, nil)
	if _, err := s.Terminal(); err != nil {
		t.Fatal(err)
	}
	var shell, nvim int
	if err := s.ExecLua(`return vim.fn.jobpid(vim.b.terminal_job_id)`, &shell); err != nil || shell <= 0 {
		t.Fatalf("shell pid %d, %v", shell, err)
	}
	if err := s.ExecLua(`return vim.fn.getpid()`, &nvim); err != nil {
		t.Fatal(err)
	}
	children := proc.Descendants(nvim)
	if runtime.GOOS == "windows" && !slices.Contains(children, shell) {
		t.Fatalf("Descendants(%d) = %v, missing the shell %d", nvim, children, shell)
	}
	_ = s.Close()
	for _, pid := range append(children, shell) {
		if proc.Alive(pid) {
			t.Errorf("process %d that Neovim started still runs after Close", pid)
		}
	}
}

// TestCloseWithAChildHoldingStdout: a program Neovim started that keeps Neovim's output pipe
// open (a daemonising clipboard tool, something a plugin spawns) must not make Close wait
// forever for the end of that pipe. Close ends it and returns (#19).
func TestCloseWithAChildHoldingStdout(t *testing.T) {
	s := startReal(t, nil)
	var holder int
	err := s.ExecLua(`local uv = vim.uv or vim.loop
local h, pid = uv.spawn(vim.v.progpath, {
  args = { "--headless", "--clean", "-n", "-c", "sleep 60", "-c", "qa!" },
  stdio = { nil, 1, 2 }, -- Neovim's own stdout: the RPC pipe to PyxForge
  detached = true,
}, function() end)
if h then h:unref() end
return pid or -1`, &holder)
	if err != nil || holder <= 0 {
		t.Fatalf("holder pid %d, %v", holder, err)
	}
	t.Cleanup(func() { _ = proc.Terminate(holder) })
	if !proc.Alive(holder) {
		t.Fatalf("the process holding Neovim's stdout (%d) ended before Close", holder)
	}
	done := make(chan struct{})
	start := time.Now()
	go func() { _ = s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		_ = proc.Terminate(holder) // lets Close finish, so the test does not hang
		<-done
		t.Fatal("Close did not return within 30 s while a child held Neovim's stdout")
	}
	t.Logf("Close took %v", time.Since(start).Round(time.Millisecond))
	if waitGone([]int{holder}, 2*time.Second); proc.Alive(holder) {
		t.Errorf("the process holding Neovim's stdout (%d) still runs after Close", holder)
	}
}

// TestFileChangedOnDisk: a file changed by another program while open (PXF-DEF-012). A buffer
// without unsaved changes reloads; one with unsaved changes keeps them, is reported as a
// conflict, and is not written by Save, SaveAll or CloseBuffer until Overwrite or Reload
// settles it; a deleted file is reported and its text kept.
func TestFileChangedOnDisk(t *testing.T) {
	events := make(chan Event, 256)
	s := startReal(t, events)
	file := filepath.Join(t.TempDir(), "notes.txt")
	later := time.Now().Add(time.Hour)
	onDisk := func(body string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		later = later.Add(time.Minute) // a time Neovim cannot have seen
		if err := os.Chtimes(file, later, later); err != nil {
			t.Fatal(err)
		}
	}
	text := func() string {
		var lines []string
		_ = s.ExecLua(`return vim.api.nvim_buf_get_lines(0, 0, -1, false)`, &lines)
		return strings.Join(lines, "\n")
	}
	conflict := func() Event {
		t.Helper()
		return waitEvent(t, events, "FileChanged", func(e Event) bool { return e.Message == "conflict" })
	}
	onDisk("first\n")
	if err := s.Open(file); err != nil {
		t.Fatal(err)
	}
	buf := waitEvent(t, events, "BufEnter", func(e Event) bool { return e.IsFile() }).Buffer

	// Clean buffer: the new text comes in.
	onDisk("second\n")
	if err := s.CheckFiles(); err != nil {
		t.Fatal(err)
	}
	if got := text(); got != "second" {
		t.Errorf("a clean buffer was not reloaded: %q", got)
	}

	// Unsaved changes: kept, and reported.
	s.Input("Omine<Esc>")
	waitEvent(t, events, "BufModifiedSet", func(e Event) bool { return e.Modified })
	onDisk("theirs\n")
	if err := s.CheckFiles(); err != nil {
		t.Fatal(err)
	}
	if e := conflict(); e.Buffer != buf || !e.Modified {
		t.Errorf("conflict event %+v", e)
	}
	if got := text(); got != "mine\nsecond" {
		t.Errorf("unsaved changes lost: %q", got)
	}
	if saved, err := s.Save(); err != nil || saved {
		t.Errorf("Save during a conflict = %v, %v; want false, nil", saved, err)
	}
	conflict()
	if left, err := s.SaveAll(); err != nil || len(left) != 1 {
		t.Errorf("SaveAll during a conflict = %v, %v", left, err)
	}
	conflict()
	if err := s.CloseBuffer(buf, true, false); !errors.Is(err, ErrNotSaved) {
		t.Errorf("CloseBuffer with save during a conflict: %v", err)
	}
	if data, _ := os.ReadFile(file); string(data) != "theirs\n" {
		t.Fatalf("the file on disk was written during the conflict: %q", data)
	}

	// Overwrite writes the buffer over the file.
	if err := s.Overwrite(buf); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(file); string(data) != "mine\nsecond\n" {
		t.Errorf("Overwrite wrote %q", data)
	}

	// Reload takes the file's text and drops the changes.
	s.Input("Oagain<Esc>")
	waitEvent(t, events, "BufModifiedSet", func(e Event) bool { return e.Modified })
	onDisk("from disk\n")
	_ = s.CheckFiles()
	conflict()
	if err := s.Reload(buf); err != nil {
		t.Fatal(err)
	}
	if m, _ := s.Modified(); m || text() != "from disk" {
		t.Errorf("after Reload: modified %v, text %q", m, text())
	}
	if saved, err := s.Save(); err != nil || !saved {
		t.Errorf("Save after Reload = %v, %v", saved, err)
	}

	// Deleted: reported, text kept.
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	_ = s.CheckFiles()
	waitEvent(t, events, "FileChanged", func(e Event) bool { return e.Message == "deleted" })
	if got := text(); got != "from disk" {
		t.Errorf("text after the file was deleted: %q", got)
	}
}

// TestKeepTranscript: a shell's output outlives it as a read-only buffer in the same window,
// and the exited terminal buffer goes.
func TestKeepTranscript(t *testing.T) {
	events := make(chan Event, 64)
	s := startReal(t, events)
	buf, err := s.Terminal()
	if err != nil {
		t.Fatal(err)
	}
	s.Input("echo kept-after-exit<CR>")
	waitGrid(t, s, "kept-after-exit")
	s.Input("exit 4<CR>")
	if e := waitEvent(t, events, "TermClose", nil); e.Buffer != buf || e.Status != 4 {
		t.Fatalf("TermClose %+v", e)
	}
	out, err := s.KeepTranscript(buf)
	if err != nil || out == 0 || out == buf {
		t.Fatalf("KeepTranscript = %d, %v", out, err)
	}
	var state struct {
		Shown, Modifiable, OldListed bool
		Text                         string
	}
	if err := s.ExecLua(`local out, old = ...
return { Shown = vim.api.nvim_get_current_buf() == out, Modifiable = vim.bo[out].modifiable,
  OldListed = vim.api.nvim_buf_is_valid(old) and vim.bo[old].buflisted,
  Text = table.concat(vim.api.nvim_buf_get_lines(out, 0, -1, false), "\n") }`, &state, out, buf); err != nil {
		t.Fatal(err)
	}
	if !state.Shown || state.Modifiable || state.OldListed || !strings.Contains(state.Text, "kept-after-exit") {
		t.Errorf("transcript: %+v", state)
	}
	// The exited terminal buffer goes a few seconds later, not as its job ends (Neovim crashed
	// on Windows deleting it then).
	deadline := time.Now().Add(10 * time.Second)
	for {
		var valid bool
		if err := s.ExecLua(`return vim.api.nvim_buf_is_valid(...)`, &valid, buf); err != nil {
			t.Fatal(err)
		}
		if !valid {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the exited terminal buffer was never deleted")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// PyxForge may still hold the old number for a moment (a restart on its way): showing and
	// closing by it reach the transcript instead of failing on a buffer that is gone.
	if err := s.Command("enew"); err != nil {
		t.Fatal(err)
	}
	if err := s.ShowTerminal(buf); err != nil {
		t.Fatalf("ShowTerminal by the old number: %v", err)
	}
	var current int
	if err := s.ExecLua(`return vim.api.nvim_get_current_buf()`, &current); err != nil || current != out {
		t.Errorf("showing by the old number shows buffer %d, want %d (%v)", current, out, err)
	}
	if err := s.CloseBuffer(buf, false, true); err != nil {
		t.Fatalf("CloseBuffer by the old number: %v", err)
	}
	var valid bool
	if err := s.ExecLua(`return vim.api.nvim_buf_is_valid(...)`, &valid, out); err != nil || valid {
		t.Errorf("the transcript outlived closing by the old number (%v)", err)
	}
	if err := s.CloseBuffer(9999, false, true); err != nil {
		t.Errorf("closing a buffer that is gone: %v", err)
	}
}

// TestRunInTerminal: a program, not the shell, runs in a terminal buffer, in its folder; it
// reads what is typed, its output shows, and its exit status is reported as a shell's is.
func TestRunInTerminal(t *testing.T) {
	events := make(chan Event, 64)
	s := startReal(t, events)
	dir := filepath.Join(t.TempDir(), "run-here")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	argv := []string{"sh", "-c", `read name; echo "hello $name in $(basename "$PWD")"; exit 3`}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd", "/v:on", "/c", `set /p name=& echo hello !name! in %CD%& exit /b 3`}
	}
	buf, err := s.RunInTerminal(argv, dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Input("PyxForge<CR>")
	waitGrid(t, s, "hello PyxForge in")
	if e := waitEvent(t, events, "TermClose", nil); e.Buffer != buf || e.Status != 3 {
		t.Fatalf("TermClose %+v, want buffer %d with status 3", e, buf)
	}
	waitGrid(t, s, "run-here") // it ran in its folder
	if _, err := s.RunInTerminal([]string{"no-such-program-pyxforge"}, dir); err == nil {
		t.Error("starting a program that does not exist reported no error")
	}
}

// TestExitStatusIsReported: on Windows, when Neovim ends abnormally its exit status reaches
// OnExit, so a crash is told from a quit in the Log (a terminal's Neovim was seen to end on CI
// with no reason given).
func TestExitStatusIsReported(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the exit status is read on Windows only")
	}
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	exited := make(chan error, 1)
	s, err := Start(context.Background(), Options{Dir: t.TempDir(), Width: 40, Height: 10, OnExit: func(err error) { exited <- err }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	p, err := os.FindProcess(s.pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Kill(); err != nil { // TerminateProcess: exit status 1
		t.Fatal(err)
	}
	select {
	case err := <-exited:
		if err == nil || !strings.Contains(err.Error(), "status 1 ") {
			t.Errorf("OnExit got %v, want the exit status", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("OnExit was not called")
	}
}

// TestTypeAtPrompt: lines are typed into a terminal's program at its prompt, each once the
// prompt shows again (how node inspect gets the editor's breakpoints, #63).
func TestTypeAtPrompt(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	s := startReal(t, nil)
	script := `const rl = require("readline").createInterface({ input: process.stdin, output: process.stdout });
rl.setPrompt("ask> "); rl.prompt();
rl.on("line", (l) => { if (l === "quit") process.exit(0); console.log("got " + l); rl.prompt(); });`
	buf, err := s.RunInTerminal([]string{node, "-e", script}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if n := s.TypeAtPrompt(buf, "ask>", []string{"one", "two"}, 15*time.Second); n != 2 {
		t.Fatalf("typed %d lines", n)
	}
	waitGrid(t, s, "got two")
	if g := s.Grid(); gridText(g, "got one") == 0 {
		t.Error("the first line did not arrive")
	}
	if n := s.TypeAtPrompt(buf, "no such prompt", []string{"x"}, 300*time.Millisecond); n != 0 {
		t.Errorf("typed %d lines without the prompt", n)
	}
}

func gridText(g Snapshot, want string) int {
	n := 0
	for r := range g.Rows {
		n += strings.Count(g.Line(r), want)
	}
	return n
}
