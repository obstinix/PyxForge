package editor

import "fyne.io/fyne/v2"

var _ fyne.Accessible = (*View)(nil)

// AccessibilityLabel names the editor's text area (#49). Neovim draws it as a grid of cells, so
// a screen reader gets its name, not its text; Neovim itself, in a terminal, serves screen
// readers better (docs/ACCESSIBILITY.md).
func (v *View) AccessibilityLabel() string {
	if v.Label != "" {
		return v.Label
	}
	return "Editor"
}

// AccessibilityRole is text's.
func (v *View) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleText }
