package shell

import (
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// The keyboard policy is decision K2 (docs/architecture/KEYMAP.md), which replaced K1's
// "Neovim owns every key". PyxForge takes:
//
//   - Ctrl+Shift chords (Cmd+Shift on macOS), its own commands, wherever focus is;
//   - the conventional editor keys (Ctrl+N, O, S, W, F, H, P; Cmd on macOS), except in the
//     terminal, whose programs need them (Ctrl+W deletes a word in a shell, Ctrl+R searches
//     history), and in the editor when the user chose Neovim keys (prefNeovimKeys);
//   - Ctrl+Tab and Ctrl+Shift+Tab, wherever focus is.
//
// It never binds Alt or Ctrl+Alt: on Windows AltGr arrives as Ctrl+Alt, and code needs AltGr
// for { } [ ] \ | on many layouts. Every key PyxForge does not take reaches Neovim.

// ShellModifier is the modifier set of the shell's own chords: Ctrl+Shift (Cmd+Shift on macOS).
const ShellModifier = fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift

// prefNeovimKeys, when set, leaves the conventional plain-Ctrl keys to Neovim in the editor.
const prefNeovimKeys = "editor.neovimKeys"

// IsShellChord reports whether a shortcut is one of the shell's Ctrl+Shift chords.
func IsShellChord(s fyne.Shortcut) bool {
	c, ok := s.(*desktop.CustomShortcut)
	return ok && c.Modifier == ShellModifier
}

// chord is the Ctrl+Shift chord on k.
func chord(k fyne.KeyName) *desktop.CustomShortcut {
	return &desktop.CustomShortcut{KeyName: k, Modifier: ShellModifier}
}

// ctrl is the conventional editor key Ctrl+k (Cmd+k on macOS).
func ctrl(k fyne.KeyName) *desktop.CustomShortcut {
	return &desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierShortcutDefault}
}

// ctrlTab is Ctrl+Tab, or Ctrl+Shift+Tab with back: Ctrl on every platform, as Cmd+Tab
// switches applications on macOS.
func ctrlTab(back bool) *desktop.CustomShortcut {
	m := fyne.KeyModifierControl
	if back {
		m |= fyne.KeyModifierShift
	}
	return &desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: m}
}

func isPlainCtrl(c *desktop.CustomShortcut) bool {
	return c.Modifier == fyne.KeyModifierShortcutDefault
}

func isTabSwitch(c *desktop.CustomShortcut) bool {
	return c.KeyName == fyne.KeyTab && (c.Modifier == fyne.KeyModifierControl || c.Modifier == fyne.KeyModifierControl|fyne.KeyModifierShift)
}

// bound returns the custom shortcut s if PyxForge binds it.
func (s *Shell) bound(sc fyne.Shortcut) (*desktop.CustomShortcut, bool) {
	c, ok := sc.(*desktop.CustomShortcut)
	if !ok {
		return nil, false
	}
	_, ok = s.chordRuns[c.ShortcutName()]
	return c, ok
}

// takesFromEditor reports whether a shortcut typed in the editor runs a PyxForge command
// instead of reaching Neovim.
func (s *Shell) takesFromEditor(sc fyne.Shortcut) bool {
	c, ok := s.bound(sc)
	if !ok {
		return false
	}
	if isPlainCtrl(c) {
		return !s.app.Preferences().Bool(prefNeovimKeys)
	}
	return true
}

// takesFromTerminal reports whether a shortcut typed in the terminal runs a PyxForge command
// instead of reaching the shell: only the Ctrl+Shift chords and tab switching do.
func (s *Shell) takesFromTerminal(sc fyne.Shortcut) bool {
	c, ok := s.bound(sc)
	return ok && (c.Modifier == ShellModifier || isTabSwitch(c))
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
	name := string(sc.KeyName)
	if n, ok := keyLabels[sc.KeyName]; ok {
		name = n
	}
	return strings.Join(append(parts, name), "+")
}

// keyLabels names keys whose Fyne names are not what the keyboard says.
var keyLabels = map[fyne.KeyName]string{fyne.KeyPageDown: "PageDown", fyne.KeyPageUp: "PageUp"}

// toggleNeovimKeys switches the editor between PyxForge's conventional keys and Neovim's own
// for Ctrl+N, O, S, W, F, H and P, and says which is on.
func (s *Shell) toggleNeovimKeys() {
	p := s.app.Preferences()
	on := !p.Bool(prefNeovimKeys)
	p.SetBool(prefNeovimKeys, on)
	if on {
		s.Notify(notifications.Info, "Neovim keys in the editor",
			"Ctrl+N, O, S, W, F, H and P now go to Neovim while the editor has focus. The command palette still runs the commands.")
		return
	}
	s.Notify(notifications.Info, "Conventional keys in the editor",
		"Ctrl+N, O, S, W, F, H and P run PyxForge's commands again, also while the editor has focus.")
}
