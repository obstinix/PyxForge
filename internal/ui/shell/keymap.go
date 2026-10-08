package shell

import (
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// ShellModifier is the only modifier set the shell binds: Ctrl+Shift (Cmd+Shift on macOS).
// Neovim owns every other key while the editor has focus (decision K1,
// docs/architecture/KEYMAP.md), so plain Ctrl, Alt and AltGr chords never reach the shell.
const ShellModifier = fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift

// IsShellChord reports whether a shortcut belongs to the shell. The Phase 3 editor widget
// forwards these to the window and sends everything else to Neovim.
func IsShellChord(s fyne.Shortcut) bool {
	c, ok := s.(*desktop.CustomShortcut)
	return ok && c.Modifier == ShellModifier
}

func chord(k fyne.KeyName) *desktop.CustomShortcut {
	return &desktop.CustomShortcut{KeyName: k, Modifier: ShellModifier}
}

// shortcutLabel renders a shortcut the way the platform writes it.
func shortcutLabel(sc *desktop.CustomShortcut) string {
	var parts []string
	m := sc.Modifier
	if m&fyne.KeyModifierControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if m&fyne.KeyModifierSuper != 0 {
		if runtime.GOOS == "darwin" {
			parts = append(parts, "Cmd")
		} else {
			parts = append(parts, "Super")
		}
	}
	if m&fyne.KeyModifierAlt != 0 {
		parts = append(parts, "Alt")
	}
	if m&fyne.KeyModifierShift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, string(sc.KeyName)), "+")
}
