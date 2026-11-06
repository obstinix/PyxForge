package kit

import (
	"strings"

	"fyne.io/fyne/v2"
)

// Accessible names and roles (fyne.Accessible, Fyne 2.8; #49). A build with Fyne's
// accessibility support (the accessibility build tag) gives them to Windows UI Automation and
// macOS's accessibility API, which screen readers read; without it they are not used.

var (
	_ fyne.Accessible = (*IconButton)(nil)
	_ fyne.Accessible = (*Text)(nil)
	_ fyne.Accessible = (*tabButton)(nil)
)

// AccessibilityLabel is the button's label, which is also its tooltip.
func (b *IconButton) AccessibilityLabel() string {
	if b.disabled {
		return b.Label + " (unavailable)"
	}
	if b.Selected {
		return b.Label + " (selected)"
	}
	return b.Label
}

// AccessibilityRole is a button's.
func (b *IconButton) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

// AccessibilityLabel is the text as shown.
func (t *Text) AccessibilityLabel() string {
	if t.Upper {
		return strings.ToUpper(t.Text)
	}
	return t.Text
}

// AccessibilityRole is text's.
func (t *Text) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleText }

// AccessibilityLabel is the tab's title, saying whether it is the selected one.
func (b *tabButton) AccessibilityLabel() string {
	if b.item == nil {
		return ""
	}
	if b.tabs != nil && b.tabs.Selected() == b.item {
		return b.item.Text + " (selected tab)"
	}
	return b.item.Text + " (tab)"
}

// AccessibilityRole is a button's: a tab is chosen by pressing it.
func (b *tabButton) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }
