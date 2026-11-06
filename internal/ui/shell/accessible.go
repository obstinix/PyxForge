package shell

import "fyne.io/fyne/v2"

var (
	_ fyne.Accessible = (*themeCard)(nil)
	_ fyne.Accessible = (*accentCard)(nil)
)

// AccessibilityLabel names a theme card as Settings shows it, saying which is current (#49).
func (c *themeCard) AccessibilityLabel() string {
	if c.s.sel.PaletteID == c.id {
		return c.name + " theme (current)"
	}
	return c.name + " theme"
}

// AccessibilityRole is a button's: pressing a card applies the theme.
func (c *themeCard) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

// AccessibilityLabel names an accent card, saying which is current.
func (c *accentCard) AccessibilityLabel() string {
	if c.s.sel.AccentID == c.a.ID {
		return c.a.Name + " accent (current)"
	}
	return c.a.Name + " accent"
}

// AccessibilityRole is a button's.
func (c *accentCard) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }
