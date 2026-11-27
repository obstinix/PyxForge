package kit

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Dropdown chooses one of a few options, like Fyne's Select, drawn as a soft Default button
// with a chevron. A click, Space, Enter or Return opens the list; Up and Down change the choice
// without opening it.
type Dropdown struct {
	widget.BaseWidget
	Options   []string
	Selected  string
	Label     string // its accessible name (the setting row's title)
	OnChanged func(string)

	hovered, focused, disabled bool
}

var (
	_ fyne.Tappable     = (*Dropdown)(nil)
	_ fyne.Focusable    = (*Dropdown)(nil)
	_ desktop.Hoverable = (*Dropdown)(nil)
	_ fyne.Accessible   = (*Dropdown)(nil)
)

// NewDropdown returns a dropdown with options and a selection.
func NewDropdown(options []string, selected string, changed func(string)) *Dropdown {
	d := &Dropdown{Options: options, Selected: selected, OnChanged: changed}
	d.ExtendBaseWidget(d)
	return d
}

// SetSelected chooses an option and reports the change.
func (d *Dropdown) SetSelected(s string) {
	if s == d.Selected || !slices.Contains(d.Options, s) {
		return
	}
	d.Selected = s
	d.Refresh()
	if d.OnChanged != nil {
		d.OnChanged(s)
	}
}

// Disable makes the dropdown ignore taps and keys until Enable.
func (d *Dropdown) Disable() { d.disabled = true; d.Refresh() }

// Enable makes it work again.
func (d *Dropdown) Enable() { d.disabled = false; d.Refresh() }

// Menu is the list a click opens; for tests.
func (d *Dropdown) Menu() *fyne.Menu {
	var items []*fyne.MenuItem
	for _, o := range d.Options {
		it := fyne.NewMenuItem(o, func() { d.SetSelected(o) })
		it.Checked = o == d.Selected
		items = append(items, it)
	}
	return fyne.NewMenu("", items...)
}

func (d *Dropdown) Tapped(*fyne.PointEvent) { d.open() }

func (d *Dropdown) open() {
	if d.disabled {
		return
	}
	c := fyne.CurrentApp().Driver().CanvasForObject(d)
	if c == nil {
		return
	}
	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(d).Add(fyne.NewPos(0, d.Size().Height))
	pop := widget.NewPopUpMenu(d.Menu(), c)
	pop.ShowAtPosition(at)
	pop.Resize(fyne.NewSize(max(d.Size().Width, pop.MinSize().Width), pop.MinSize().Height))
}

// step moves the choice by delta options.
func (d *Dropdown) step(delta int) {
	if d.disabled || len(d.Options) == 0 {
		return
	}
	i := slices.Index(d.Options, d.Selected) + delta
	d.SetSelected(d.Options[max(0, min(i, len(d.Options)-1))])
}

func (d *Dropdown) MouseIn(*desktop.MouseEvent)    { d.hovered = true; d.Refresh() }
func (d *Dropdown) MouseMoved(*desktop.MouseEvent) {}
func (d *Dropdown) MouseOut()                      { d.hovered = false; d.Refresh() }
func (d *Dropdown) FocusGained()                   { d.focused = true; d.Refresh() }
func (d *Dropdown) FocusLost()                     { d.focused = false; d.Refresh() }
func (d *Dropdown) TypedRune(r rune) {
	if r == ' ' {
		d.open()
	}
}
func (d *Dropdown) TypedKey(e *fyne.KeyEvent) {
	switch e.Name {
	case fyne.KeyReturn, fyne.KeyEnter:
		d.open()
	case fyne.KeyUp:
		d.step(-1)
	case fyne.KeyDown:
		d.step(1)
	}
}

// AccessibilityLabel names the dropdown and its choice.
func (d *Dropdown) AccessibilityLabel() string { return d.Label + ": " + d.Selected }

// AccessibilityRole is a button's: it opens a list.
func (d *Dropdown) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

func (d *Dropdown) CreateRenderer() fyne.WidgetRenderer {
	r := &dropdownRenderer{d: d, bg: canvas.NewRectangle(nil), ring: canvas.NewRectangle(nil),
		text: canvas.NewText("", nil), chevron: canvas.NewImageFromResource(nil)}
	r.chevron.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

type dropdownRenderer struct {
	d       *Dropdown
	bg      *canvas.Rectangle
	ring    *canvas.Rectangle
	text    *canvas.Text
	chevron *canvas.Image
}

func (r *dropdownRenderer) Refresh() {
	t := theme.Current()
	st := StateNormal
	switch {
	case r.d.disabled:
		st = StateDisabled
	case r.d.hovered:
		st = StateHover
	}
	fill, text, outline := ButtonColors(t, ButtonDefault, st)
	r.bg.FillColor, r.bg.StrokeColor, r.bg.StrokeWidth, r.bg.CornerRadius = fill, outline, 1, theme.RadiusControl
	styleRing(r.ring, t)
	r.ring.Hidden = !r.d.focused
	r.text.Text, r.text.Color, r.text.TextSize, r.text.FontSource = r.d.Selected, text, theme.TextBody, theme.FontUI
	r.chevron.Resource = icons.Get(icons.ChevronDown, text)
	r.bg.Refresh()
	r.ring.Refresh()
	r.text.Refresh()
	r.chevron.Refresh()
	r.Layout(r.d.Size())
}

func (r *dropdownRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	r.ring.Resize(s)
	th := r.text.MinSize().Height
	r.text.Move(fyne.NewPos(theme.Space3, (s.Height-th)/2))
	r.text.Resize(fyne.NewSize(max(s.Width-2*theme.Space3-theme.IconSize-theme.Space1, 0), th))
	r.chevron.Move(fyne.NewPos(s.Width-theme.Space2-theme.IconSize, (s.Height-theme.IconSize)/2))
	r.chevron.Resize(fyne.NewSquareSize(theme.IconSize))
}

// MinSize fits the longest option, so the control does not change width with the choice.
func (r *dropdownRenderer) MinSize() fyne.Size {
	w := float32(0)
	for _, o := range r.d.Options {
		w = max(w, fyne.MeasureText(o, theme.TextBody, fyne.TextStyle{}).Width)
	}
	return fyne.NewSize(w+2*theme.Space3+theme.IconSize+theme.Space1, ButtonHeight)
}

func (r *dropdownRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.text, r.chevron, r.ring}
}
func (r *dropdownRenderer) Destroy() {}
