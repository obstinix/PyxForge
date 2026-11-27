package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// SearchField is a one-line search box: a magnifier at the left, a placeholder, a clear button
// at the right while there is text, and Esc to clear. Its well is the Sunken surface with the
// control radius (the Fyne theme maps the entry's background and radius to them).
type SearchField struct {
	widget.Entry
	clear *IconButton
	// OnEscapeEmpty runs when Esc is pressed with nothing typed: the owner closes or leaves.
	OnEscapeEmpty func()
}

// NewSearchField returns a search box with a placeholder.
func NewSearchField(placeholder string, changed func(string)) *SearchField {
	f := &SearchField{}
	f.ExtendBaseWidget(f)
	f.PlaceHolder = placeholder
	f.clear = NewIconButton(icons.X, "Clear search", func() { f.SetText("") })
	f.clear.Side = 24
	f.clear.Hide()
	f.ActionItem = f.clear
	f.OnChanged = func(s string) {
		f.syncClear()
		if changed != nil {
			changed(s)
		}
	}
	f.refreshIcon()
	return f
}

// SetText replaces the text and shows or hides the clear button.
func (f *SearchField) SetText(s string) {
	f.Entry.SetText(s)
	f.syncClear()
}

func (f *SearchField) syncClear() {
	if f.Text == "" {
		f.clear.Hide()
	} else {
		f.clear.Show()
	}
}

// ClearButton is the clear button; for tests.
func (f *SearchField) ClearButton() *IconButton { return f.clear }

func (f *SearchField) refreshIcon() { f.Icon = icons.Get(icons.Search, theme.Current().Text.Tertiary) }

// Refresh recolours the magnifier for the current theme.
func (f *SearchField) Refresh() {
	f.refreshIcon()
	f.Entry.Refresh()
}

// TypedKey clears the text on Esc, or reports an Esc on an empty field.
func (f *SearchField) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyEscape {
		if f.Text != "" {
			f.SetText("")
		} else if f.OnEscapeEmpty != nil {
			f.OnEscapeEmpty()
		}
		return
	}
	f.Entry.TypedKey(e)
}
