package shell

import (
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// The menus (PXF-DEF-009, PXF-DEF-019) are PyxForge's own pop-up menus under a row of titles,
// not Fyne's main menu: a main menu runs its items' shortcuts before the focused widget sees
// the key, which would take Ctrl+W from a shell in the terminal and ignore "Neovim keys"
// (KEYMAP.md). Here the shortcuts are only shown; keys go through the keyboard policy. Every
// item is a registered command, so the menus, the palette and the keys cannot disagree, and
// the menus are built when they open, so what is unavailable is greyed out.

// menuSep separates groups in a menu.
const menuSep = "-"

// menus lists each menu's commands by ID, in order.
var menus = []struct {
	title string
	items []string
}{
	{"File", []string{"file.new", "file.open", "file.openFolder", "file.recent", menuSep,
		"file.save", "file.saveAs", "file.saveAll", menuSep, "view.closeEditor", "view.closeAll", menuSep, "app.exit"}},
	{"Edit", []string{"edit.undo", "edit.redo", menuSep, "edit.cut", "edit.copy", "edit.paste", "edit.selectAll",
		menuSep, "edit.find", "edit.replace"}},
	{"View", []string{"view.commands", menuSep, "view.explorer", "view.panel", "view.inspector", menuSep,
		"view.nextTab", "view.previousTab", menuSep, "view.focusExplorer", "view.focusEditor", "view.focusPanel",
		"view.focusInspector", menuSep, "prefs.settings", "prefs.neovimKeys"}},
	{"Go", []string{"go.file", menuSep, "problems.next", "problems.previous"}},
	{"Git", []string{"git.fetch", "git.pull", "git.push", menuSep, "git.commit", "git.stageAll", menuSep,
		"git.switchBranch", "git.createBranch", "git.addRemote", menuSep, "git.abort", "git.init", menuSep, "git.refresh"}},
	{"Run", []string{"run.file", "run.debugFile", "debug.toggleBreakpoint", menuSep, "build.run", "build.stop", "build.profile", menuSep, "run.run", "run.debug", "run.stop",
		"machine.setup", menuSep, "terminal.new", menuSep, "tools.check", "tools.doctor"}},
	{"Help", []string{"help.keys", "help.docs", menuSep, "help.about"}},
}

// buildMenuBar is the row of menu titles above the workbench.
func (s *Shell) buildMenuBar() fyne.CanvasObject {
	row := container.NewHBox()
	for _, m := range menus {
		title, items := m.title, m.items
		var b *widget.Button
		b = widget.NewButton(title, func() { s.showMenu(title, items, b) })
		b.Importance = widget.LowImportance
		row.Add(b)
	}
	s.menuBar = row
	return container.NewVBox(row, kit.NewRule(false))
}

// showMenu opens a menu under its title. The menu sits on PyxForge's overlay material: on
// the Windows build Fyne's own pop-up menu let the workspace show through its items.
func (s *Shell) showMenu(title string, ids []string, under fyne.CanvasObject) {
	m := widget.NewMenu(s.menu(title, ids))
	g := kit.NewGlass(m)
	g.Padding = 0
	pop := widget.NewPopUp(g, s.win.Canvas())
	m.OnDismiss = pop.Hide
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(under).AddXY(0, under.Size().Height)
	pop.ShowAtPosition(pos)
	pop.Resize(g.MinSize())
	s.openMenu = pop
}

// menu builds one menu from the registry, as it is now.
func (s *Shell) menu(title string, ids []string) *fyne.Menu {
	var items []*fyne.MenuItem
	for _, id := range ids {
		if id == menuSep {
			items = append(items, fyne.NewMenuItemSeparator())
			continue
		}
		c, ok := s.cmds.Get(id)
		if !ok {
			continue // a command a build left out
		}
		it := fyne.NewMenuItem(strings.TrimSuffix(c.Title, " PyxForge"), c.Run)
		if sc, ok := s.cmdKeys[id]; ok {
			it.Shortcut = sc // shown only; see the comment above
		}
		it.Disabled = !s.available(id)
		items = append(items, it)
	}
	return fyne.NewMenu(title, items...)
}

// available reports whether a command can do something now; menus grey out the rest.
func (s *Shell) available(id string) bool {
	switch id {
	case "file.save", "file.saveAs", "edit.find", "edit.replace", "run.file", "run.debugFile", "debug.toggleBreakpoint":
		return s.activeBuffer() != nil
	case "file.saveAll":
		return s.ed != nil && s.ed.sess != nil
	case "view.closeEditor", "view.closeAll", "view.nextTab", "view.previousTab":
		return len(s.editors.Items) > 0
	case "edit.undo", "edit.redo", "edit.cut", "edit.copy", "edit.paste", "edit.selectAll":
		return s.editTarget() != nil
	case "build.stop":
		return s.buildp.running
	case "git.fetch", "git.pull", "git.push":
		return s.gitp.repo != nil && s.gitp.netBusy == ""
	case "git.commit", "git.stageAll", "git.switchBranch", "git.createBranch", "git.addRemote":
		return s.gitp.repo != nil
	case "git.init":
		return s.gitp.repo == nil
	case "git.abort":
		return s.gitp.op != ""
	case "file.recent":
		return len(s.app.Preferences().StringList(prefRecentFiles))+len(s.recentFolders()) > 0
	}
	return true
}

// editTarget is what the Edit menu's clipboard and undo items act on: the editor, or a text
// field, when it has keyboard focus; nil otherwise.
func (s *Shell) editTarget() fyne.Focusable {
	f := s.win.Canvas().Focused()
	if f == nil {
		return nil
	}
	if s.ed != nil && s.ed.sess != nil && f == s.ed.view {
		return f
	}
	if _, ok := f.(fyne.Shortcutable); ok {
		if _, isView := f.(interface{ FlushHook() }); !isView { // the terminal's view is no text field
			return f
		}
	}
	return nil
}

// editAction runs an Edit menu item on the focused editor or text field. In the editor it is
// Neovim's own: undo, redo, the system clipboard register for cut, copy and paste (the line
// when nothing is selected), and select all as a visual selection of the file.
func (s *Shell) editAction(kind string) {
	f := s.editTarget()
	if f == nil {
		return
	}
	if s.ed != nil && f == s.ed.view {
		sess := s.ed.sess
		visual := strings.HasPrefix(sess.Grid().Mode, "visual")
		insert := strings.HasPrefix(sess.Grid().Mode, "insert")
		keys := map[string]string{
			"undo": "<Cmd>undo<CR>", "redo": "<Cmd>redo<CR>", "selectAll": "<Esc>ggVG",
			"copy": `<Esc>"+yy`, "cut": `<Esc>"+dd`, "paste": `<Esc>"+P`,
		}
		switch {
		case visual && kind == "copy":
			keys["copy"] = `"+y`
		case visual && kind == "cut":
			keys["cut"] = `"+d`
		case visual && kind == "paste":
			keys["paste"] = `"+P`
		case insert && kind == "paste":
			keys["paste"] = "<C-r><C-o>+"
		}
		sess.Input(keys[kind])
		return
	}
	cb := s.app.Clipboard()
	var sc fyne.Shortcut
	switch kind {
	case "undo":
		sc = &fyne.ShortcutUndo{}
	case "redo":
		sc = &fyne.ShortcutRedo{}
	case "cut":
		sc = &fyne.ShortcutCut{Clipboard: cb}
	case "copy":
		sc = &fyne.ShortcutCopy{Clipboard: cb}
	case "paste":
		sc = &fyne.ShortcutPaste{Clipboard: cb}
	case "selectAll":
		sc = &fyne.ShortcutSelectAll{}
	}
	f.(fyne.Shortcutable).TypedShortcut(sc)
}

// openURL opens a page in the user's browser, saying so if it cannot.
func (s *Shell) openURL(raw string) {
	u, err := url.Parse(raw)
	if err == nil {
		err = s.app.OpenURL(u)
	}
	if err != nil {
		s.Notify(notifications.Error, "Cannot open the page", raw+": "+err.Error())
	}
}
