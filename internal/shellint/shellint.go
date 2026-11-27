// Package shellint gives the shells PyxForge starts its shell integration (D24): a script
// that marks prompts and commands with OSC 133 and the folder with OSC 7, so the Terminal
// panel can show command blocks. The scripts are embedded and written to PyxForge's cache;
// each shell gets them through a wrapper it is started with (bash --rcfile, zsh ZDOTDIR, fish
// --init-command, PowerShell -Command), which loads the user's own configuration first. The
// user's rc files are never changed.
package shellint

import (
	"bytes"
	"embed"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/obstinix/PyxForge/internal/pty"
)

//go:embed scripts
var scripts embed.FS

// ErrUnsupported is returned for a shell that cannot be integrated (cmd.exe, sh, nu).
var ErrUnsupported = errors.New("command blocks need PowerShell, bash, zsh or fish")

// Supported reports whether a shell can be integrated.
func Supported(sh pty.Shell) bool {
	switch sh.Kind() {
	case "bash", "zsh", "fish", "pwsh", "powershell":
		return true
	}
	return false
}

// Launch is how to start a shell with the integration: its arguments and environment
// additions (KEY=value).
type Launch struct {
	Argv []string
	Env  []string
}

// Prepare writes the integration for sh into dir (PyxForge's cache) and returns how to start
// it. A shell started with arguments of its own beyond the integrated ones keeps them first.
func Prepare(sh pty.Shell, dir string) (Launch, error) {
	if len(sh.Argv) == 0 || !Supported(sh) {
		return Launch{}, ErrUnsupported
	}
	exe, args := sh.Argv[0], sh.Argv[1:]
	switch sh.Kind() {
	case "bash":
		rc, err := write(dir, "bash.sh", "bashrc")
		if err != nil {
			return Launch{}, err
		}
		return Launch{Argv: append([]string{exe, "--rcfile", rc}, append(without(args, "-l", "--login"), "-i")...)}, nil
	case "zsh":
		zdir := filepath.Join(dir, "zsh")
		for _, f := range []string{"zshenv", "zprofile", "zshrc"} {
			if _, err := write(zdir, f, "."+f); err != nil {
				return Launch{}, err
			}
		}
		user := os.Getenv("ZDOTDIR")
		if user == "" {
			user, _ = os.UserHomeDir()
		}
		return Launch{Argv: append([]string{exe}, args...), Env: []string{"ZDOTDIR=" + zdir, "PYXFORGE_USER_ZDOTDIR=" + user}}, nil
	case "fish":
		f, err := write(dir, "fish.fish", "pyxforge.fish")
		if err != nil {
			return Launch{}, err
		}
		return Launch{Argv: append([]string{exe}, append(args, "--init-command", "source "+quoteFish(f))...)}, nil
	default: // pwsh, powershell
		f, err := write(dir, "pwsh.ps1", "pyxforge.ps1")
		if err != nil {
			return Launch{}, err
		}
		return Launch{Argv: append([]string{exe}, append(without(args, "-NoExit"), "-NoExit", "-Command", ". "+quotePS(f))...)}, nil
	}
}

// write copies an embedded script to dir/name, unless it is there already, and returns its path.
func write(dir, script, name string) (string, error) {
	data, err := scripts.ReadFile("scripts/" + script)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	if old, err := os.ReadFile(p); err == nil && bytes.Equal(old, data) {
		return p, nil
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

func without(args []string, drop ...string) []string {
	var out []string
	for _, a := range args {
		keep := true
		for _, d := range drop {
			if strings.EqualFold(a, d) {
				keep = false
			}
		}
		if keep {
			out = append(out, a)
		}
	}
	return out
}

// quotePS quotes a path for PowerShell: single quotes, doubled inside.
func quotePS(p string) string { return "'" + strings.ReplaceAll(p, "'", "''") + "'" }

// quoteFish quotes a path for fish: single quotes, with \ and ' escaped.
func quoteFish(p string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(p) + "'"
}

// Dir is where the integration is written: PyxForge's folder in the user's cache.
func Dir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "PyxForge", "shell-integration")
}
