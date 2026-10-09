package shell

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/obstinix/PyxForge/internal/command"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// registerCommands registers every action once. Only commands that do something real today
// are registered; later phases add theirs. Keybindings are Ctrl+Shift chords only (K1).
func (s *Shell) registerCommands() {
	add := func(id, category, title string, sc *desktop.CustomShortcut, run func()) {
		keys := ""
		if sc != nil {
			keys = shortcutLabel(sc)
			s.win.Canvas().AddShortcut(sc, func(fyne.Shortcut) { run() })
			s.shortcuts = append(s.shortcuts, sc)
			s.chordRuns[sc.ShortcutName()] = run
		}
		s.cmds.Add(command.Command{ID: id, Title: title, Category: category, Keys: keys, Run: run})
	}

	add("view.commands", "View", "Show All Commands", chord(fyne.KeyP), s.ShowCommands)
	add("go.file", "Go", "Go to File", chord(fyne.KeyO), s.QuickOpen)
	add("view.explorer", "View", "Toggle Explorer", chord(fyne.KeyE), s.ToggleExplorer)
	add("view.panel", "View", "Toggle Panel", chord(fyne.KeyJ), s.ToggleDock)
	add("view.inspector", "View", "Toggle Inspector", chord(fyne.KeyI), s.ToggleInspector)
	add("view.closeEditor", "View", "Close Editor Tab", chord(fyne.KeyW), s.closeEditor)
	add("file.save", "File", "Save", chord(fyne.KeyS), s.saveCurrent)
	add("file.saveAll", "File", "Save All", nil, s.saveAll)
	add("build.run", "Build", "Build", chord(fyne.KeyB), func() {
		s.showDockTab(slices.Index(s.dock.Items, s.buildp.tab))
		s.buildp.start()
	})
	add("build.stop", "Build", "Stop Build", nil, s.buildp.halt)
	add("build.profile", "Build", "Choose Build Profile", nil, s.chooseBuildProfile)
	add("git.refresh", "Git", "Refresh Status", nil, s.gitp.refresh)
	add("git.stageAll", "Git", "Stage All Changes", nil, func() { s.gitp.apply("stage", true, nil) })
	add("git.commit", "Git", "Commit…", nil, func() {
		s.showDockTab(slices.Index(s.dock.Items, s.gitp.tab))
		s.gitp.refresh()
		s.win.Canvas().Focus(s.gitp.message)
	})
	add("terminal.restart", "Terminal", "Restart Terminal", nil, func() {
		s.showDockTab(slices.Index(s.dock.Items, s.term.tab))
		s.term.restart()
	})
	add("explorer.reload", "Explorer", "Reload", nil, s.explorer.Reload)
	add("explorer.newFile", "Explorer", "New File…", nil, func() { s.newEntry(false) })
	add("explorer.newFolder", "Explorer", "New Folder…", nil, func() { s.newEntry(true) })
	add("explorer.reveal", "Explorer", "Reveal Active File", nil, s.revealActive)
	add("tools.check", "Tools", "Check Toolchain", nil, func() { s.checkTools(true) })
	add("view.nextTab", "View", "Next Tab", chord(fyne.KeyPageDown), func() { s.cycleTab(1) })
	add("view.previousTab", "View", "Previous Tab", chord(fyne.KeyPageUp), func() { s.cycleTab(-1) })
	add("view.focusExplorer", "View", "Focus Explorer", nil, s.FocusExplorer)
	add("view.focusEditor", "View", "Focus Editor", nil, s.FocusEditor)
	add("view.focusPanel", "View", "Focus Panel", nil, s.FocusPanel)
	add("view.focusInspector", "View", "Focus Inspector", nil, s.FocusInspector)
	for i, it := range s.dock.Items {
		add("panel."+strings.ToLower(it.Text), "Panel", "Show "+it.Text, nil, func() { s.showDockTab(i) })
	}
	for i, it := range s.inspectorTabs.Items {
		add("inspector."+strings.ToLower(it.Text), "Inspector", "Show "+it.Text, nil, func() { s.showInspectorTab(i) })
	}
	add("prefs.settings", "Preferences", "Open Settings", chord(fyne.KeyComma), s.openSettingsFocused)
	add("prefs.theme", "Preferences", "Change Theme", nil, s.chooseTheme)
	add("prefs.accent", "Preferences", "Change Accent", nil, s.chooseAccent)
	// One command per theme and accent, grouped so "theme" finds Change Theme first and
	// "theme mono" finds Monochrome.
	add("prefs.theme."+theme.SystemID, "Theme", "System", nil, func() { s.setPalette(theme.SystemID) })
	for _, p := range theme.Palettes {
		id := p.ID
		add("prefs.theme."+id, "Theme", p.Name, nil, func() { s.setPalette(id) })
	}
	for _, a := range theme.Accents {
		id := a.ID
		add("prefs.accent."+id, "Accent", a.Name, nil, func() { s.setAccent(id) })
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

// chooseBuildProfile lists pyxforge.toml's profiles in the palette; choosing one builds it.
func (s *Shell) chooseBuildProfile() {
	if !s.buildp.reload() {
		s.showDockTab(slices.Index(s.dock.Items, s.buildp.tab))
		return
	}
	var items []commandpalette.Item
	for _, name := range s.buildp.pick.Options {
		detail := ""
		if name == s.buildp.pick.Selected {
			detail = "current"
		} else if p, ok := s.buildp.cfg.Profiles[name]; ok {
			detail = p.Description
		}
		items = append(items, commandpalette.Item{Title: name, Detail: detail, Icon: icons.Hammer, Run: func() {
			s.buildp.pick.SetSelected(name)
			s.showDockTab(slices.Index(s.dock.Items, s.buildp.tab))
			s.buildp.start()
		}})
	}
	s.palette.Show("Select a build profile", items)
}
