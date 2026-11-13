package shell

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/ui/editor"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// The editor and the terminal each have a text size of their own (Settings > Type): code is
// read for hours and a terminal is glanced at, and people want them different. The sizes are
// kept in the preferences and apply at once, to the view that is open and to the next one.
const (
	prefEditorTextSize   = "type.editorSize"
	prefTerminalTextSize = "type.terminalSize"
)

// textSize is a view's saved size, or the default for one never set or out of range.
func (s *Shell) textSize(pref string) float32 {
	v := float32(s.app.Preferences().FloatWithFallback(pref, float64(theme.TextCode)))
	if v < editor.MinTextSize || v > editor.MaxTextSize {
		return theme.TextCode
	}
	return v
}

// setTextSize saves a view's size, within the limits, and applies it to the view if it is open.
func (s *Shell) setTextSize(pref string, size float32) float32 {
	size = max(editor.MinTextSize, min(editor.MaxTextSize, size))
	s.app.Preferences().SetFloat(pref, float64(size))
	switch pref {
	case prefEditorTextSize:
		if s.ed != nil && s.ed.view != nil {
			s.ed.view.SetTextSize(size)
		}
	case prefTerminalTextSize:
		if s.term != nil && s.term.view != nil {
			s.term.view.SetTextSize(size)
		}
	}
	return size
}

// textSizeSetting is a row in Settings: the area's name, a smaller and a larger button around
// the size, a reset, and a line of code drawn at that size.
func (s *Shell) textSizeSetting(label, pref, sample string) fyne.CanvasObject {
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
	smaller := widget.NewButton("−", step(-1))
	larger := widget.NewButton("+", step(1))
	reset := widget.NewButton("Reset", func() { show(s.setTextSize(pref, theme.TextCode)) })
	show(s.textSize(pref))
	name := kit.NewText(label, kit.Body, kit.Primary)
	middle := func(o fyne.CanvasObject) fyne.CanvasObject { // centred top to bottom, at the left
		return container.NewVBox(layout.NewSpacer(), o, layout.NewSpacer())
	}
	smaller.Importance, larger.Importance = widget.LowImportance, widget.LowImportance
	return container.NewVBox(
		container.NewHBox(container.NewGridWrap(fyne.NewSize(88, 32), middle(name)),
			smaller, container.NewGridWrap(fyne.NewSize(32, 32), container.NewCenter(value)), larger, reset),
		preview)
}
