package editor

import (
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Keys translate Fyne's key events into Neovim's key notation (`:help key-notation`).
// Printable characters arrive through TypedRune; TypedKey only carries keys that produce no
// character; modified keys arrive as shortcuts.

var specialKeys = map[fyne.KeyName]string{
	fyne.KeyReturn: "CR", fyne.KeyEnter: "CR", fyne.KeyEscape: "Esc", fyne.KeyBackspace: "BS",
	fyne.KeyDelete: "Del", fyne.KeyTab: "Tab", fyne.KeyInsert: "Insert",
	fyne.KeyUp: "Up", fyne.KeyDown: "Down", fyne.KeyLeft: "Left", fyne.KeyRight: "Right",
	fyne.KeyHome: "Home", fyne.KeyEnd: "End", fyne.KeyPageUp: "PageUp", fyne.KeyPageDown: "PageDown",
	fyne.KeyF1: "F1", fyne.KeyF2: "F2", fyne.KeyF3: "F3", fyne.KeyF4: "F4", fyne.KeyF5: "F5", fyne.KeyF6: "F6",
	fyne.KeyF7: "F7", fyne.KeyF8: "F8", fyne.KeyF9: "F9", fyne.KeyF10: "F10", fyne.KeyF11: "F11", fyne.KeyF12: "F12",
	fyne.KeySpace: "Space",
}

// Keys that name a character in a shortcut, beyond letters and digits.
var punctuation = map[fyne.KeyName]string{
	fyne.KeyComma: ",", fyne.KeyPeriod: ".", fyne.KeySlash: "/", fyne.KeyBackslash: "Bslash",
	fyne.KeyMinus: "-", fyne.KeyEqual: "=", fyne.KeySemicolon: ";", fyne.KeyApostrophe: "'",
	fyne.KeyLeftBracket: "[", fyne.KeyRightBracket: "]", fyne.KeyBackTick: "`",
}

// runeKeys turns a typed character into key notation.
func runeKeys(r rune) string {
	switch r {
	case '<':
		return "<lt>"
	case '\x00':
		return ""
	}
	return string(r)
}

// specialKey turns a non-printing key into key notation; ok is false for printable keys,
// which arrive again through TypedRune.
func specialKey(k fyne.KeyName, shift bool) (string, bool) {
	name, ok := specialKeys[k]
	if !ok || k == fyne.KeySpace {
		return "", false
	}
	if shift {
		return "<S-" + name + ">", true
	}
	return "<" + name + ">", true
}

// shortcutKeys turns a shortcut into key notation. ok is false when the shortcut carries no
// key Neovim should see (AltGr characters on Windows, which arrive as runes).
func shortcutKeys(s fyne.Shortcut) (string, bool) {
	switch sc := s.(type) {
	case *fyne.ShortcutCopy:
		if sc.Secondary {
			return "<C-Insert>", true
		}
		return "<C-c>", true
	case *fyne.ShortcutPaste:
		if sc.Secondary {
			return "<S-Insert>", true
		}
		return "<C-v>", true
	case *fyne.ShortcutCut:
		if sc.Secondary {
			return "<S-Del>", true
		}
		return "<C-x>", true
	case *fyne.ShortcutSelectAll:
		return "<C-a>", true
	case *fyne.ShortcutUndo:
		return "<C-z>", true
	case *fyne.ShortcutRedo:
		return "<C-y>", true
	case *desktop.CustomShortcut:
		return customKeys(sc.Modifier, sc.KeyName)
	}
	return "", false
}

func customKeys(m fyne.KeyModifier, k fyne.KeyName) (string, bool) {
	// On Windows AltGr is reported as Ctrl+Alt, and the character it makes ({, [, \ on many
	// layouts) arrives separately as a rune. Sending both would type twice.
	if runtime.GOOS == "windows" && m&fyne.KeyModifierControl != 0 && m&fyne.KeyModifierAlt != 0 && m&fyne.KeyModifierSuper == 0 {
		return "", false
	}
	var key string
	switch {
	case len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z':
		key = strings.ToLower(string(k))
	case len(k) == 1 && k[0] >= '0' && k[0] <= '9':
		key = string(k)
	case punctuation[k] != "":
		key = punctuation[k]
	case specialKeys[k] != "":
		key = specialKeys[k]
	default:
		return "", false
	}
	var mods strings.Builder
	if m&fyne.KeyModifierControl != 0 {
		mods.WriteString("C-")
	}
	if m&fyne.KeyModifierAlt != 0 {
		mods.WriteString("M-")
	}
	if m&fyne.KeyModifierShift != 0 {
		mods.WriteString("S-")
	}
	if m&fyne.KeyModifierSuper != 0 {
		mods.WriteString("D-")
	}
	return "<" + mods.String() + key + ">", true
}
