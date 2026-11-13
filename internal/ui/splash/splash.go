// Package splash is PyxForge's start-up screen: a borderless window in the user's theme that
// shows the work start-up really does (each step's name, a bar that advances as steps finish)
// while that work runs off the UI thread, then hands over to the main window. Nothing in it
// waits for show: a step that fails is reported and start-up goes on, and the window closes
// whatever happens. With reduced motion the screen is drawn once, without animation.
package splash

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/ui/brand"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Step is one piece of start-up work.
type Step struct {
	Label string       // shown while it runs, such as "Preparing the editor"
	Run   func() error // runs off the UI thread
}

// Options adjust the screen.
type Options struct {
	ReduceMotion bool          // draw it still: the user or the system asked for less motion
	MinDuration  time.Duration // keep it up at least this long, so it does not flash by
}

// Size of the window, in Fyne units.
var size = fyne.NewSize(560, 340)

// Run shows the screen, runs steps in order off the UI thread and then, on the UI thread,
// closes it and calls done with the errors of the steps that failed (nil when none did). When
// the driver has no splash windows (tests, mobile), the steps still run and done is called.
func Run(a fyne.App, t theme.Tokens, steps []Step, o Options, done func(errs []error)) {
	var sp *screen
	if drv, ok := a.Driver().(desktop.Driver); ok {
		sp = newScreen(drv.CreateSplashWindow(), t, len(steps), o)
		sp.win.Show()
		sp.enter()
	}
	go func() {
		start := time.Now()
		var errs []error
		for i, st := range steps {
			if sp != nil {
				fyne.Do(func() { sp.status(st.Label, i, len(steps)) })
			}
			if err := st.Run(); err != nil {
				errs = append(errs, err)
			}
			if sp != nil {
				fyne.Do(func() { sp.status(st.Label, i+1, len(steps)) })
			}
		}
		if wait := o.MinDuration - time.Since(start); wait > 0 && sp != nil {
			time.Sleep(wait)
		}
		fyne.Do(func() {
			done(errs)
			if sp != nil {
				sp.win.Close()
			}
		})
	}()
}

// screen is the window's drawing: the forge mark in the middle, the name and the line under
// it, and along the bottom the step that is running, the version and a thin bar of the steps
// done.
type screen struct {
	win     fyne.Window
	t       theme.Tokens
	o       Options
	anvil   *canvas.Image
	pyx     *canvas.Image
	spark   *canvas.Image
	name    *canvas.Image
	tagline *canvas.Image
	step    *canvas.Text
	track   *canvas.Rectangle
	bar     *canvas.Rectangle
	shown   float32 // the bar's drawn fraction, eased toward target
	target  float32
	barAnim *fyne.Animation
	intro   *fyne.Animation
}

// Where things are, in Fyne units on the 560 by 340 window.
const (
	markSide   = 108
	markTop    = 48
	nameHeight = 30
	nameTop    = 176
	tagHeight  = 10
	tagTop     = 222
	margin     = 40
)

func newScreen(w fyne.Window, t theme.Tokens, steps int, o Options) *screen {
	sp := &screen{win: w, t: t, o: o}
	parts := brand.SplashParts(t.Polarity == theme.Light)
	bg := canvas.NewRectangle(t.Surface.Sunken)
	bg.Resize(size)
	edge := canvas.NewRectangle(color.Transparent)
	edge.StrokeColor, edge.StrokeWidth = t.Border.Strong, 1
	edge.Resize(size)

	part := func(r fyne.Resource) *canvas.Image {
		img := canvas.NewImageFromResource(r)
		img.FillMode = canvas.ImageFillContain
		img.Resize(fyne.NewSize(markSide, markSide))
		img.Move(markPos)
		return img
	}
	sp.anvil, sp.pyx, sp.spark = part(parts.Anvil), part(parts.Pyx), part(parts.Spark)
	line := func(r fyne.Resource, aspect, height, top float32) *canvas.Image {
		img := canvas.NewImageFromResource(r)
		img.FillMode = canvas.ImageFillContain
		img.Resize(fyne.NewSize(height*aspect, height))
		img.Move(fyne.NewPos((size.Width-height*aspect)/2, top))
		return img
	}
	sp.name = line(parts.Name, parts.NameAspect, nameHeight, nameTop)
	sp.tagline = line(parts.Tagline, parts.TaglineAspect, tagHeight, tagTop)

	sp.step = canvas.NewText("Starting", t.Text.Tertiary)
	sp.step.FontSource = theme.FontUI
	sp.step.TextSize = 12
	sp.step.Move(fyne.NewPos(margin, size.Height-58))
	ver := canvas.NewText(buildinfo.Version, t.Text.Tertiary)
	ver.FontSource = theme.FontMono
	ver.TextSize = 11
	ver.Alignment = fyne.TextAlignTrailing
	ver.Resize(fyne.NewSize(240, 16))
	ver.Move(fyne.NewPos(size.Width-margin-240, size.Height-58))

	sp.track = canvas.NewRectangle(t.Border.Hairline)
	sp.track.Resize(fyne.NewSize(size.Width-2*margin, 2))
	sp.track.Move(fyne.NewPos(margin, size.Height-32))
	sp.bar = canvas.NewRectangle(t.Accent.Primary)
	sp.bar.Resize(fyne.NewSize(0, 2))
	sp.bar.Move(sp.track.Position())

	content := container.NewWithoutLayout(bg, edge, sp.anvil, sp.pyx, sp.spark, sp.name, sp.tagline, sp.step, ver, sp.track, sp.bar)
	content.Resize(size)
	w.SetPadded(false) // the drawing fills the window edge to edge
	w.SetContent(content)
	w.Resize(size)
	w.SetFixedSize(true)
	w.CenterOnScreen()
	if o.ReduceMotion {
		sp.frame(1) // drawn as the entrance ends
	} else {
		sp.frame(0)
	}
	return sp
}

var markPos = fyne.NewPos((560-markSide)/2, markTop)

// introLength is the whole entrance. Start-up work runs meanwhile, off the UI thread; the
// entrance never waits for it and it never waits for the entrance.
const introLength = 800 * time.Millisecond

// MinDuration is how long the screen stays up at least with motion on: the entrance and a
// moment to read it. A start that takes longer is never held up by it.
const MinDuration = introLength + 150*time.Millisecond

// frame draws the entrance at f, from 0 (nothing but the surface) to 1 (the finished screen):
//
//	the anvil rises into place and settles;
//	the piece comes down onto its face, gathering speed, as a hammer blow does;
//	on the blow the anvil gives by two units and the spark leaves the piece;
//	the name, then the line under it, come up as the metal cools.
func (sp *screen) frame(f float32) {
	span := func(from, to float32) float32 { return clamp((f - from) / (to - from)) }
	out := func(x float32) float32 { return 1 - (1-x)*(1-x)*(1-x) } // decelerating
	in := func(x float32) float32 { return x * x }                  // accelerating

	rise := out(span(0, 0.38))
	blow := span(0.60, 0.74) // 0 before the piece lands, 1 once the anvil has settled again
	give := float32(0)
	if blow > 0 && blow < 1 {
		give = 2 * (1 - (2*blow-1)*(2*blow-1)) // down two units and back
	}
	place := func(img *canvas.Image, dx, dy, alpha float32) {
		img.Move(fyne.NewPos(markPos.X+dx, markPos.Y+dy))
		img.Translucency = float64(1 - clamp(alpha))
		img.Refresh()
	}
	place(sp.anvil, 0, (1-rise)*10+give, rise)
	fall := in(span(0.26, 0.60))
	place(sp.pyx, 0, -(1-fall)*34+give, span(0.26, 0.40))
	fly := out(span(0.60, 0.86))
	place(sp.spark, -(1-fly)*9, (1-fly)*9, span(0.60, 0.70))
	fade := func(img *canvas.Image, alpha float32) {
		img.Translucency = float64(1 - clamp(alpha))
		img.Refresh()
	}
	fade(sp.name, out(span(0.56, 0.90)))
	fade(sp.tagline, out(span(0.72, 1)))
}

// enter plays the entrance. Reduced motion skips it; the screen is already drawn as it ends.
func (sp *screen) enter() {
	if sp.o.ReduceMotion {
		return
	}
	sp.intro = fyne.NewAnimation(introLength, sp.frame)
	sp.intro.Curve = fyne.AnimationLinear // frame has each part's own easing
	sp.intro.Start()
}

// status shows the step running and how many of n are done. It runs on the UI thread.
func (sp *screen) status(label string, done, n int) {
	sp.step.Text = label
	sp.step.Refresh()
	if n > 0 {
		sp.target = float32(done) / float32(n)
	}
	full := sp.track.Size().Width
	if sp.o.ReduceMotion {
		sp.shown = sp.target
		sp.bar.Resize(fyne.NewSize(full*sp.shown, 2))
		return
	}
	if sp.barAnim != nil {
		sp.barAnim.Stop()
	}
	from := sp.shown
	sp.barAnim = fyne.NewAnimation(220*time.Millisecond, func(f float32) {
		sp.shown = from + (sp.target-from)*f
		sp.bar.Resize(fyne.NewSize(full*sp.shown, 2))
	})
	sp.barAnim.Curve = fyne.AnimationEaseOut
	sp.barAnim.Start()
}

func clamp(f float32) float32 { return max(0, min(1, f)) }
