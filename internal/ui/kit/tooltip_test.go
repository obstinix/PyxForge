package kit

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/icons"
)

// TestTooltips: resting on an icon button shows its label under it, inside the window;
// leaving, or clicking, hides it; a button with no label has none.
func TestTooltips(t *testing.T) {
	test.NewTempApp(t)
	tips := NewTooltips()
	tips.Delay = 0
	b := NewIconButton(icons.Refresh, "Reload Explorer", func() {})
	plain := NewIconButton(icons.X, "", func() {})
	content := container.NewStack(container.NewVBox(b, plain), tips.Layer())
	w := test.NewTempWindow(t, content)
	w.Resize(fyne.NewSize(400, 300))

	b.MouseIn(nil)
	if tips.Text() != "Reload Explorer" {
		t.Fatalf("tooltip %q", tips.Text())
	}
	if tips.tip.Position().Y < b.Position().Y+b.Size().Height {
		t.Errorf("the tooltip covers its button: tip at %v, button %v+%v", tips.tip.Position(), b.Position(), b.Size())
	}
	b.MouseOut()
	if tips.Text() != "" {
		t.Error("the tooltip stayed after the pointer left")
	}
	b.MouseIn(nil)
	test.Tap(b)
	if tips.Text() != "" {
		t.Error("the tooltip stayed after a click")
	}
	plain.MouseIn(nil)
	if tips.Text() != "" {
		t.Error("a button with no label showed a tooltip")
	}
}
