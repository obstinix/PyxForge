package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
	"github.com/obstinix/PyxForge/internal/ui/icons"
)

// Fyne adapts a Selection to fyne.Theme. With SystemID the palette follows the variant Fyne
// passes in, so an OS light/dark switch takes effect without a restart.
type Fyne struct {
	sel Selection
}

// NewFyne returns the Fyne theme for a selection.
func NewFyne(sel Selection) *Fyne { return &Fyne{sel: sel} }

// Selection reports what this theme was built from.
func (f *Fyne) Selection() Selection { return f.sel }

// Tokens resolves the selection for an appearance variant.
func (f *Fyne) Tokens(v fyne.ThemeVariant) Tokens { return f.sel.Tokens(v) }

// Current returns the tokens in effect for the running app. PyxForge widgets call it from their
// renderers' Refresh, which Fyne runs on every theme change.
func Current() Tokens {
	app := fyne.CurrentApp()
	if app == nil {
		return Default.Tokens(fynetheme.VariantDark)
	}
	v := app.Settings().ThemeVariant()
	if f, ok := app.Settings().Theme().(*Fyne); ok {
		return f.Tokens(v)
	}
	return Default.Tokens(v)
}

func (f *Fyne) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	t := f.sel.Tokens(v)
	switch n {
	case fynetheme.ColorNameBackground:
		return t.Surface.Base
	case fynetheme.ColorNameButton, fynetheme.ColorNameHeaderBackground:
		return t.Surface.Raised
	case fynetheme.ColorNameDisabledButton, fynetheme.ColorNameInputBackground:
		return t.Surface.Sunken
	case fynetheme.ColorNameMenuBackground, fynetheme.ColorNameOverlayBackground:
		// Fyne's dialogs and menus float, so they take the overlay material (Section 11.5):
		// opaque by default, the glass tint with the glass setting on.
		return t.Glass.Fill
	case fynetheme.ColorNameForeground:
		return t.Text.Primary
	case fynetheme.ColorNamePlaceHolder:
		return t.Text.Tertiary
	case fynetheme.ColorNameDisabled:
		return t.Text.Disabled
	case fynetheme.ColorNameSeparator, fynetheme.ColorNameInnerWindowBorderInactive:
		return t.Border.Hairline
	case fynetheme.ColorNameInputBorder, fynetheme.ColorNameInnerWindowBorder:
		return t.Border.Strong
	case fynetheme.ColorNameHover:
		return t.State.Hover
	case fynetheme.ColorNamePressed:
		return t.State.Pressed
	case fynetheme.ColorNamePrimary, fynetheme.ColorNameHyperlink:
		return t.Accent.Primary
	case fynetheme.ColorNameForegroundOnPrimary:
		return t.Accent.OnPrimary
	case fynetheme.ColorNameFocus:
		// Fyne blends its focus colour over button, menu-item and check backgrounds, so it is a
		// wash; it is neutral so focus never looks like selection. PyxForge's own controls draw
		// a 2 px Accent.Focus ring instead.
		return t.State.Focus
	case fynetheme.ColorNameSelection:
		return t.Accent.Selection
	case fynetheme.ColorNameScrollBar:
		return withAlpha(t.Text.Tertiary, 0.5)
	case fynetheme.ColorNameScrollBarBackground:
		return color.Transparent
	case fynetheme.ColorNameShadow:
		// Fyne draws this over the workspace behind modals, and under menus.
		return t.Glass.Scrim
	case fynetheme.ColorNameSuccess:
		return t.Status.Success
	case fynetheme.ColorNameWarning:
		return t.Status.Warning
	case fynetheme.ColorNameError:
		return t.Status.Error
	case fynetheme.ColorNameForegroundOnError, fynetheme.ColorNameForegroundOnSuccess,
		fynetheme.ColorNameForegroundOnWarning:
		return t.Status.OnStatus
	}
	return fynetheme.DefaultTheme().Color(n, v)
}

func (f *Fyne) Font(s fyne.TextStyle) fyne.Resource {
	switch {
	case s.Symbol:
		return fynetheme.DefaultTheme().Font(s)
	case s.Monospace && s.Bold:
		return FontMonoBold
	case s.Monospace && s.Italic:
		return FontMonoItalic
	case s.Monospace:
		return FontMono
	case s.Bold:
		return FontUIStrong
	}
	// Geist ships no italic; UI text never relies on italics.
	return FontUI
}

// fyneIcons swaps the icons Fyne's own widgets draw (tree expanders, tab overflow and close)
// for the bundled Lucide set, so one icon language runs through the app. Icons on buttons
// that Fyne may disable or recolour by fill (dialog confirm buttons) keep Fyne's defaults:
// fill recolouring would fill these stroke-only outlines solid.
var fyneIcons = map[fyne.ThemeIconName]icons.Name{
	fynetheme.IconNameNavigateNext:   icons.ChevronRight,
	fynetheme.IconNameMenuExpand:     icons.ChevronRight,
	fynetheme.IconNameMoveDown:       icons.ChevronDown,
	fynetheme.IconNameMoreHorizontal: icons.Ellipsis,
	fynetheme.IconNameCancel:         icons.X,
	fynetheme.IconNameWindowClose:    icons.X,
}

func (f *Fyne) Icon(n fyne.ThemeIconName) fyne.Resource {
	if name, ok := fyneIcons[n]; ok {
		v := fynetheme.VariantDark
		if app := fyne.CurrentApp(); app != nil {
			v = app.Settings().ThemeVariant()
		}
		return icons.Get(name, f.Tokens(v).Text.Secondary)
	}
	return fynetheme.DefaultTheme().Icon(n)
}

func (f *Fyne) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case fynetheme.SizeNameText:
		return TextBody
	case fynetheme.SizeNameCaptionText:
		return TextCaption
	case fynetheme.SizeNameSubHeadingText:
		return TextLarge
	case fynetheme.SizeNameHeadingText:
		return TextHeading
	case fynetheme.SizeNamePadding, fynetheme.SizeNameLineSpacing:
		return Space1
	case fynetheme.SizeNameInnerPadding:
		return Space2
	case fynetheme.SizeNameInlineIcon:
		return IconSize
	case fynetheme.SizeNameSeparatorThickness, fynetheme.SizeNameInputBorder:
		return 1
	case fynetheme.SizeNameScrollBar:
		return Space2
	case fynetheme.SizeNameScrollBarSmall, fynetheme.SizeNameScrollBarRadius, fynetheme.SizeNameSelectionRadius:
		return 3
	case fynetheme.SizeNameInputRadius, fynetheme.SizeNameButtonRadius:
		return RadiusControl
	case fynetheme.SizeNameCardRadius:
		return RadiusPanel
	case fynetheme.SizeNameDialogRadius, fynetheme.SizeNamePopupRadius, fynetheme.SizeNameMenuRadius:
		return RadiusOverlay
	case fynetheme.SizeNameModalBlurRadius:
		return f.sel.Tokens(fynetheme.VariantDark).Glass.Blur
	}
	return fynetheme.DefaultTheme().Size(n)
}

func withAlpha(c color.NRGBA, a float64) color.NRGBA {
	return rgba(c.R, c.G, c.B, a)
}
