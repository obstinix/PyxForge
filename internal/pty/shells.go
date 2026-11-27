package pty

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Shell is a shell the Terminal panel can start (decision D24).
type Shell struct {
	Name string   // as Settings > Terminal > Shell lists it
	Argv []string // the program and its arguments; Argv[0] identifies the shell
}

// Kind is the shell's family: "pwsh", "powershell", "cmd", "bash", "zsh", "fish", "sh" or the
// program's base name. Command blocks (D24) support the first five but cmd.
func (s Shell) Kind() string {
	if len(s.Argv) == 0 {
		return ""
	}
	name := strings.ToLower(s.Argv[0])
	if i := strings.LastIndexAny(name, `/\`); i >= 0 { // either separator: a saved path may be Windows'
		name = name[i+1:]
	}
	return strings.TrimSuffix(name, ".exe")
}

// lookups are how Shells finds programs; tests replace them.
var (
	lookPath  = exec.LookPath
	getenv    = os.Getenv
	exists    = func(p string) bool { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
	etcShells = "/etc/shells"
)

// Shells lists the shells found on this machine, the preferred one first: on Windows
// PowerShell 7 (pwsh), Windows PowerShell, the Command Prompt (%COMSPEC%) and Git Bash; on other
// systems $SHELL and then the other shells in /etc/shells that exist.
func Shells() []Shell {
	if runtime.GOOS == "windows" {
		return windowsShells()
	}
	return unixShells()
}

func windowsShells() []Shell {
	var out []Shell
	if p, err := lookPath("pwsh"); err == nil {
		out = append(out, Shell{"PowerShell 7 (pwsh)", []string{p, "-NoLogo"}})
	}
	if p, err := lookPath("powershell"); err == nil {
		out = append(out, Shell{"Windows PowerShell", []string{p, "-NoLogo"}})
	}
	cmd := getenv("COMSPEC")
	if cmd == "" {
		cmd = "cmd.exe"
	}
	out = append(out, Shell{"Command Prompt (cmd.exe)", []string{cmd}})
	// Git for Windows' bash, in the bin folder of the Git installation that holds git.exe
	// (cmd\git.exe, bin\git.exe or mingw64\bin\git.exe): never System32\bash.exe, which is
	// WSL's launcher.
	if git, err := lookPath("git"); err == nil {
		for dir, i := filepath.Dir(git), 0; i < 3; dir, i = filepath.Dir(dir), i+1 {
			if b := filepath.Join(dir, "bin", "bash.exe"); exists(b) {
				out = append(out, Shell{"Git Bash", []string{b, "--login", "-i"}})
				break
			}
		}
	}
	return out
}

func unixShells() []Shell {
	var out []Shell
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" || seen[p] || !exists(p) {
			return
		}
		seen[p] = true
		out = append(out, Shell{filepath.Base(p) + " (" + p + ")", []string{p}})
	}
	add(getenv("SHELL"))
	if f, err := os.Open(etcShells); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				add(line)
			}
		}
	}
	if len(out) == 0 {
		out = append(out, Shell{"sh (/bin/sh)", []string{"/bin/sh"}})
	}
	return out
}

// ShellFor is the shell to start for a saved choice: the listed shell whose program is choice,
// or, for "" or a program no longer found, the preferred one (the first listed).
func ShellFor(choice string, shells []Shell) Shell {
	if i := slices.IndexFunc(shells, func(s Shell) bool { return s.Argv[0] == choice }); i >= 0 {
		return shells[i]
	}
	if len(shells) == 0 {
		return Shell{"Default", DefaultShell()}
	}
	return shells[0]
}
