package neovim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/nvim"
)

// startConfigured starts the real Neovim with PyxForge's configuration, with every XDG folder
// in a temporary directory so the test never touches the user's own folders.
func startConfigured(t *testing.T, o Options) (*Session, string) {
	t.Helper()
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim is not installed")
	}
	init, err := nvim.Install(filepath.Join(t.TempDir(), "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	xdg := t.TempDir()
	for _, d := range []string{"config", "data", "state", "cache"} {
		o.Env = append(o.Env, "XDG_"+strings.ToUpper(d)+"_HOME="+filepath.Join(xdg, d))
	}
	o.Config, o.Width, o.Height = init, 80, 24
	if o.Dir == "" {
		o.Dir = t.TempDir()
	}
	s, err := Start(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, ok := s.WaitFlush(0, 5*time.Second); !ok {
		t.Fatal("no first redraw")
	}
	return s, xdg
}

func luaString(t *testing.T, s *Session, code string) string {
	t.Helper()
	var out string
	if err := s.ExecLua(code, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestConfigIsIsolatedAndLoadsUserFile(t *testing.T) {
	// user.lua lives in PyxForge's own Neovim config folder, which PyxForge never writes.
	s, xdg := startConfigured(t, Options{})
	cfg := luaString(t, s, `return vim.fn.stdpath("config")`)
	if filepath.Base(cfg) != nvim.AppName || !strings.HasPrefix(filepath.Clean(cfg), filepath.Clean(xdg)) {
		t.Fatalf("stdpath(config) = %q: not PyxForge's own folder", cfg)
	}
	_ = s.Close()

	userDir := filepath.Join(xdg, "config", nvim.AppName)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "user.lua"), []byte(`vim.g.pyxforge_user = "loaded"`), 0o644); err != nil {
		t.Fatal(err)
	}
	init, _ := nvim.Install(filepath.Join(t.TempDir(), "runtime"))
	env := []string{}
	for _, d := range []string{"config", "data", "state", "cache"} {
		env = append(env, "XDG_"+strings.ToUpper(d)+"_HOME="+filepath.Join(xdg, d))
	}
	s2, err := Start(context.Background(), Options{Config: init, Env: env, Dir: t.TempDir(), Width: 80, Height: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if got := luaString(t, s2, `return vim.g.pyxforge_user or ""`); got != "loaded" {
		t.Errorf("user.lua not loaded: %q", got)
	}
}

func TestCtrlSSavesAndTreesitterHighlights(t *testing.T) {
	events := make(chan Event, 64)
	s, _ := startConfigured(t, Options{OnEvent: func(e Event) {
		select {
		case events <- e:
		default:
		}
	}})
	file := filepath.Join(t.TempDir(), "kernel.c")
	if err := os.WriteFile(file, []byte("int kmain(void) { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Open(file); err != nil {
		t.Fatal(err)
	}
	waitGrid(t, s, "kmain")
	// Neovim ships a C parser, so C gets Tree-sitter highlighting without any setup.
	if got := luaString(t, s, `return tostring(vim.treesitter.highlighter.active[vim.api.nvim_get_current_buf()] ~= nil)`); got != "true" {
		t.Errorf("Tree-sitter is not highlighting C")
	}
	s.Input("O/* boot */<C-s><Esc>") // Ctrl+S in insert mode saves and stays in insert mode
	waitEvent(t, events, "BufWritePost", nil)
	if data, _ := os.ReadFile(file); !strings.HasPrefix(string(data), "/* boot */") {
		t.Errorf("Ctrl+S did not save: %q", data)
	}
}

func TestSystemClipboard(t *testing.T) {
	var mu sync.Mutex
	system := "from the system\n"
	var copied []string
	s, _ := startConfigured(t, Options{
		OnClipboardGet: func() string { mu.Lock(); defer mu.Unlock(); return system },
		OnClipboardSet: func(text string) { mu.Lock(); copied = append(copied, text); mu.Unlock() },
	})
	if !s.Clipboard() {
		t.Fatal("Neovim did not take PyxForge as its clipboard") // 0.9 included (PXF-DEF-029)
	}
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("first line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Open(file); err != nil {
		t.Fatal(err)
	}
	waitGrid(t, s, "first line")
	s.Input("p") // 'clipboard' is unnamedplus: p pastes the system clipboard
	waitGrid(t, s, "from the system")
	s.Input("ggyy") // and y copies to it
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(copied)
		got := ""
		if n > 0 {
			got = copied[n-1]
		}
		mu.Unlock()
		if got == "first line\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("yank did not reach the system clipboard: %q", copied)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestLanguageServers(t *testing.T) {
	events := make(chan Event, 256)
	s, _ := startConfigured(t, Options{OnEvent: func(e Event) {
		select {
		case events <- e:
		default:
		}
	}})
	dir := t.TempDir()

	// Server notices reach PyxForge instead of Neovim's message line.
	if err := s.ExecLua(`require("pyxforge.lsp").show_message(nil, { type = 3, message = "indexing" }, { client_id = -1 })`, nil); err != nil {
		t.Fatal(err)
	}
	if e := waitEvent(t, events, "LspMessage", nil); e.Message != "indexing" || e.Status != 3 || e.Name != "language server" {
		t.Errorf("server message event = %+v", e)
	}
	if msgs := luaString(t, s, `return vim.api.nvim_exec2("messages", { output = true }).output`); strings.Contains(msgs, "indexing") {
		t.Errorf("the notice also went to Neovim's messages: %q", msgs)
	}

	// A filetype whose server is not installed is reported once as missing.
	if _, err := exec.LookPath("taplo"); err != nil {
		toml := filepath.Join(dir, "pyxforge.toml")
		_ = os.WriteFile(toml, []byte("[project]\nname = \"x\"\n"), 0o644)
		if err := s.Open(toml); err != nil {
			t.Fatal(err)
		}
		waitEvent(t, events, "LspMissing", func(e Event) bool { return e.Name == "taplo" })
	}

	// clangd attaches to C and its diagnostics arrive like any other.
	if _, err := exec.LookPath("clangd"); err != nil {
		t.Skip("clangd is not installed")
	}
	src := filepath.Join(dir, "boot.c")
	if err := os.WriteFile(src, []byte("int main(void) { return undeclared_symbol; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Open(src); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, "LspAttach", func(e Event) bool { return e.Name == "clangd" })
	e := waitEvent(t, events, "DiagnosticChanged", func(e Event) bool {
		for _, d := range e.Diagnostics {
			if strings.Contains(d.Message, "undeclared_symbol") {
				return true
			}
		}
		return false
	})
	if e.Diagnostics[0].Severity != 1 {
		t.Errorf("clangd diagnostic = %+v", e.Diagnostics)
	}
}

// TestTerminalRestart runs the user's shell with Terminal, reports its exit status, and
// starts a fresh shell in the same Neovim, in terminal mode, ready for keys.
func TestTerminalRestart(t *testing.T) {
	events := make(chan Event, 256)
	s, _ := startConfigured(t, Options{OnEvent: func(e Event) {
		select {
		case events <- e:
		default:
		}
	}})
	waitOutput := func(want string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			// The output is a line of its own. (Counting the command line too fails where a long
			// prompt, such as CI's temp folder, wraps the command across rows.)
			g := s.Grid()
			for r := range g.Rows {
				if strings.TrimSpace(g.Line(r)) == want {
					return
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		g := s.Grid()
		for r := range g.Rows {
			t.Logf("grid %2d| %s", r, strings.TrimRight(g.Line(r), " "))
		}
		t.Fatalf("no output %q; mode %q", want, luaString(t, s, `return vim.api.nvim_get_mode().mode`))
	}

	first, err := s.Terminal()
	if err != nil {
		t.Fatal(err)
	}
	s.Input("echo one-shell<CR>")
	waitOutput("one-shell")
	s.Input("exit 3<CR>")
	if e := waitEvent(t, events, "TermClose", nil); e.Buffer != first || e.Status != 3 {
		t.Errorf("TermClose = %+v, want buffer %d status 3", e, first)
	}

	second, err := s.Terminal()
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("Terminal reused the finished buffer")
	}
	s.Input("echo two-shell<CR>")
	waitOutput("two-shell")
	if got := luaString(t, s, fmt.Sprintf(`return tostring(vim.api.nvim_buf_is_valid(%d))`, first)); got != "false" {
		t.Errorf("the finished terminal's buffer is still loaded (valid: %s)", got)
	}
}

// TestBuildDiagnosticsInTheEditor sends build diagnostics to Neovim: they appear in an open
// buffer and in a file opened afterwards, and the next build replaces them.
func TestBuildDiagnosticsInTheEditor(t *testing.T) {
	events := make(chan Event, 256)
	s, _ := startConfigured(t, Options{OnEvent: func(e Event) {
		select {
		case events <- e:
		default:
		}
	}})
	dir := t.TempDir()
	open := filepath.Join(dir, "boot.asm")
	later := filepath.Join(dir, "my os", "kernel.c")
	_ = os.MkdirAll(filepath.Dir(later), 0o755)
	for _, p := range []string{open, later} {
		if err := os.WriteFile(p, []byte("line one\nline two\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Open(open); err != nil {
		t.Fatal(err)
	}
	err := s.SetBuildDiagnostics([]BuildDiagnostic{
		{File: open, Line: 1, Col: 4, Severity: 1, Message: "instruction expected", Source: "build: boot"},
		{File: later, Line: 0, Col: 0, Severity: 2, Message: "unused variable", Source: "build: kernel"},
	})
	if err != nil {
		t.Fatal(err)
	}
	e := waitEvent(t, events, "DiagnosticChanged", func(e Event) bool {
		return len(e.Diagnostics) == 1 && e.Diagnostics[0].Message == "instruction expected"
	})
	if d := e.Diagnostics[0]; d.Line != 1 || d.Col != 4 || d.Severity != 1 || d.Source != "build: boot" {
		t.Errorf("diagnostic in the open buffer: %+v", d)
	}
	if err := s.Open(later); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, events, "DiagnosticChanged", func(e Event) bool {
		return len(e.Diagnostics) == 1 && e.Diagnostics[0].Message == "unused variable" && e.Diagnostics[0].Severity == 2
	})

	// The next build replaces them all.
	if err := s.SetBuildDiagnostics(nil); err != nil {
		t.Fatal(err)
	}
	n := luaString(t, s, `return tostring(#vim.diagnostic.get(nil, { namespace = require("pyxforge.build").ns }))`)
	if n != "0" {
		t.Errorf("%s build diagnostics left after an empty build", n)
	}
}

// TestSavingKeepsEveryByte: writing a file that was not edited leaves it byte for byte as it
// was, whatever its line endings, byte-order mark, encoding or final line break, and a binary
// file opens read-only in binary mode, so even a forced write changes nothing.
func TestSavingKeepsEveryByte(t *testing.T) {
	s, _ := startConfigured(t, Options{})
	dir := t.TempDir()
	latin1 := []byte("caf\xe9 cr\xe8me\n")
	all := make([]byte, 0, 1024)
	for i := 0; i < 4; i++ {
		for b := 0; b < 256; b++ {
			all = append(all, byte(b))
		}
	}
	for _, tc := range []struct {
		name   string
		data   []byte
		binary bool
	}{
		{"utf8.txt", []byte("print('héllo')\nsecond\n"), false},
		{"crlf.txt", []byte("line one\r\nline two\r\n"), false},
		{"no-final-newline.txt", []byte("first\nlast line"), false},
		{"bom.txt", append([]byte{0xEF, 0xBB, 0xBF}, "café\n"...), false},
		{"utf16le.txt", []byte{0xFF, 0xFE, 'U', 0, 'T', 0, 'F', 0, '\r', 0, '\n', 0}, false},
		{"utf16be.txt", []byte{0xFE, 0xFF, 0, 'h', 0, 'i', 0, '\n'}, false},
		{"latin1.txt", latin1, false},
		{"binary.bin", all, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, tc.name)
			if err := os.WriteFile(p, tc.data, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := s.Open(p); err != nil {
				t.Fatal(err)
			}
			got := luaString(t, s, `return tostring(vim.bo.binary) .. " " .. tostring(vim.bo.readonly)`)
			if want := fmt.Sprintf("%v %v", tc.binary, tc.binary); got != want {
				t.Errorf("binary readonly = %s, want %s", got, want)
			}
			if tc.binary {
				if err := s.Command("write"); err == nil {
					t.Error("a plain :w wrote a binary file")
				}
				if err := s.Command("write!"); err != nil {
					t.Fatal(err)
				}
			} else if err := s.Command("write"); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(tc.data) {
				at := 0
				for at < min(len(after), len(tc.data)) && after[at] == tc.data[at] {
					at++
				}
				t.Errorf("saving changed the file: %d bytes, was %d; first difference at byte %d", len(after), len(tc.data), at)
			}
		})
	}
}

// TestShadaPerWorkspace: a session keeps its history in the ShaDa file it is given, written when
// it closes: a file opened with one is among its old files next time, and not among another's
// (PXF-DEF-001).
func TestShadaPerWorkspace(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.shada"), filepath.Join(dir, "b.shada")
	file := filepath.Join(t.TempDir(), "kernel.c")
	if err := os.WriteFile(file, []byte("int main(void) { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := startConfigured(t, Options{Shada: a})
	if err := s.Open(file); err != nil {
		t.Fatal(err)
	}
	waitGrid(t, s, "int main")
	_ = s.Close()
	if _, err := os.Stat(a); err != nil {
		t.Fatalf("closing the session did not write its ShaDa file: %v", err)
	}
	remembers := func(shada string) bool {
		t.Helper()
		s, _ := startConfigured(t, Options{Shada: shada})
		var old []string
		if err := s.ExecLua(`return vim.v.oldfiles`, &old); err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(old, func(f string) bool { return strings.EqualFold(filepath.Clean(f), filepath.Clean(file)) })
	}
	if !remembers(a) {
		t.Error("the file is not among the old files of its own ShaDa")
	}
	if remembers(b) {
		t.Error("another workspace's ShaDa lists the file")
	}
}

// TestBreakpoints: F9's breakpoints are set and cleared on the cursor's line and reported, move
// with their line when lines are added above, come back from a kept list (for files opened
// later too), survive reading the file again, and are refused for a buffer that is no file.
func TestBreakpoints(t *testing.T) {
	reported := make(chan []Breakpoint, 16)
	s, _ := startConfigured(t, Options{OnBreakpoints: func(l []Breakpoint) { reported <- l }})
	dir := t.TempDir()
	main := filepath.Join(dir, "main.c")
	other := filepath.Join(dir, "other.c")
	for _, f := range []string{main, other} {
		if err := os.WriteFile(f, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Open(main); err != nil {
		t.Fatal(err)
	}
	waitGrid(t, s, "e")
	same := func(got []Breakpoint, want map[string][]int) bool {
		if len(got) != len(want) {
			return false
		}
		for _, b := range got {
			var lines []int
			for f, ls := range want {
				if strings.EqualFold(filepath.Clean(b.File), filepath.Clean(f)) {
					lines = ls
				}
			}
			if !slices.Equal(b.Lines, lines) {
				return false
			}
		}
		return true
	}
	list := func() []Breakpoint {
		t.Helper()
		l, err := s.Breakpoints()
		if err != nil {
			t.Fatal(err)
		}
		return l
	}

	s.Input("3G<F9>") // the key, as typed in the editor
	waitGrid(t, s, "●")
	select {
	case l := <-reported:
		if !same(l, map[string][]int{main: {3}}) {
			t.Errorf("reported %+v", l)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("setting a breakpoint was not reported")
	}
	s.Input("ggOnew<Esc>") // a line above it
	deadline := time.Now().Add(5 * time.Second)
	for !same(list(), map[string][]int{main: {4}}) {
		if time.Now().After(deadline) {
			t.Fatalf("after adding a line above: %+v", list())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := s.ExecLua(`vim.api.nvim_win_set_cursor(0, { 4, 0 })`, nil); err != nil {
		t.Fatal(err)
	}
	if set, err := s.ToggleBreakpoint(); set || err != nil || len(list()) != 0 {
		t.Errorf("clearing it: set %v, %v, left %+v", set, err, list())
	}

	// A kept list: the open file gets its breakpoints now, the other when it is opened.
	if err := s.RestoreBreakpoints([]Breakpoint{{File: main, Lines: []int{2, 5}}, {File: other, Lines: []int{1}}}); err != nil {
		t.Fatal(err)
	}
	if !same(list(), map[string][]int{main: {2, 5}, other: {1}}) {
		t.Errorf("restored: %+v", list())
	}
	if err := s.ExecLua(`vim.cmd("silent! write!") vim.cmd.edit({ bang = true })`, nil); err != nil {
		t.Fatal(err)
	}
	if !same(list(), map[string][]int{main: {2, 5}, other: {1}}) {
		t.Errorf("after reading main.c again: %+v", list())
	}
	// Deleting a breakpoint's line removes it, rather than moving it onto the next line; the one
	// below moves up with its line. The last line is deleted the same way (#21).
	waitList := func(what string, want map[string][]int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !same(list(), want) {
			if time.Now().After(deadline) {
				t.Fatalf("%s: %+v", what, list())
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	for len(reported) > 0 {
		<-reported
	}
	s.Input("2Gdd")
	select { // PyxForge is told at once, not at the next save (listing first would prune it)
	case l := <-reported:
		if !same(l, map[string][]int{main: {4}, other: {1}}) {
			t.Errorf("reported after deleting line 2: %+v", l)
		}
	case <-time.After(5 * time.Second):
		t.Error("deleting a breakpoint's line was not reported")
	}
	waitList("after deleting line 2", map[string][]int{main: {4}, other: {1}})
	s.Input("Gdd") // the last line, "e": the breakpoint on "d" stays, now on the last line
	waitGrid(t, s, "d")
	waitList("after deleting the last line", map[string][]int{main: {4}, other: {1}})
	s.Input("Gdd") // "d" itself, as the last line
	waitList("after deleting the breakpoint's line at the end", map[string][]int{other: {1}})
	if err := s.Open(other); err != nil {
		t.Fatal(err)
	}
	var signs int
	if err := s.ExecLua(`return #vim.api.nvim_buf_get_extmarks(0, require("pyxforge.breakpoints").ns, 0, -1, {})`, &signs); err != nil || signs != 1 {
		t.Errorf("other.c opened with %d breakpoints shown, %v", signs, err)
	}

	if err := s.ExecLua(`vim.cmd.enew()`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ToggleBreakpoint(); !errors.Is(err, ErrNotAFile) {
		t.Errorf("a breakpoint in an untitled buffer: %v", err)
	}
}
