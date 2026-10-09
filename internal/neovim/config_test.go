package neovim

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		t.Skip("Neovim older than 0.10 cannot use PyxForge as its clipboard")
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
