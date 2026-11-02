package splash

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

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
// steps done, and with reduced motion everything is drawn at once (the grid visible, the bar
// at its place without easing).
func TestScreen(t *testing.T) {
	test.NewTempApp(t)
	tk := theme.Current()
	sp := newScreen(test.NewWindow(nil), tk, 4, Options{ReduceMotion: true})
	if sp.logo.Translucency != 0 || sp.title.Color != tk.Text.Primary {
		t.Error("with reduced motion the mark and name are not drawn at once")
	}
	if len(sp.grid) == 0 {
		t.Fatal("no grid")
	}
	if _, _, _, a := sp.grid[0].StrokeColor.RGBA(); a == 0 {
		t.Error("with reduced motion the grid is not drawn")
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
	if moving.logo.Translucency != 1 {
		t.Error("with motion the mark should start hidden, to fade in")
	}
	if _, _, _, a := moving.title.Color.RGBA(); a != 0 {
		t.Error("with motion the name should start transparent")
	}
	if got := moving.win.Content().Size(); got != size {
		t.Errorf("content size %v, want %v: the drawing should fill the window", got, size)
	}
}
