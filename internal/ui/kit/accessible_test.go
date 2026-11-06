package kit

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/icons"
)

// TestAccessibleNames: PyxForge's own controls give a screen reader the name a sighted user
// reads from them, and their kind (#49).
func TestAccessibleNames(t *testing.T) {
	test.NewTempApp(t)
	b := NewIconButton(icons.Terminal, "New Terminal (Ctrl+Shift+`)", func() {})
	if b.AccessibilityLabel() != "New Terminal (Ctrl+Shift+`)" || b.AccessibilityRole() != fyne.AccessibleRoleButton {
		t.Errorf("button: %q %q", b.AccessibilityLabel(), b.AccessibilityRole())
	}
	b.Selected = true
	if b.AccessibilityLabel() != "New Terminal (Ctrl+Shift+`) (selected)" {
		t.Errorf("selected button: %q", b.AccessibilityLabel())
	}
	b.Selected = false
	b.Disable()
	if b.AccessibilityLabel() != "New Terminal (Ctrl+Shift+`) (unavailable)" {
		t.Errorf("disabled button: %q", b.AccessibilityLabel())
	}
	title := NewText("Explorer", Label, Tertiary)
	title.Upper = true
	if title.AccessibilityLabel() != "EXPLORER" || title.AccessibilityRole() != fyne.AccessibleRoleText {
		t.Errorf("text: %q %q", title.AccessibilityLabel(), title.AccessibilityRole())
	}
	tabs := NewTabs(container.NewTabItem("boot.asm", NewText("a", Body, Primary)), container.NewTabItem("main.c", NewText("b", Body, Primary)))
	first := &tabButton{tabs: tabs, item: tabs.Items[0]}
	second := &tabButton{tabs: tabs, item: tabs.Items[1]}
	tabs.Select(tabs.Items[0])
	if first.AccessibilityLabel() != "boot.asm (selected tab)" || second.AccessibilityLabel() != "main.c (tab)" {
		t.Errorf("tabs: %q, %q", first.AccessibilityLabel(), second.AccessibilityLabel())
	}
}
