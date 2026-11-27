package pty

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeMachine replaces the lookups for one test.
func fakeMachine(t *testing.T, paths map[string]string, env map[string]string, files ...string) {
	t.Helper()
	ol, og, oe, os_ := lookPath, getenv, exists, etcShells
	t.Cleanup(func() { lookPath, getenv, exists, etcShells = ol, og, oe, os_ })
	lookPath = func(name string) (string, error) {
		if p, ok := paths[name]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
	getenv = func(k string) string { return env[k] }
	have := map[string]bool{}
	for _, f := range files {
		have[f] = true
	}
	exists = func(p string) bool { return have[p] }
}

func names(ss []Shell) []string {
	var out []string
	for _, s := range ss {
		out = append(out, s.Name)
	}
	return out
}

// TestWindowsShells: PowerShell 7 first, then Windows PowerShell, the Command Prompt and Git
// Bash found beside Git (never WSL's System32 launcher); with only cmd.exe, cmd.exe.
func TestWindowsShells(t *testing.T) {
	git := filepath.Join("C:", "Program Files", "Git")
	fakeMachine(t, map[string]string{"pwsh": `C:\pwsh\pwsh.exe`, "powershell": `C:\WINDOWS\powershell.exe`,
		"git": filepath.Join(git, "cmd", "git.exe"), "bash": `C:\WINDOWS\System32\bash.exe`},
		map[string]string{"COMSPEC": `C:\WINDOWS\system32\cmd.exe`}, filepath.Join(git, "bin", "bash.exe"))
	got := windowsShells()
	want := []string{"PowerShell 7 (pwsh)", "Windows PowerShell", "Command Prompt (cmd.exe)", "Git Bash"}
	if len(got) != 4 || names(got)[0] != want[0] || names(got)[3] != want[3] || got[3].Argv[0] != filepath.Join(git, "bin", "bash.exe") {
		t.Fatalf("shells %v (%v)", names(got), got)
	}
	if got[0].Kind() != "pwsh" || got[1].Kind() != "powershell" || got[2].Kind() != "cmd" || got[3].Kind() != "bash" {
		t.Errorf("kinds %q %q %q %q", got[0].Kind(), got[1].Kind(), got[2].Kind(), got[3].Kind())
	}
	if ShellFor("", got).Kind() != "pwsh" || ShellFor(`C:\WINDOWS\system32\cmd.exe`, got).Kind() != "cmd" ||
		ShellFor(`D:\gone\fish.exe`, got).Kind() != "pwsh" {
		t.Error("ShellFor does not prefer pwsh, keep a choice, or fall back from a missing one")
	}

	// Git found as mingw64\bin\git.exe, two folders below its root.
	fakeMachine(t, map[string]string{"git": filepath.Join(git, "mingw64", "bin", "git.exe")},
		map[string]string{"COMSPEC": `C:\WINDOWS\system32\cmd.exe`}, filepath.Join(git, "bin", "bash.exe"))
	if got := windowsShells(); len(got) != 2 || got[1].Argv[0] != filepath.Join(git, "bin", "bash.exe") {
		t.Errorf("with Git's mingw64 git.exe on PATH: %v", got)
	}

	fakeMachine(t, nil, map[string]string{"COMSPEC": `C:\WINDOWS\system32\cmd.exe`})
	if got := windowsShells(); len(got) != 1 || got[0].Kind() != "cmd" {
		t.Errorf("with nothing but cmd.exe: %v", names(got))
	}
}

// TestUnixShells: $SHELL first, then /etc/shells without repeats, comments or missing programs.
func TestUnixShells(t *testing.T) {
	f := filepath.Join(t.TempDir(), "shells")
	os.WriteFile(f, []byte("# valid login shells\n/bin/sh\n/bin/bash\n/usr/bin/zsh\n/usr/bin/fish\n/bin/bash\n"), 0o644)
	fakeMachine(t, nil, map[string]string{"SHELL": "/usr/bin/zsh"}, "/bin/sh", "/bin/bash", "/usr/bin/zsh")
	etcShells = f
	got := unixShells()
	if len(got) != 3 || got[0].Argv[0] != "/usr/bin/zsh" || got[1].Argv[0] != "/bin/sh" || got[2].Argv[0] != "/bin/bash" {
		t.Fatalf("shells %v", names(got))
	}
	if got[0].Name != "zsh (/usr/bin/zsh)" {
		t.Errorf("name %q", got[0].Name)
	}
}

// TestShellsOnThisMachine: the real lookups find at least one shell that exists.
func TestShellsOnThisMachine(t *testing.T) {
	got := Shells()
	if len(got) == 0 {
		t.Fatal("no shell found")
	}
	if runtime.GOOS != "windows" {
		if _, err := os.Stat(got[0].Argv[0]); err != nil {
			t.Errorf("%v: %v", got[0], err)
		}
	}
}
