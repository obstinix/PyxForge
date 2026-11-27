package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// NewGallery lays out every component of this kit in every state it can be in (normal, hover,
// keyboard focus, pressed, disabled, on and off), for the design review render
// (tools/snapshot, scenario kit-gallery). States are set directly rather than by pointer, so
// one still image shows them all.
func NewGallery() fyne.CanvasObject {
	heading := func(s string) fyne.CanvasObject {
		t := NewText(s, Label, Tertiary)
		t.Upper = true
		return t
	}
	cell := func(o fyne.CanvasObject) fyne.CanvasObject { return container.NewCenter(o) }
	states := []string{"Normal", "Hover", "Focus", "Pressed", "Disabled"}

	// Buttons: a row per variant, a column per state.
	grid := []fyne.CanvasObject{NewText("", Label, Tertiary)}
	for _, s := range states {
		grid = append(grid, cell(NewText(s, Label, Tertiary)))
	}
	for _, v := range []struct {
		name string
		v    Variant
		text string
		icon icons.Name
	}{{"Default", ButtonDefault, "Check for Updates", ""}, {"CTA", ButtonCTA, "Install Selected", ""},
		{"Ghost", ButtonGhost, "Open File", icons.FileText}, {"Danger", ButtonDanger, "Discard Changes", ""},
		{"Icon", ButtonIcon, "Refresh", icons.Refresh}} {
		grid = append(grid, NewText(v.name, Body, Secondary))
		for i := range states {
			b := NewButton(v.text, func() {})
			b.Variant, b.Icon = v.v, v.icon
			switch i {
			case 1:
				b.hovered = true
			case 2:
				b.focused = true
			case 3:
				b.hovered, b.pressed = true, true
			case 4:
				b.disabled = true
			}
			grid = append(grid, cell(b))
		}
	}
	buttons := container.New(layout.NewGridLayoutWithColumns(len(states)+1), grid...)

	// Switches, dropdowns and sliders in their states.
	sw := func(on, hover, focus, disabled bool) fyne.CanvasObject {
		s := NewSwitch(on, nil)
		s.hovered, s.focused, s.disabled = hover, focus, disabled
		return cell(s)
	}
	switches := container.NewGridWithColumns(6,
		sw(false, false, false, false), sw(true, false, false, false), sw(false, true, false, false),
		sw(true, false, true, false), sw(false, false, false, true), sw(true, false, false, true))
	dd := func(hover, focus, disabled bool) fyne.CanvasObject {
		d := NewDropdown([]string{"Flat", "Liquid Glass"}, "Flat", nil)
		d.hovered, d.focused, d.disabled = hover, focus, disabled
		return cell(d)
	}
	dropdowns := container.NewGridWithColumns(4, dd(false, false, false), dd(true, false, false), dd(false, true, false), dd(false, false, true))
	sl := func(label string, v float64, focus, disabled bool) fyne.CanvasObject {
		s := NewSlider(label, 0, 1, 0.01, v, nil)
		s.focused, s.disabled = focus, disabled
		return container.New(layout.NewCustomPaddedLayout(0, 0, 0, theme.Space4), s)
	}
	sliders := container.New(layout.NewGridLayoutWithColumns(3), sl("Link opacity", 0.06, false, false), sl("Pull to centre", 0.25, true, false), sl("Smoothness", 0.6, false, true))

	// A settings group, search fields and a navigation list.
	shell := NewDropdown([]string{"PowerShell 7 (pwsh)", "Windows PowerShell", "Command Prompt (cmd.exe)"}, "PowerShell 7 (pwsh)", nil)
	group := NewSettingGroup("Terminal",
		NewSettingRow("Shell", "The program new terminal sessions run. Command blocks need PowerShell, bash, zsh or fish.", shell),
		NewSettingRow("Command blocks", "Marks each command with its exit status and duration, through shell integration that PyxForge adds to the sessions it starts.", NewSwitch(true, nil)).
			WithLink("How shell integration works", func() {}),
		NewSettingRow("Check for updates", "At start-up, at most once a week, asks GitHub whether a newer PyxForge is out. It downloads nothing.", NewButton("Check Now", func() {})))
	empty := NewSearchField("Search settings…", nil)
	typed := NewSearchField("Search settings…", nil)
	typed.SetText("font size")
	nav := NewNavList([]NavItem{{Label: "Options", Heading: true}, {ID: "general", Label: "General", Icon: icons.Settings},
		{ID: "editor", Label: "Editor", Icon: icons.FileCode}, {ID: "appearance", Label: "Appearance", Icon: icons.Palette},
		{ID: "terminal", Label: "Terminal", Icon: icons.Terminal}, {Label: "Core features", Heading: true},
		{ID: "blocks", Label: "Command Blocks", Icon: icons.List}}, nil)
	nav.focused, nav.cursor = true, 4

	left := container.New(layout.NewCustomPaddedVBoxLayout(theme.Space3),
		heading("Buttons"), buttons,
		heading("Switches: off, on, hover, focus, disabled off, disabled on"), switches,
		heading("Dropdowns: normal, hover, focus, disabled"), dropdowns,
		heading("Sliders: normal, focus, disabled"), sliders)
	right := container.New(layout.NewCustomPaddedVBoxLayout(theme.Space3),
		heading("Search fields"), empty, typed,
		heading("Navigation list (keyboard cursor on Terminal)"), container.NewGridWrap(fyne.NewSize(240, nav.MinSize().Height), nav),
		group)
	body := container.New(layout.NewCustomPaddedLayout(theme.Space6, theme.Space6, theme.Space6, theme.Space6),
		container.NewBorder(nil, nil, left, nil, container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space6, 0), right)))
	return NewSurface(Base, body)
}
