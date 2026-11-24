// Package pty runs a program on a pseudo-terminal: ConPTY on Windows, a Unix pty elsewhere
// (D17). The program's output, escape sequences included, is read from the PTY; what is
// written to it is the program's keyboard input. Decoding the output is internal/vt's work.
package pty

import (
	"errors"
	"os"
	"runtime"
	"strings"
)

// Options say what to run and on how large a terminal.
type Options struct {
	Argv []string // the program and its arguments; the program is looked up in PATH
	Dir  string   // its working folder; "" is PyxForge's
	// Env is the environment, os.Environ() when nil. TERM and COLORTERM are set on Unix.
	Env        []string
	Cols, Rows int // the size, at least 2 by 1
}

// ErrClosed is returned by Resize and Write once the PTY is closed.
var ErrClosed = errors.New("the terminal is closed")

// DefaultShell is the user's shell: %COMSPEC% (cmd.exe) on Windows, $SHELL elsewhere, else
// /bin/sh. It is what Neovim's 'shell' defaulted to, so the Terminal panel keeps its shell.
func DefaultShell() []string {
	if runtime.GOOS == "windows" {
		if c := os.Getenv("COMSPEC"); c != "" {
			return []string{c}
		}
		return []string{"cmd.exe"}
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return []string{sh}
	}
	return []string{"/bin/sh"}
}

// IsShell reports whether argv runs an interactive shell that clears its screen on Ctrl+L
// (most) or on cls (cmd.exe), and which: "cmd", "ctrl-l" or "". sh and dash are not: dash
// (Debian's and Ubuntu's sh) has no line editor and would take Ctrl+L as part of a command.
func IsShell(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	name := strings.ToLower(argv[0])
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, ".exe")
	switch name {
	case "cmd":
		return "cmd"
	case "bash", "zsh", "fish", "ksh", "pwsh", "powershell", "nu":
		return "ctrl-l"
	}
	return ""
}

func size(o Options) (int, int) {
	return max(o.Cols, 2), max(o.Rows, 1)
}
