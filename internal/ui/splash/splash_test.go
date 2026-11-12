package splash

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// TestRunsEveryStep: the steps run in order, off the UI thread; one that fails is reported and
// the rest still run; done is called once, after all of them.
func TestRunsEveryStep(t *testing.T) {
	a := test.NewTempApp(t)
	var mu sync.Mutex
	var ran []string
	step := func(name string, err error) Step {
		return Step{Label: name, Run: func() error {
			mu.Lock()
			ran = append(ran, name)
			mu.Unlock()
			return err
		}}
	}
	broken := errors.New("the editor's configuration could not be prepared")
	got := make(chan []error, 2)
	Run(a, theme.Current(), []Step{step("one", nil), step("two", broken), step("three", nil)}, Options{}, func(errs []error) { got <- errs })
	select {
	case errs := <-got:
		if len(errs) != 1 || !errors.Is(errs[0], broken) {
			t.Errorf("errors %v", errs)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("done was not called")
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(ran, []string{"one", "two", "three"}) {
		t.Errorf("ran %v", ran)
	}
	select {
	case <-got:
		t.Error("done was called twice")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestScreen draws the screen in a test window: the step's name shows, the bar follows the
// steps done, and with reduced motion everything is drawn at once, where the entrance ends.
func TestScreen(t *testing.T) {
	test.NewTempApp(t)
	tk := theme.Current()
	sp := newScreen(test.NewWindow(nil), tk, 4, Options{ReduceMotion: true})
	for name, img := range map[string]*canvas.Image{"anvil": sp.anvil, "piece": sp.pyx, "spark": sp.spark, "name": sp.name, "line": sp.tagline} {
		if img.Translucency != 0 {
			t.Errorf("with reduced motion the %s is not drawn at once (translucency %v)", name, img.Translucency)
		}
	}
	if sp.anvil.Position() != markPos || sp.pyx.Position() != markPos || sp.spark.Position() != markPos {
		t.Error("with reduced motion the mark's parts are not in place")
	}
	sp.enter()
	if sp.intro != nil {
		t.Error("with reduced motion an entrance was started")
	}
	sp.status("Preparing the editor", 1, 4)
	if sp.step.Text != "Preparing the editor" {
		t.Errorf("status %q", sp.step.Text)
	}
	want := sp.track.Size().Width / 4
	if got := sp.bar.Size().Width; got != want {
		t.Errorf("bar %v after one of four steps, want %v", got, want)
	}
	sp.status("Locating toolchains", 4, 4)
	if got := sp.bar.Size().Width; got != sp.track.Size().Width {
		t.Errorf("bar %v when all are done, want the full %v", got, sp.track.Size().Width)
	}

	moving := newScreen(test.NewWindow(nil), tk, 2, Options{})
	if moving.anvil.Translucency != 1 || moving.name.Translucency != 1 {
		t.Error("with motion the mark and the name should start hidden")
	}
	if got := moving.win.Content().Size(); got != size {
		t.Errorf("content size %v, want %v: the drawing should fill the window", got, size)
	}
}

// TestEntrance follows the entrance frame by frame: the anvil comes first, the piece is still
// above its place and falling until the blow, the spark is not there before it, the lettering
// comes last, and the last frame is the finished screen, the one reduced motion draws.
func TestEntrance(t *testing.T) {
	test.NewTempApp(t)
	sp := newScreen(test.NewWindow(nil), theme.Current(), 2, Options{})
	sp.frame(0.2)
	if sp.anvil.Translucency >= 1 || sp.pyx.Translucency != 1 || sp.name.Translucency != 1 {
		t.Errorf("early: anvil %v piece %v name %v", sp.anvil.Translucency, sp.pyx.Translucency, sp.name.Translucency)
	}
	sp.frame(0.45)
	mid := sp.pyx.Position().Y
	sp.frame(0.55)
	if late := sp.pyx.Position().Y; !(mid < late && late < markPos.Y) {
		t.Errorf("the piece is not coming down onto the anvil: %v then %v, to rest at %v", mid, late, markPos.Y)
	}
	if sp.spark.Translucency != 1 {
		t.Error("the spark shows before the blow")
	}
	sp.frame(0.67)
	if sp.pyx.Position().Y <= markPos.Y || sp.anvil.Position().Y <= markPos.Y {
		t.Error("the anvil does not give under the blow")
	}
	if sp.spark.Translucency >= 1 {
		t.Error("no spark at the blow")
	}
	sp.frame(1)
	still := newScreen(test.NewWindow(nil), theme.Current(), 2, Options{ReduceMotion: true})
	for name, pair := range map[string][2]*canvas.Image{"anvil": {sp.anvil, still.anvil}, "piece": {sp.pyx, still.pyx},
		"spark": {sp.spark, still.spark}, "name": {sp.name, still.name}, "line": {sp.tagline, still.tagline}} {
		if pair[0].Position() != pair[1].Position() || pair[0].Translucency != pair[1].Translucency {
			t.Errorf("the %s ends at %v (translucency %v), not where the still screen has it", name, pair[0].Position(), pair[0].Translucency)
		}
	}
	if MinDuration > time.Second {
		t.Errorf("the screen stays up %v at least: start-up must not feel slower for it", MinDuration)
	}
}

// TestFrames writes the entrance's frames as PNGs when PYXFORGE_SPLASH_FRAMES names a folder,
// for looking at; it checks nothing.
func TestFrames(t *testing.T) {
	dir := os.Getenv("PYXFORGE_SPLASH_FRAMES")
	if dir == "" {
		t.Skip("set PYXFORGE_SPLASH_FRAMES to a folder to write the frames")
	}
	a := test.NewTempApp(t)
	for _, sel := range []theme.Selection{{PaletteID: theme.SmokedKraft.ID, AccentID: theme.Crimson.ID}, {PaletteID: theme.InkPaper.ID, AccentID: theme.Amber.ID}} {
		a.Settings().SetTheme(theme.NewFyne(sel))
		w := test.NewWindow(nil)
		sp := newScreen(w, theme.Current(), 4, Options{})
		sp.status("Preparing the editor", 1, 4)
		for _, f := range []float32{0.15, 0.45, 0.58, 0.67, 0.8, 1} {
			sp.frame(f)
			out, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%03.0f.png", sel.PaletteID, f*100)))
			if err != nil {
				t.Fatal(err)
			}
			_ = png.Encode(out, w.Canvas().Capture())
			out.Close()
		}
	}
}
