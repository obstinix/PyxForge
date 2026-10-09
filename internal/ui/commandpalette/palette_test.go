package commandpalette

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/kit"
)

func TestPalette(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("")
	w.Resize(fyne.NewSize(1024, 640))
	p := New(w.Canvas())

	ran := ""
	items := []Item{
		{Title: "Toggle Explorer", Detail: "View", Run: func() { ran = "explorer" }},
		{Title: "Toggle Inspector", Detail: "View", Run: func() { ran = "inspector" }},
		{Title: "Change Theme", Detail: "Preferences", Run: func() { ran = "theme" }},
	}
	p.Show("Run a command", items)
	if !p.Visible() || len(p.Results()) != 3 || w.Canvas().Focused() != p.entry {
		t.Fatalf("palette not open and focused with every item")
	}

	test.Type(p.entry, "insp")
	if r := p.Results(); len(r) != 1 || r[0].Title != "Toggle Inspector" {
		t.Errorf("filter: %v", r)
	}

	// The matched characters are drawn as strong runs: "Toggle " then "Insp" then "ector".
	title := container.New(runs{})
	setTitle(title, p.Results()[0], "insp")
	var got []string
	for _, o := range title.Objects {
		txt := o.(*kit.Text)
		if txt.Face == kit.Strong {
			got = append(got, txt.Text)
		}
	}
	if len(title.Objects) != 3 || len(got) != 1 || got[0] != "Insp" {
		t.Errorf("highlighted runs %v of %d", got, len(title.Objects))
	}

	// Arrow keys move the selection and wrap; Enter runs it and closes the palette.
	p.entry.SetText("toggle")
	p.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	p.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if p.sel != 0 {
		t.Errorf("selection did not wrap: %d", p.sel)
	}
	p.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	p.entry.OnSubmitted(p.entry.Text)
	if ran == "" || p.Visible() {
		t.Errorf("Enter ran %q, visible %v", ran, p.Visible())
	}

	// Nothing matching shows the empty state instead of an empty list.
	p.Show("Run a command", items)
	p.entry.SetText("zzz")
	if len(p.Results()) != 0 || !p.empty.Visible() || p.list.Visible() {
		t.Error("empty state not shown")
	}
	p.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if p.Visible() {
		t.Error("Escape did not close the palette")
	}
}
