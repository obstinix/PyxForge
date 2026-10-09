package editor

import (
	"runtime"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func TestRuneKeys(t *testing.T) {
	for r, want := range map[rune]string{'a': "a", 'Z': "Z", '<': "<lt>", '{': "{", 'é': "é", 0: ""} {
		if got := runeKeys(r); got != want {
			t.Errorf("runeKeys(%q) = %q, want %q", r, got, want)
		}
	}
}

func TestSpecialKeys(t *testing.T) {
	for _, c := range []struct {
		k     fyne.KeyName
		shift bool
		want  string
		ok    bool
	}{
		{fyne.KeyReturn, false, "<CR>", true},
		{fyne.KeyEscape, false, "<Esc>", true},
		{fyne.KeyTab, true, "<S-Tab>", true},
		{fyne.KeyUp, false, "<Up>", true},
		{fyne.KeyF5, false, "<F5>", true},
		{fyne.KeyA, false, "", false},     // arrives as a rune
		{fyne.KeySpace, false, "", false}, // arrives as a rune
	} {
		got, ok := specialKey(c.k, c.shift)
		if got != c.want || ok != c.ok {
			t.Errorf("specialKey(%s, shift=%v) = %q %v", c.k, c.shift, got, ok)
		}
	}
}

func TestShortcutKeys(t *testing.T) {
	ctrl, alt, shift := fyne.KeyModifierControl, fyne.KeyModifierAlt, fyne.KeyModifierShift
	for _, c := range []struct {
		s    fyne.Shortcut
		want string
	}{
		{&fyne.ShortcutCopy{}, "<C-c>"},
		{&fyne.ShortcutPaste{}, "<C-v>"},
		{&fyne.ShortcutPaste{Secondary: true}, "<S-Insert>"},
		{&fyne.ShortcutUndo{}, "<C-z>"},
		{&desktop.CustomShortcut{KeyName: fyne.KeyW, Modifier: ctrl}, "<C-w>"},
		{&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: ctrl}, "<C-f>"},
		{&desktop.CustomShortcut{KeyName: fyne.KeyRightBracket, Modifier: ctrl}, "<C-]>"},
		{&desktop.CustomShortcut{KeyName: fyne.KeyBackslash, Modifier: ctrl}, "<C-Bslash>"},
		{&desktop.CustomShortcut{KeyName: fyne.KeyX, Modifier: alt}, "<M-x>"},
		{&desktop.CustomShortcut{KeyName: fyne.KeyUp, Modifier: ctrl | shift}, "<C-S-Up>"},
	} {
		if got, ok := shortcutKeys(c.s); !ok || got != c.want {
			t.Errorf("shortcutKeys(%#v) = %q %v, want %q", c.s, got, ok, c.want)
		}
	}
	altGr := &desktop.CustomShortcut{KeyName: fyne.KeyB, Modifier: ctrl | alt}
	got, ok := shortcutKeys(altGr)
	if runtime.GOOS == "windows" && ok {
		t.Errorf("AltGr+B on Windows sent %q; the brace arrives as a rune", got)
	}
	if runtime.GOOS != "windows" && (!ok || got != "<C-M-b>") {
		t.Errorf("Ctrl+Alt+B = %q %v", got, ok)
	}
}
