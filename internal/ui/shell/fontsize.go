package shell

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// The editor and the terminal each have a text size of their own (Settings > Type): code is
// read for hours and a terminal is glanced at, and people want them different. The sizes are
// kept in the preferences and apply at once, to the view that is open and to the next one.
const (
	prefEditorTextSize   = "type.editorSize"
	prefTerminalTextSize = "type.terminalSize"
	// minTextSize and maxTextSize bound the editor's and the terminal's text sizes.
	minTextSize float32 = 9
	maxTextSize float32 = 28
)

// textSize is a view's saved size, or the default for one never set or out of range.
func (s *Shell) textSize(pref string) float32 {
	v := float32(s.app.Preferences().FloatWithFallback(pref, float64(theme.TextCode)))
	if v < minTextSize || v > maxTextSize {
		return theme.TextCode
	}
	return v
}

// setTextSize saves a view's size, within the limits, and applies it to the view if it is open.
func (s *Shell) setTextSize(pref string, size float32) float32 {
	size = max(minTextSize, min(maxTextSize, size))
	s.app.Preferences().SetFloat(pref, float64(size))
	switch pref {
	case prefEditorTextSize:
		if s.nat != nil {
			for _, d := range s.nat.docs {
				d.ed.SetTextSize(size)
			}
		}
	case prefTerminalTextSize:
		if s.term != nil && s.term.view != nil {
			s.term.view.SetTextSize(size)
		}
	}
	return size
}

// textSizeControl is a text-size row's control in Settings: a smaller and a larger button around
// the size, a reset, and a line of code drawn at that size.
func (s *Shell) textSizeControl(pref, sample string) fyne.CanvasObject {
	value := kit.NewText("", kit.Mono, kit.Primary)
	preview := kit.NewText(sample, kit.Mono, kit.Secondary)
	show := func(size float32) {
		value.SetText(fmt.Sprintf("%g", size))
		preview.TextSize = size
		preview.Refresh()
	}
	step := func(by float32) func() {
		return func() { show(s.setTextSize(pref, s.textSize(pref)+by)) }
	}
	smaller := kit.NewButton("−", step(-1))
	larger := kit.NewButton("+", step(1))
	reset := kit.NewButton("Reset", func() { show(s.setTextSize(pref, theme.TextCode)) })
	smaller.Variant, larger.Variant = kit.ButtonGhost, kit.ButtonGhost
	show(s.textSize(pref))
	return container.NewVBox(
		container.NewHBox(layout.NewSpacer(), smaller, container.NewGridWrap(fyne.NewSize(32, 32), container.NewCenter(value)), larger, reset),
		container.NewHBox(layout.NewSpacer(), preview))
}
