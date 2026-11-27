package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Variant is a Button's kind.
type Variant int

const (
	// ButtonDefault is a soft raised button: the Raised surface with a hairline.
	ButtonDefault Variant = iota
	// ButtonCTA is the one primary action of a view: an accent fill (A1 allows one or two
	// accented things per screen, and this is one of them).
	ButtonCTA
	// ButtonGhost has no fill until the pointer is on it: toolbar and inline actions.
	ButtonGhost
	// ButtonDanger is an action that destroys or discards: error-coloured text.
	ButtonDanger
	// ButtonIcon is a square ghost button showing only its icon; Text is its tooltip and
	// accessible name.
	ButtonIcon
)

// ButtonState is what the pointer and keyboard are doing to a button.
type ButtonState int

const (
	StateNormal ButtonState = iota
	StateHover
	StatePressed
	StateDisabled
)

// Button heights (Obsidian-style controls on the 4 px grid).
const (
	ButtonHeight        float32 = 32
	ButtonHeightCompact float32 = 28
)

// disabledAlpha is how much of a disabled button's colours are drawn.
const disabledAlpha = 0.4

// ButtonColors are the fill, text and outline of a variant in a state, from the tokens. The
// contrast tests read them, so what is checked is what is drawn.
func ButtonColors(t theme.Tokens, v Variant, st ButtonState) (fill, text, outline color.NRGBA) {
	clear := color.NRGBA{}
	switch v {
	case ButtonCTA:
		fill, text = t.Accent.Primary, t.Accent.OnPrimary
		switch st {
		case StateHover:
			fill = t.Accent.Hover
		case StatePressed:
			// The theme's pressed wash over the accent (lighter on dark themes, darker on light
			// ones) keeps the label at 4.5:1, which Accent.Active does not with Crimson on dark.
			fill = theme.Over(t.State.Pressed, t.Accent.Primary)
		}
	case ButtonGhost, ButtonIcon:
		fill, text = clear, t.Text.Secondary
		switch st {
		case StateHover:
			fill, text = t.State.Hover, t.Text.Primary
		case StatePressed:
			fill, text = t.State.Pressed, t.Text.Primary
		}
	default: // Default and Danger: Raised; hover a wash up, press one surface step down
		fill, text, outline = t.Surface.Raised, t.Text.Primary, t.Border.Hairline
		if v == ButtonDanger {
			text = t.Status.Error
		}
		switch {
		case st == StateHover && v == ButtonDanger:
			// A lighter fill would take error text under 4.5:1: the outline says it instead.
			outline = t.Status.Error
		case st == StateHover:
			fill, outline = theme.Over(t.State.Hover, t.Surface.Raised), t.Border.Strong
		case st == StatePressed:
			fill = t.Surface.Base
		}
	}
	if st == StateDisabled {
		fade := func(c color.NRGBA) color.NRGBA { c.A = uint8(float64(c.A) * disabledAlpha); return c }
		fill, text, outline = fade(fill), fade(text), fade(outline)
	}
	return fill, text, outline
}

// Button is PyxForge's push button: focusable, activated by a click, Space, Enter or Return,
// with a 2 px focus ring for keyboard focus. It replaces Fyne's widget.Button in the shell.
type Button struct {
	widget.BaseWidget
	Text     string
	Icon     icons.Name // a leading icon, or "" for none
	Variant  Variant
	Compact  bool // 28 px high instead of 32
	OnTapped func()

	hovered, pressed, focused, disabled bool
}

var (
	_ fyne.Tappable     = (*Button)(nil)
	_ fyne.Focusable    = (*Button)(nil)
	_ desktop.Hoverable = (*Button)(nil)
	_ desktop.Mouseable = (*Button)(nil)
	_ fyne.Accessible   = (*Button)(nil)
)

// NewButton returns a Default button.
func NewButton(text string, tapped func()) *Button {
	b := &Button{Text: text, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

// NewButtonWithIcon returns a Default button with a leading icon.
func NewButtonWithIcon(text string, icon icons.Name, tapped func()) *Button {
	b := NewButton(text, tapped)
	b.Icon = icon
	return b
}

// NewCTA returns the primary-action button.
func NewCTA(text string, tapped func()) *Button {
	b := NewButton(text, tapped)
	b.Variant = ButtonCTA
	return b
}

// SetText changes the label.
func (b *Button) SetText(s string) { b.Text = s; b.Refresh() }

// SetVariant changes the kind.
func (b *Button) SetVariant(v Variant) { b.Variant = v; b.Refresh() }

// Disable greys the button out; taps and keys do nothing until Enable.
func (b *Button) Disable() { b.disabled = true; b.Refresh() }

// Enable makes the button work again.
func (b *Button) Enable() { b.disabled = false; b.Refresh() }

// Disabled reports whether the button is disabled.
func (b *Button) Disabled() bool { return b.disabled }

func (b *Button) state() ButtonState {
	switch {
	case b.disabled:
		return StateDisabled
	case b.pressed:
		return StatePressed
	case b.hovered:
		return StateHover
	}
	return StateNormal
}

func (b *Button) Tapped(*fyne.PointEvent) {
	if active != nil {
		active.leave(b)
	}
	if !b.disabled && b.OnTapped != nil {
		b.OnTapped()
	}
}

func (b *Button) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
	if active != nil && b.Variant == ButtonIcon && b.Text != "" {
		active.hover(b, b.Text)
	}
}
func (b *Button) MouseMoved(*desktop.MouseEvent) {}
func (b *Button) MouseOut() {
	b.hovered, b.pressed = false, false
	b.Refresh()
	if active != nil {
		active.leave(b)
	}
}
func (b *Button) MouseDown(*desktop.MouseEvent) { b.pressed = true; b.Refresh() }
func (b *Button) MouseUp(*desktop.MouseEvent)   { b.pressed = false; b.Refresh() }

func (b *Button) FocusGained() { b.focused = true; b.Refresh() }
func (b *Button) FocusLost()   { b.focused = false; b.Refresh() }
func (b *Button) TypedRune(r rune) {
	if r == ' ' {
		b.Tapped(nil)
	}
}
func (b *Button) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		b.Tapped(nil)
	}
}

// AccessibilityLabel is the button's text, saying when it is unavailable.
func (b *Button) AccessibilityLabel() string {
	if b.disabled {
		return b.Text + " (unavailable)"
	}
	return b.Text
}

// AccessibilityRole is a button's.
func (b *Button) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

func (b *Button) CreateRenderer() fyne.WidgetRenderer {
	r := &buttonRenderer{b: b, bg: canvas.NewRectangle(nil), ring: canvas.NewRectangle(nil),
		icon: canvas.NewImageFromResource(nil), label: canvas.NewText("", nil)}
	r.icon.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

type buttonRenderer struct {
	b     *Button
	bg    *canvas.Rectangle
	ring  *canvas.Rectangle
	icon  *canvas.Image
	label *canvas.Text
}

func (r *buttonRenderer) height() float32 {
	if r.b.Compact {
		return ButtonHeightCompact
	}
	return ButtonHeight
}

func (r *buttonRenderer) Refresh() {
	t := theme.Current()
	b := r.b
	fill, text, outline := ButtonColors(t, b.Variant, b.state())
	r.bg.FillColor, r.bg.StrokeColor, r.bg.CornerRadius = fill, outline, theme.RadiusControl
	r.bg.StrokeWidth = 0
	if outline.A > 0 {
		r.bg.StrokeWidth = 1
	}
	styleRing(r.ring, t)
	r.ring.Hidden = !b.focused
	r.label.Text = b.Text
	if b.Variant == ButtonIcon {
		r.label.Text = ""
	}
	r.label.Color, r.label.TextSize = text, theme.TextBody
	r.label.FontSource = theme.FontUI
	if b.Variant == ButtonCTA {
		r.label.FontSource = theme.FontUIStrong
	}
	r.icon.Hidden = b.Icon == ""
	if b.Icon != "" {
		r.icon.Resource = icons.Get(b.Icon, text)
	}
	r.bg.Refresh()
	r.ring.Refresh()
	r.label.Refresh()
	r.icon.Refresh()
	r.Layout(b.Size())
}

func (r *buttonRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	r.ring.Resize(s)
	lw := r.label.MinSize().Width
	x := (s.Width - r.contentWidth()) / 2
	if r.b.Icon != "" {
		r.icon.Move(fyne.NewPos(x, (s.Height-theme.IconSize)/2))
		r.icon.Resize(fyne.NewSquareSize(theme.IconSize))
		x += theme.IconSize
		if r.label.Text != "" {
			x += buttonIconGap
		}
	}
	lh := r.label.MinSize().Height
	r.label.Move(fyne.NewPos(x, (s.Height-lh)/2))
	r.label.Resize(fyne.NewSize(lw, lh))
}

const buttonIconGap float32 = 6

func (r *buttonRenderer) contentWidth() float32 {
	w := float32(0)
	if r.label.Text != "" {
		w = r.label.MinSize().Width
	}
	if r.b.Icon != "" {
		w += theme.IconSize
		if r.label.Text != "" {
			w += buttonIconGap
		}
	}
	return w
}

func (r *buttonRenderer) MinSize() fyne.Size {
	h := r.height()
	if r.b.Variant == ButtonIcon {
		return fyne.NewSquareSize(h)
	}
	return fyne.NewSize(r.contentWidth()+2*theme.Space3, h)
}

func (r *buttonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.icon, r.label, r.ring}
}

func (r *buttonRenderer) Destroy() {}
