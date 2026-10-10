package shell

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/obstinix/PyxForge/internal/command"
	"github.com/obstinix/PyxForge/internal/git"
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
	add("problems.next", "Go", "Next Problem", chord(fyne.KeyF8), func() { s.stepProblem(1) })
	add("problems.previous", "Go", "Previous Problem", chord(fyne.KeyF7), func() { s.stepProblem(-1) })
	add("build.profile", "Build", "Choose Build Profile", nil, s.chooseBuildProfile)
	add("git.refresh", "Git", "Refresh Status", nil, s.gitp.refresh)
	add("git.stageAll", "Git", "Stage All Changes", nil, func() { s.gitp.apply("stage", true, nil) })
	add("git.switchBranch", "Git", "Switch Branch…", nil, s.gitp.chooseBranch)
	add("git.createBranch", "Git", "Create Branch…", nil, s.gitp.createBranch)
	add("git.deleteBranch", "Git", "Delete Branch…", nil, s.gitp.deleteBranch)
	add("git.stash", "Git", "Stash Changes…", nil, s.gitp.stashChanges)
	add("git.stashApply", "Git", "Apply Stash…", nil, func() {
		s.gitp.chooseStash("Apply a stash (it is kept)", func(st git.Stash) { s.gitp.applyStash(st.Ref, false) })
	})
	add("git.stashPop", "Git", "Pop Stash…", nil, func() {
		s.gitp.chooseStash("Pop a stash (applied, then removed)", func(st git.Stash) { s.gitp.applyStash(st.Ref, true) })
	})
	add("git.stashDrop", "Git", "Drop Stash…", nil, func() { s.gitp.chooseStash("Drop a stash", s.gitp.dropStash) })
	add("git.commit", "Git", "Commit…", nil, func() {
		s.showDockTab(slices.Index(s.dock.Items, s.gitp.tab))
		s.gitp.refresh()
		s.win.Canvas().Focus(s.gitp.message)
	})
	add("run.run", "Run", "Run in QEMU", chord(fyne.KeyR), func() { s.mach.start(false) })
	add("run.debug", "Run", "Debug in QEMU", chord(fyne.KeyD), func() { s.mach.start(true) })
	add("run.stop", "Run", "Stop QEMU", chord(fyne.KeyF2), s.mach.halt)
	add("debug.continue", "Debug", "Continue", chord(fyne.KeyF5), s.mach.cont)
	add("debug.pause", "Debug", "Pause", nil, func() {
		if !s.mach.paused {
			s.mach.togglePause()
		}
	})
	add("debug.stepInstruction", "Debug", "Step Instruction", chord(fyne.KeyF11), s.mach.stepInstruction)
	add("debug.nextInstruction", "Debug", "Step Over Instruction", chord(fyne.KeyF10), s.mach.nextInstruction)
	add("inspector.disassembleAs", "Inspector", "Disassemble As…", nil, s.mviews.disassembleAs)
	add("snapshot.capture", "Debug", "Capture Snapshot…", nil, s.snaps.capture)
	add("snapshot.compare", "Debug", "Compare Snapshots…", nil, func() {
		s.snaps.refresh()
		s.snaps.chooseCompare()
	})
	add("machine.save", "Run", "Save Machine State…", nil, s.snaps.saveMachine)
	add("machine.restore", "Run", "Restore Machine State…", nil, func() { s.snaps.chooseMachine("Restore a machine state", s.snaps.restoreMachine) })
	add("machine.delete", "Run", "Delete Machine State…", nil, func() { s.snaps.chooseMachine("Delete a machine state", s.snaps.deleteMachine) })
	add("debug.breakpoint", "Debug", "Add Breakpoint…", nil, s.mach.addBreakpoint)
	add("qemu.monitor", "Run", "QEMU Monitor Command…", nil, func() {
		s.mach.showTab()
		s.win.Canvas().Focus(s.mach.monitor)
	})
	showTerm := func(run func()) func() {
		return func() {
			s.showDockTab(slices.Index(s.dock.Items, s.term.tab))
			run()
		}
	}
	add("terminal.new", "Terminal", "New Terminal", chord(fyne.KeyBackTick), showTerm(s.term.newSession))
	add("terminal.switch", "Terminal", "Switch Terminal…", nil, s.term.chooseSession)
	add("terminal.next", "Terminal", "Next Terminal", chord(fyne.KeyRightBracket), showTerm(func() { s.term.cycle(1) }))
	add("terminal.previous", "Terminal", "Previous Terminal", chord(fyne.KeyLeftBracket), showTerm(func() { s.term.cycle(-1) }))
	add("terminal.rename", "Terminal", "Rename Terminal…", nil, s.term.renameCurrent)
	add("terminal.restart", "Terminal", "Restart Terminal", nil, showTerm(s.term.restart))
	add("terminal.close", "Terminal", "Close Terminal", nil, s.term.closeCurrent)
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
	panelKeys := map[string]fyne.KeyName{"Problems": fyne.KeyM, "Git": fyne.KeyG}
	for i, it := range s.dock.Items {
		var sc *desktop.CustomShortcut
		if k, ok := panelKeys[it.Text]; ok {
			sc = chord(k)
		}
		add("panel."+strings.ToLower(it.Text), "Panel", "Show "+it.Text, sc, func() { s.showDockTab(i) })
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
