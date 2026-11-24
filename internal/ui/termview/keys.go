package termview

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"

	"github.com/obstinix/PyxForge/internal/vt"
)

// modCode is xterm's modifier parameter: 1 plus Shift 1, Alt 2, Ctrl 4.
func modCode(m fyne.KeyModifier) int {
	n := 1
	if m&fyne.KeyModifierShift != 0 {
		n++
	}
	if m&fyne.KeyModifierAlt != 0 {
		n += 2
	}
	if m&fyne.KeyModifierControl != 0 {
		n += 4
	}
	return n
}

// keyBytes is what a key that is no character sends, xterm's way, under the program's modes
// and with modifiers held; nil for a key that sends nothing.
func keyBytes(k fyne.KeyName, m fyne.KeyModifier, modes vt.Modes) []byte {
	mod := modCode(m)
	// cursor keys: ESC [ A, ESC O A in application mode, ESC [ 1;m A with modifiers
	cursor := func(final byte) []byte {
		switch {
		case mod > 1:
			return []byte("\x1b[1;" + itoa(mod) + string(final))
		case modes.AppCursor:
			return []byte{0x1b, 'O', final}
		}
		return []byte{0x1b, '[', final}
	}
	// keys sent as ESC [ n ~, or ESC [ n;m ~ with modifiers
	tilde := func(n int) []byte {
		if mod > 1 {
			return []byte("\x1b[" + itoa(n) + ";" + itoa(mod) + "~")
		}
		return []byte("\x1b[" + itoa(n) + "~")
	}
	// F1–F4: ESC O P, or ESC [ 1;m P with modifiers
	ss3 := func(final byte) []byte {
		if mod > 1 {
			return []byte("\x1b[1;" + itoa(mod) + string(final))
		}
		return []byte{0x1b, 'O', final}
	}
	switch k {
	case fyne.KeyReturn, fyne.KeyEnter:
		if m&fyne.KeyModifierAlt != 0 {
			return []byte("\x1b\r")
		}
		return []byte("\r")
	case fyne.KeyTab:
		if m&fyne.KeyModifierShift != 0 {
			return []byte("\x1b[Z")
		}
		return []byte("\t")
	case fyne.KeyBackspace:
		switch {
		case m&fyne.KeyModifierControl != 0:
			return []byte{0x08}
		case m&fyne.KeyModifierAlt != 0:
			return []byte{0x1b, 0x7f}
		}
		return []byte{0x7f}
	case fyne.KeyEscape:
		return []byte{0x1b}
	case fyne.KeyUp:
		return cursor('A')
	case fyne.KeyDown:
		return cursor('B')
	case fyne.KeyRight:
		return cursor('C')
	case fyne.KeyLeft:
		return cursor('D')
	case fyne.KeyHome:
		return cursor('H')
	case fyne.KeyEnd:
		return cursor('F')
	case fyne.KeyInsert:
		return tilde(2)
	case fyne.KeyDelete:
		return tilde(3)
	case fyne.KeyPageUp:
		return tilde(5)
	case fyne.KeyPageDown:
		return tilde(6)
	case fyne.KeyF1:
		return ss3('P')
	case fyne.KeyF2:
		return ss3('Q')
	case fyne.KeyF3:
		return ss3('R')
	case fyne.KeyF4:
		return ss3('S')
	case fyne.KeyF5:
		return tilde(15)
	case fyne.KeyF6:
		return tilde(17)
	case fyne.KeyF7:
		return tilde(18)
	case fyne.KeyF8:
		return tilde(19)
	case fyne.KeyF9:
		return tilde(20)
	case fyne.KeyF10:
		return tilde(21)
	case fyne.KeyF11:
		return tilde(23)
	case fyne.KeyF12:
		return tilde(24)
	}
	return nil
}

// ctrlBytes is what Ctrl with a printable key sends: the control character (Ctrl+A is 0x01,
// Ctrl+[ is ESC, Ctrl+Space and Ctrl+@ NUL), with ESC before it when Alt is held too. ok is
// false for a key with no control character.
func ctrlBytes(k fyne.KeyName, alt bool) ([]byte, bool) {
	var c byte
	switch {
	case len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z':
		c = k[0] - 'A' + 1
	default:
		switch k {
		case fyne.KeySpace, "@", "2":
			c = 0
		case fyne.KeyLeftBracket, "3":
			c = 0x1b
		case fyne.KeyBackslash, "4":
			c = 0x1c
		case fyne.KeyRightBracket, "5":
			c = 0x1d
		case "6":
			c = 0x1e
		case fyne.KeySlash, fyne.KeyMinus, "7":
			c = 0x1f
		case "8":
			c = 0x7f
		default:
			return nil, false
		}
	}
	if alt {
		return []byte{0x1b, c}, true
	}
	return []byte{c}, true
}

// altBytes is Alt with a printable key: ESC, then the key's character (lower case unless
// Shift is held). ok is false for a key with no character.
func altBytes(k fyne.KeyName, shift bool) ([]byte, bool) {
	if len(k) == 1 {
		s := string(k)
		if !shift {
			s = strings.ToLower(s)
		}
		return []byte("\x1b" + s), true
	}
	if k == fyne.KeySpace {
		return []byte("\x1b "), true
	}
	return nil, false
}

// pasteBytes is pasted text as typed: line ends as Enter, and inside the bracketed-paste
// markers when the program asked for them (with any ESC removed, so the text cannot end the
// paste early).
func pasteBytes(text string, bracketed bool) []byte {
	text = strings.ReplaceAll(text, "\r\n", "\r")
	text = strings.ReplaceAll(text, "\n", "\r")
	if bracketed {
		return []byte("\x1b[200~" + strings.ReplaceAll(text, "\x1b", "") + "\x1b[201~")
	}
	return []byte(text)
}

// mouseBytes is a mouse report: button 0–2 (left, middle, right), 3 release (in the old form),
// 64 and 65 the wheel; drag adds 32. x and y count from 0.
func mouseBytes(button, x, y int, release, sgr bool, m fyne.KeyModifier) []byte {
	if m&fyne.KeyModifierShift != 0 {
		button |= 4
	}
	if m&fyne.KeyModifierAlt != 0 {
		button |= 8
	}
	if m&fyne.KeyModifierControl != 0 {
		button |= 16
	}
	if sgr {
		final := "M"
		if release {
			final = "m"
		}
		return []byte("\x1b[<" + itoa(button) + ";" + itoa(x+1) + ";" + itoa(y+1) + final)
	}
	if release {
		button = 3 | button&^3
	}
	if x > 222 || y > 222 {
		return nil // the old form cannot say it
	}
	return []byte{0x1b, '[', 'M', byte(32 + button), byte(33 + x), byte(33 + y)}
}

func itoa(n int) string { return strconv.Itoa(n) }
