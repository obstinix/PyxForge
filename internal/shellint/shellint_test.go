package shellint

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/term"
)

// candidates are the shells this machine has, by kind. On Windows they are the ones the
// Terminal offers (never WSL's System32 bash launcher).
func candidates() map[string]pty.Shell {
	found := map[string]pty.Shell{}
	if runtime.GOOS == "windows" {
		for _, sh := range pty.Shells() {
			found[sh.Kind()] = sh
		}
		return found
	}
	for _, name := range []string{"bash", "zsh", "fish", "pwsh"} {
		if p, err := exec.LookPath(name); err == nil {
			found[name] = pty.Shell{Name: name, Argv: []string{p}}
		}
	}
	return found
}

// failing is a command that exits with a known status in each shell.
func failing(kind string) (string, int) {
	switch kind {
	case "fish":
		return "false", 1
	case "pwsh", "powershell":
		if runtime.GOOS == "windows" {
			return "cmd /c exit 3", 3
		}
		return "sh -c 'exit 3'", 3
	}
	return "(exit 3)", 3
}

func waitFor(t *testing.T, s *term.Session, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s; screen:\n%s", what, s.Screen.String())
		}
		select {
		case <-s.Ended():
			t.Fatalf("the shell ended waiting for %s; screen:\n%s", what, s.Screen.String())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func lastBlock(s *term.Session, cmd string) (term.Block, bool) {
	bs := term.Blocks(s.Screen)
	for i := len(bs) - 1; i >= 0; i-- {
		if bs[i].Command == cmd && !bs[i].Running {
			return bs[i], true
		}
	}
	return term.Block{}, false
}

func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	fa, ea := os.Stat(a)
	fb, eb := os.Stat(b)
	return ea == nil && eb == nil && os.SameFile(fa, fb)
}

// TestIntegratedShells: every integrated shell on this machine, started as the Terminal starts
// it, marks a command that succeeds and one that fails as blocks with their command, exit
// status and folder. A shell the machine lacks is skipped with the reason.
func TestIntegratedShells(t *testing.T) {
	found := candidates()
	for _, kind := range []string{"bash", "zsh", "fish", "pwsh", "powershell"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "powershell" && runtime.GOOS != "windows" {
				t.Skip("Windows PowerShell exists only on Windows")
			}
			sh, ok := found[kind]
			if !ok {
				t.Skipf("%s is not installed on this machine", kind)
			}
			cache, work, home := t.TempDir(), t.TempDir(), t.TempDir()
			l, err := Prepare(sh, cache)
			if err != nil {
				t.Fatal(err)
			}
			// A fresh home, so the test sees the integration and not the machine's own rc files;
			// fish gets a prompt of the user's own, which the integration must keep.
			config := filepath.Join(home, ".config")
			if kind == "fish" {
				fns := filepath.Join(config, "fish", "functions")
				os.MkdirAll(fns, 0o755)
				os.WriteFile(filepath.Join(fns, "fish_prompt.fish"), []byte("function fish_prompt\n    echo -n 'myfish> '\nend\n"), 0o644)
			}
			env := append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "XDG_CONFIG_HOME="+config)
			env = append(env, l.Env...)
			s, err := term.Start(pty.Options{Argv: l.Argv, Dir: work, Env: env, Cols: 100, Rows: 30}, term.Hooks{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { s.Close(); <-s.Ended() }()
			waitFor(t, s, "the first prompt", func() bool { return s.Screen.MarkSeq() > 0 })
			s.Write([]byte("echo pyxforge\r"))
			waitFor(t, s, "echo's block", func() bool { _, ok := lastBlock(s, "echo pyxforge"); return ok })
			b, _ := lastBlock(s, "echo pyxforge")
			if b.Exit != 0 || !strings.Contains(term.OutputText(s.Screen, b), "pyxforge") {
				t.Errorf("echo: exit %d, output %q", b.Exit, term.OutputText(s.Screen, b))
			}
			if kind == "fish" && !strings.Contains(s.Screen.String(), "myfish> echo pyxforge") {
				t.Errorf("the user's own fish prompt is gone:\n%s", s.Screen.String())
			}
			if !sameDir(b.Cwd, work) {
				t.Errorf("echo ran in %q, want %q", b.Cwd, work)
			}
			cmd, want := failing(kind)
			s.Write([]byte(cmd + "\r"))
			waitFor(t, s, "the failing command's block", func() bool { _, ok := lastBlock(s, cmd); return ok })
			if b, _ := lastBlock(s, cmd); b.Exit != want || !b.Failed() {
				t.Errorf("%s: exit %d, want %d", cmd, b.Exit, want)
			}
		})
	}
}

// TestPrepare: each shell gets the integration its own way, the user's arguments kept, and
// cmd.exe gets none.
func TestPrepare(t *testing.T) {
	dir := t.TempDir()
	l, err := Prepare(pty.Shell{Argv: []string{"/bin/bash", "-l"}}, dir)
	if err != nil || len(l.Argv) != 4 || l.Argv[1] != "--rcfile" || l.Argv[3] != "-i" {
		t.Errorf("bash: %v %v", l.Argv, err)
	}
	if data, _ := os.ReadFile(l.Argv[2]); !strings.Contains(string(data), "~/.bashrc") {
		t.Error("the bash wrapper does not load ~/.bashrc")
	}
	l, _ = Prepare(pty.Shell{Argv: []string{"zsh"}}, dir)
	if !strings.HasPrefix(l.Env[0], "ZDOTDIR=") {
		t.Errorf("zsh env %v", l.Env)
	}
	for _, f := range []string{".zshenv", ".zprofile", ".zshrc"} {
		if _, err := os.Stat(filepath.Join(dir, "zsh", f)); err != nil {
			t.Errorf("zsh wrapper %s: %v", f, err)
		}
	}
	l, _ = Prepare(pty.Shell{Argv: []string{"fish"}}, dir)
	if l.Argv[1] != "--init-command" || !strings.HasPrefix(l.Argv[2], "source '") {
		t.Errorf("fish %v", l.Argv)
	}
	l, _ = Prepare(pty.Shell{Argv: []string{`C:\pwsh\pwsh.exe`, "-NoLogo"}}, dir)
	if strings.Join(l.Argv[:4], " ") != `C:\pwsh\pwsh.exe -NoLogo -NoExit -Command` || !strings.HasPrefix(l.Argv[4], ". '") {
		t.Errorf("pwsh %v", l.Argv)
	}
	if _, err := Prepare(pty.Shell{Argv: []string{`C:\WINDOWS\system32\cmd.exe`}}, dir); err != ErrUnsupported {
		t.Errorf("cmd.exe: %v", err)
	}
	if quotePS(`C:\it's`) != `'C:\it''s'` || quoteFish(`/a b/it's`) != `'/a b/it\'s'` {
		t.Error("quoting")
	}
}
