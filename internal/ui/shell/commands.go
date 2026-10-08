package shell

import (
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/obstinix/PyxForge/internal/command"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// registerCommands registers every action once. Only commands that do something real today
// are registered; later phases add theirs.
func (s *Shell) registerCommands() {
	mod := fyne.KeyModifierShortcutDefault
	key := func(k fyne.KeyName, m fyne.KeyModifier) *desktop.CustomShortcut {
		return &desktop.CustomShortcut{KeyName: k, Modifier: m}
	}
	add := func(id, category, title string, sc *desktop.CustomShortcut, run func()) {
		keys := ""
		if sc != nil {
			keys = shortcutLabel(sc)
			s.win.Canvas().AddShortcut(sc, func(fyne.Shortcut) { run() })
		}
		s.cmds.Add(command.Command{ID: id, Title: title, Category: category, Keys: keys, Run: run})
	}

	add("view.commands", "View", "Show All Commands", key(fyne.KeyP, mod|fyne.KeyModifierShift), s.ShowCommands)
	add("go.file", "Go", "Go to File", key(fyne.KeyP, mod), s.QuickOpen)
	add("view.explorer", "View", "Toggle Explorer", key(fyne.KeyB, mod), s.ToggleExplorer)
	add("view.panel", "View", "Toggle Panel", key(fyne.KeyJ, mod), s.ToggleDock)
	add("view.inspector", "View", "Toggle Inspector", key(fyne.KeyB, mod|fyne.KeyModifierAlt), s.ToggleInspector)
	add("view.closeEditor", "View", "Close Editor Tab", key(fyne.KeyW, mod), s.closeEditor)
	add("explorer.reload", "Explorer", "Reload", nil, s.explorer.Reload)
	add("prefs.settings", "Preferences", "Open Settings", key(fyne.KeyComma, mod), s.OpenSettings)
	add("prefs.theme", "Preferences", "Change Theme", nil, s.chooseTheme)
	add("prefs.accent", "Preferences", "Change Accent", nil, s.chooseAccent)
	add("prefs.theme."+theme.SystemID, "Preferences", "Theme: System", nil, func() { s.setPalette(theme.SystemID) })
	for _, p := range theme.Palettes {
		id := p.ID
		add("prefs.theme."+id, "Preferences", "Theme: "+p.Name, nil, func() { s.setPalette(id) })
	}
	for _, a := range theme.Accents {
		id := a.ID
		add("prefs.accent."+id, "Preferences", "Accent: "+a.Name, nil, func() { s.setAccent(id) })
	}
	add("prefs.glass", "Preferences", "Toggle Glass Overlays", nil, func() { s.setGlass(!s.sel.Glass) })
	add("prefs.reset", "Preferences", "Reset Appearance", nil, s.confirmReset)
	add("help.about", "Help", "About PyxForge", nil, s.about)
}

func (s *Shell) setPalette(id string) {
	sel := s.sel
	sel.PaletteID = id
	s.SetSelection(sel)
}

func (s *Shell) setAccent(id string) {
	sel := s.sel
	sel.AccentID = id
	s.SetSelection(sel)
}

func (s *Shell) setGlass(on bool) {
	sel := s.sel
	sel.Glass = on
	s.SetSelection(sel)
}

// chooseTheme lists the themes in the palette (the "Change Theme" action D2 asks for).
func (s *Shell) chooseTheme() {
	mark := func(id string) string {
		if s.sel.PaletteID == id {
			return "current"
		}
		return ""
	}
	items := []commandpalette.Item{{Title: "System", Detail: mark(theme.SystemID), Icon: icons.SunMoon,
		Run: func() { s.setPalette(theme.SystemID) }}}
	for _, p := range theme.Palettes {
		id := p.ID
		items = append(items, commandpalette.Item{Title: p.Name, Detail: mark(id), Icon: icons.Palette,
			Run: func() { s.setPalette(id) }})
	}
	s.palette.Show("Select a theme", items)
}

func (s *Shell) chooseAccent() {
	var items []commandpalette.Item
	for _, a := range theme.Accents {
		id, detail := a.ID, ""
		if s.sel.AccentID == id {
			detail = "current"
		}
		items = append(items, commandpalette.Item{Title: a.Name, Detail: detail, Icon: icons.Circle,
			Run: func() { s.setAccent(id) }})
	}
	s.palette.Show("Select an accent", items)
}

// shortcutLabel renders a shortcut the way the platform writes it.
func shortcutLabel(sc *desktop.CustomShortcut) string {
	var parts []string
	m := sc.Modifier
	if m&fyne.KeyModifierControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if m&fyne.KeyModifierSuper != 0 {
		if runtime.GOOS == "darwin" {
			parts = append(parts, "Cmd")
		} else {
			parts = append(parts, "Super")
		}
	}
	if m&fyne.KeyModifierAlt != 0 {
		parts = append(parts, "Alt")
	}
	if m&fyne.KeyModifierShift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, string(sc.KeyName)), "+")
}
