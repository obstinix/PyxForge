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

// screen is the window's drawing.
type screen struct {
	win      fyne.Window
	t        theme.Tokens
	o        Options
	grid     []*canvas.Line
	logo     *canvas.Image
	title    *canvas.Text
	tagline  *canvas.Text
	step     *canvas.Text
	track    *canvas.Rectangle
	bar      *canvas.Rectangle
	shown    float32 // the bar's drawn fraction, eased toward target
	target   float32
	barAnim  *fyne.Animation
	gridBase color.NRGBA
}

const gridGap = 28

func newScreen(w fyne.Window, t theme.Tokens, steps int, o Options) *screen {
	sp := &screen{win: w, t: t, o: o, gridBase: t.Border.Hairline}
	bg := canvas.NewRectangle(t.Surface.Sunken)
	bg.Resize(size)
	objs := []fyne.CanvasObject{bg}
	// A technical grid, faint, under everything: the forge's work surface.
	for x := float32(gridGap); x < size.Width; x += gridGap {
		l := canvas.NewLine(transparent(sp.gridBase))
		l.StrokeWidth = 1
		l.Position1, l.Position2 = fyne.NewPos(x, 0), fyne.NewPos(x, size.Height)
		sp.grid = append(sp.grid, l)
		objs = append(objs, l)
	}
	for y := float32(gridGap); y < size.Height; y += gridGap {
		l := canvas.NewLine(transparent(sp.gridBase))
		l.StrokeWidth = 1
		l.Position1, l.Position2 = fyne.NewPos(0, y), fyne.NewPos(size.Width, y)
		sp.grid = append(sp.grid, l)
		objs = append(objs, l)
	}
	edge := canvas.NewRectangle(color.Transparent)
	edge.StrokeColor, edge.StrokeWidth = t.Border.Strong, 1
	edge.Resize(size)

	sp.logo = canvas.NewImageFromResource(brand.Icon)
	sp.logo.FillMode = canvas.ImageFillContain
	sp.logo.Resize(fyne.NewSize(76, 76))
	sp.logo.Move(fyne.NewPos(56, 104))

	sp.title = canvas.NewText("PyxForge", t.Text.Primary)
	sp.title.FontSource = theme.FontDisplay
	sp.title.TextSize = 40
	sp.title.Move(fyne.NewPos(152, 104))
	sp.tagline = canvas.NewText("A native development environment", t.Text.Secondary)
	sp.tagline.FontSource = theme.FontUI
	sp.tagline.TextSize = 14
	sp.tagline.Move(fyne.NewPos(154, 160))

	sp.step = canvas.NewText("Starting", t.Text.Tertiary)
	sp.step.FontSource = theme.FontMono
	sp.step.TextSize = 12
	sp.step.Move(fyne.NewPos(56, size.Height-56))
	ver := canvas.NewText(buildinfo.Version, t.Text.Tertiary)
	ver.FontSource = theme.FontMono
	ver.TextSize = 12
	ver.Alignment = fyne.TextAlignTrailing
	ver.Resize(fyne.NewSize(200, 16))
	ver.Move(fyne.NewPos(size.Width-56-200, size.Height-56))

	sp.track = canvas.NewRectangle(t.Border.Hairline)
	sp.track.Resize(fyne.NewSize(size.Width-112, 2))
	sp.track.Move(fyne.NewPos(56, size.Height-32))
	sp.bar = canvas.NewRectangle(t.Accent.Primary)
	sp.bar.Resize(fyne.NewSize(0, 2))
	sp.bar.Move(sp.track.Position())

	objs = append(objs, edge, sp.logo, sp.title, sp.tagline, sp.step, ver, sp.track, sp.bar)
	content := container.NewWithoutLayout(objs...)
	content.Resize(size)
	w.SetPadded(false) // the drawing fills the window edge to edge
	w.SetContent(content)
	w.Resize(size)
	w.SetFixedSize(true)
	w.CenterOnScreen()
	if !o.ReduceMotion {
		sp.logo.Translucency, sp.title.Color, sp.tagline.Color = 1, transparent(t.Text.Primary), transparent(t.Text.Secondary)
	} else {
		for _, l := range sp.grid {
			l.StrokeColor = withAlpha(sp.gridBase, 0.55)
		}
	}
	return sp
}

// enter plays the entrance: the grid sweeps in from the left, then the mark and the name
// fade up. Reduced motion skips it; the screen is drawn as it ends.
func (sp *screen) enter() {
	if sp.o.ReduceMotion {
		return
	}
	n := float32(len(sp.grid))
	grid := fyne.NewAnimation(700*time.Millisecond, func(f float32) {
		for i, l := range sp.grid {
			// each line starts a little after the one before it
			local := clamp((f*1.6 - float32(i)/n*0.6))
			l.StrokeColor = withAlpha(sp.gridBase, 0.55*local)
			l.Refresh()
		}
	})
	grid.Curve = fyne.AnimationEaseOut
	brandIn := fyne.NewAnimation(600*time.Millisecond, func(f float32) {
		sp.logo.Translucency = float64(1 - f)
		sp.logo.Refresh()
		sp.title.Color = withAlpha(sp.t.Text.Primary, f)
		sp.title.Refresh()
		sp.tagline.Color = withAlpha(sp.t.Text.Secondary, clamp(f*1.3-0.3))
		sp.tagline.Refresh()
	})
	brandIn.Curve = fyne.AnimationEaseOut
	grid.Start()
	time.AfterFunc(250*time.Millisecond, func() { fyne.Do(brandIn.Start) })
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

func transparent(c color.NRGBA) color.NRGBA { c.A = 0; return c }

func withAlpha(c color.NRGBA, f float32) color.NRGBA {
	c.A = uint8(float32(c.A) * clamp(f))
	return c
}
