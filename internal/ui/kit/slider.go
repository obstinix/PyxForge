package kit

import (
	"math"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Slider sets a number in a range: its label on the left and the value on the right (in mono),
// the track under them. Dragging or clicking the track sets it; with keyboard focus the arrow
// keys move one step and Shift with an arrow ten steps.
type Slider struct {
	widget.BaseWidget
	Label          string
	Min, Max, Step float64
	Value          float64
	Decimals       int // digits after the point in the value shown
	OnChanged      func(float64)

	hovered, focused, disabled bool
}

var (
	_ fyne.Tappable     = (*Slider)(nil)
	_ fyne.Draggable    = (*Slider)(nil)
	_ fyne.Focusable    = (*Slider)(nil)
	_ desktop.Hoverable = (*Slider)(nil)
	_ fyne.Accessible   = (*Slider)(nil)
)

// NewSlider returns a slider.
func NewSlider(label string, min, max, step, value float64, changed func(float64)) *Slider {
	s := &Slider{Label: label, Min: min, Max: max, Step: step, Value: value, OnChanged: changed}
	if step > 0 && step < 1 {
		s.Decimals = int(math.Ceil(-math.Log10(step)))
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetValue sets the value, kept in range and on a step, and reports a change.
func (s *Slider) SetValue(v float64) {
	if s.Step > 0 {
		v = s.Min + math.Round((v-s.Min)/s.Step)*s.Step
	}
	v = math.Max(s.Min, math.Min(v, s.Max))
	if v == s.Value {
		return
	}
	s.Value = v
	s.Refresh()
	if s.OnChanged != nil {
		s.OnChanged(v)
	}
}

// Disable makes the slider ignore input until Enable.
func (s *Slider) Disable() { s.disabled = true; s.Refresh() }

// Enable makes it work again.
func (s *Slider) Enable() { s.disabled = false; s.Refresh() }

// Text is the value as shown.
func (s *Slider) Text() string { return strconv.FormatFloat(s.Value, 'f', s.Decimals, 64) }

func (s *Slider) fraction() float64 {
	if s.Max <= s.Min {
		return 0
	}
	return (s.Value - s.Min) / (s.Max - s.Min)
}

// trackY is where the track's centre is, below the label line.
func (s *Slider) trackY() float32 { return sliderLabelH + sliderKnob/2 + theme.Space1 }

func (s *Slider) setFromX(x float32) {
	if s.disabled {
		return
	}
	w := s.Size().Width - sliderKnob
	if w <= 0 {
		return
	}
	f := float64(max(0, min((x-sliderKnob/2)/w, 1)))
	s.SetValue(s.Min + f*(s.Max-s.Min))
}

func (s *Slider) Tapped(e *fyne.PointEvent) {
	if e.Position.Y >= sliderLabelH {
		s.setFromX(e.Position.X)
	}
}
func (s *Slider) Dragged(e *fyne.DragEvent)      { s.setFromX(e.Position.X) }
func (s *Slider) DragEnd()                       {}
func (s *Slider) MouseIn(*desktop.MouseEvent)    { s.hovered = true; s.Refresh() }
func (s *Slider) MouseMoved(*desktop.MouseEvent) {}
func (s *Slider) MouseOut()                      { s.hovered = false; s.Refresh() }
func (s *Slider) FocusGained()                   { s.focused = true; s.Refresh() }
func (s *Slider) FocusLost()                     { s.focused = false; s.Refresh() }
func (s *Slider) TypedRune(rune)                 {}

// TypedKey steps with the arrows; Shift steps ten times as far (TypedShortcut carries it).
func (s *Slider) TypedKey(e *fyne.KeyEvent) { s.key(e.Name, 1) }

// TypedShortcut handles Shift with an arrow.
func (s *Slider) TypedShortcut(sc fyne.Shortcut) {
	if c, ok := sc.(*desktop.CustomShortcut); ok && c.Modifier == fyne.KeyModifierShift {
		s.key(c.KeyName, 10)
	}
}

func (s *Slider) key(k fyne.KeyName, times float64) {
	if s.disabled {
		return
	}
	step := s.Step
	if step <= 0 {
		step = (s.Max - s.Min) / 100
	}
	switch k {
	case fyne.KeyRight, fyne.KeyUp:
		s.SetValue(s.Value + step*times)
	case fyne.KeyLeft, fyne.KeyDown:
		s.SetValue(s.Value - step*times)
	case fyne.KeyHome:
		s.SetValue(s.Min)
	case fyne.KeyEnd:
		s.SetValue(s.Max)
	}
}

// AccessibilityLabel names the slider and its value.
func (s *Slider) AccessibilityLabel() string { return s.Label + ": " + s.Text() }

// AccessibilityRole is a button's (Fyne 2.8 has no slider role); the label carries the value.
func (s *Slider) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

const (
	sliderLabelH float32 = 18
	sliderKnob   float32 = 14
	sliderTrack  float32 = 4
)

func (s *Slider) CreateRenderer() fyne.WidgetRenderer {
	r := &sliderRenderer{s: s, label: NewText(s.Label, Body, Secondary), value: NewText("", Mono, Primary),
		track: canvas.NewRectangle(nil), fill: canvas.NewRectangle(nil), knob: canvas.NewCircle(nil),
		ring: canvas.NewRectangle(nil)}
	r.value.Align = fyne.TextAlignTrailing
	r.value.TextSize = theme.TextSmall
	r.Refresh()
	return r
}

type sliderRenderer struct {
	s            *Slider
	label, value *Text
	track, fill  *canvas.Rectangle
	knob         *canvas.Circle
	ring         *canvas.Rectangle
}

func (r *sliderRenderer) Refresh() {
	t := theme.Current()
	s := r.s
	r.label.SetText(s.Label)
	r.value.SetText(s.Text())
	r.track.FillColor, r.track.CornerRadius = t.Surface.Sunken, sliderTrack/2
	r.track.StrokeColor, r.track.StrokeWidth = t.Border.Strong, 1
	r.fill.FillColor, r.fill.CornerRadius = t.Text.Secondary, sliderTrack/2
	r.knob.FillColor = t.Text.Primary
	r.knob.StrokeColor, r.knob.StrokeWidth = t.Surface.Base, 2
	if s.disabled {
		r.fill.FillColor, r.knob.FillColor = t.Text.Disabled, t.Text.Disabled
	}
	styleRing(r.ring, t)
	r.ring.CornerRadius = (sliderKnob + 6) / 2
	r.ring.Hidden = !s.focused
	r.Layout(s.Size())
	for _, o := range []fyne.CanvasObject{r.track, r.fill, r.knob, r.ring} {
		o.Refresh()
	}
}

func (r *sliderRenderer) Layout(sz fyne.Size) {
	s := r.s
	lm := r.label.MinSize()
	r.label.Move(fyne.NewPos(0, (sliderLabelH-lm.Height)/2))
	r.label.Resize(fyne.NewSize(max(sz.Width-r.value.MinSize().Width-theme.Space2, 0), lm.Height))
	vm := r.value.MinSize()
	r.value.Move(fyne.NewPos(sz.Width-vm.Width, (sliderLabelH-vm.Height)/2))
	r.value.Resize(vm)
	y := s.trackY()
	w := sz.Width - sliderKnob
	r.track.Move(fyne.NewPos(sliderKnob/2, y-sliderTrack/2))
	r.track.Resize(fyne.NewSize(max(w, 0), sliderTrack))
	kx := sliderKnob/2 + float32(s.fraction())*max(w, 0)
	r.fill.Move(r.track.Position())
	r.fill.Resize(fyne.NewSize(kx-sliderKnob/2, sliderTrack))
	r.knob.Move(fyne.NewPos(kx-sliderKnob/2, y-sliderKnob/2))
	r.knob.Resize(fyne.NewSquareSize(sliderKnob))
	r.ring.Move(fyne.NewPos(kx-sliderKnob/2-3, y-sliderKnob/2-3))
	r.ring.Resize(fyne.NewSquareSize(sliderKnob + 6))
}

func (r *sliderRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.label.MinSize().Width+r.value.MinSize().Width+theme.Space4, r.s.trackY()+sliderKnob/2+theme.Space1)
}

func (r *sliderRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.label, r.value, r.track, r.fill, r.knob, r.ring}
}

func (r *sliderRenderer) Destroy() {}
