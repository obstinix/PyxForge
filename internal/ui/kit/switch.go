package kit

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// ReduceMotion reports whether the user asked for less motion; the shell sets it from its
// preference. With it, controls change state at once instead of animating.
var ReduceMotion = func() bool { return false }

// Switch sizes: a 36×20 pill with a 16 px knob.
const (
	switchW    float32 = 36
	switchH    float32 = 20
	switchKnob float32 = 16
)

// Switch is a pill toggle (Fyne has none): the accent fill when on, the Sunken surface with a
// hairline when off. A click, Space, Enter or Return toggles it; the knob slides over 120 ms,
// or jumps under Reduce Motion.
type Switch struct {
	widget.BaseWidget
	On        bool
	Label     string // its accessible name (the setting row's title)
	OnChanged func(bool)

	hovered, focused, disabled bool
	knobAt                     float32 // 0 off … 1 on, while it moves
	anim                       *fyne.Animation
}

var (
	_ fyne.Tappable     = (*Switch)(nil)
	_ fyne.Focusable    = (*Switch)(nil)
	_ desktop.Hoverable = (*Switch)(nil)
	_ fyne.Accessible   = (*Switch)(nil)
)

// NewSwitch returns a switch.
func NewSwitch(on bool, changed func(bool)) *Switch {
	s := &Switch{On: on, OnChanged: changed}
	if on {
		s.knobAt = 1
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetOn sets the state without running OnChanged.
func (s *Switch) SetOn(on bool) {
	if s.On == on {
		return
	}
	s.On = on
	s.move()
}

// Disable makes the switch ignore taps and keys until Enable.
func (s *Switch) Disable() { s.disabled = true; s.Refresh() }

// Enable makes a disabled switch work again.
func (s *Switch) Enable() { s.disabled = false; s.Refresh() }

// Disabled reports whether the switch is disabled.
func (s *Switch) Disabled() bool { return s.disabled }

// Toggle flips the switch and reports the change.
func (s *Switch) Toggle() {
	if s.disabled {
		return
	}
	s.On = !s.On
	s.move()
	if s.OnChanged != nil {
		s.OnChanged(s.On)
	}
}

// move slides the knob to the current state.
func (s *Switch) move() {
	to := float32(0)
	if s.On {
		to = 1
	}
	if s.anim != nil {
		s.anim.Stop()
		s.anim = nil
	}
	if ReduceMotion() || fyne.CurrentApp() == nil {
		s.knobAt = to
		s.Refresh()
		return
	}
	from := s.knobAt
	s.anim = fyne.NewAnimation(theme.TransitionMillis*time.Millisecond, func(p float32) {
		s.knobAt = from + (to-from)*p
		s.Refresh()
	})
	s.anim.Curve = fyne.AnimationEaseOut
	s.anim.Start()
}

// KnobPosition is where the knob is, 0 (off) to 1 (on); for tests.
func (s *Switch) KnobPosition() float32 { return s.knobAt }

func (s *Switch) Tapped(*fyne.PointEvent)        { s.Toggle() }
func (s *Switch) MouseIn(*desktop.MouseEvent)    { s.hovered = true; s.Refresh() }
func (s *Switch) MouseMoved(*desktop.MouseEvent) {}
func (s *Switch) MouseOut()                      { s.hovered = false; s.Refresh() }
func (s *Switch) FocusGained()                   { s.focused = true; s.Refresh() }
func (s *Switch) FocusLost()                     { s.focused = false; s.Refresh() }
func (s *Switch) TypedRune(r rune) {
	if r == ' ' {
		s.Toggle()
	}
}
func (s *Switch) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter {
		s.Toggle()
	}
}

// AccessibilityLabel names the switch and says whether it is on.
func (s *Switch) AccessibilityLabel() string {
	state := " (off)"
	if s.On {
		state = " (on)"
	}
	return s.Label + state
}

// AccessibilityRole is a button's: Fyne 2.8 has only button, container, link and text roles;
// the label says whether it is on.
func (s *Switch) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

// SwitchColors are the track fill, track outline and knob colour of a switch, from the tokens.
func SwitchColors(t theme.Tokens, on, disabled bool) (track, outline, knob color.NRGBA) {
	track, outline, knob = t.Surface.Sunken, t.Border.Strong, t.Text.Secondary
	if on {
		track, outline, knob = t.Accent.Primary, color.NRGBA{}, t.Accent.OnPrimary
	}
	if disabled {
		fade := func(c color.NRGBA) color.NRGBA { c.A = uint8(float64(c.A) * disabledAlpha); return c }
		track, outline, knob = fade(track), fade(outline), fade(knob)
	}
	return
}

func (s *Switch) CreateRenderer() fyne.WidgetRenderer {
	r := &switchRenderer{s: s, track: canvas.NewRectangle(nil), knob: canvas.NewCircle(nil), ring: canvas.NewRectangle(nil)}
	r.Refresh()
	return r
}

type switchRenderer struct {
	s     *Switch
	track *canvas.Rectangle
	knob  *canvas.Circle
	ring  *canvas.Rectangle
}

func (r *switchRenderer) Refresh() {
	t := theme.Current()
	track, outline, knob := SwitchColors(t, r.s.On, r.s.disabled)
	if r.s.hovered && !r.s.disabled && !r.s.On {
		track = theme.Over(t.State.Hover, track)
	}
	r.track.FillColor, r.track.StrokeColor = track, outline
	r.track.StrokeWidth = 0
	if outline.A > 0 {
		r.track.StrokeWidth = 1
	}
	r.track.CornerRadius = switchH / 2
	r.knob.FillColor = knob
	styleRing(r.ring, t)
	r.ring.CornerRadius = switchH/2 + 2
	r.ring.Hidden = !r.s.focused
	r.Layout(r.s.Size())
	r.track.Refresh()
	r.knob.Refresh()
	r.ring.Refresh()
}

func (r *switchRenderer) Layout(sz fyne.Size) {
	x, y := (sz.Width-switchW)/2, (sz.Height-switchH)/2
	r.track.Move(fyne.NewPos(x, y))
	r.track.Resize(fyne.NewSize(switchW, switchH))
	inset := (switchH - switchKnob) / 2
	kx := x + inset + r.s.knobAt*(switchW-switchKnob-2*inset)
	r.knob.Move(fyne.NewPos(kx, y+inset))
	r.knob.Resize(fyne.NewSquareSize(switchKnob))
	r.ring.Move(fyne.NewPos(x-2, y-2))
	r.ring.Resize(fyne.NewSize(switchW+4, switchH+4))
}

func (r *switchRenderer) MinSize() fyne.Size { return fyne.NewSize(switchW+4, switchH+4) }
func (r *switchRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.knob, r.ring}
}
func (r *switchRenderer) Destroy() {}
