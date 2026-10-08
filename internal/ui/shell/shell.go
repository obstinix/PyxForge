// Package shell assembles PyxForge's main window: rail, explorer, editor tabs, bottom dock,
// inspector, status bar, command palette, notifications and dialogs.
package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/command"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/explorer"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Version is the PyxForge release this build belongs to.
const Version = "3.0.0-dev"

const (
	prefPalette = "appearance.palette"
	prefAccent  = "appearance.accent"
	prefGlass   = "appearance.glass"
	maxLog      = 500
)

// Shell is one PyxForge window and the state of its workbench.
type Shell struct {
	app  fyne.App
	win  fyne.Window
	root string
	cmds command.Registry
	sel  theme.Selection

	shortcuts []*desktop.CustomShortcut // every keybinding registered on the window

	appearance []func() // run after every theme or accent change

	bench         *workbench
	inspectorAuto bool // the inspector was collapsed by the breakpoint, not by the user
	content       *fyne.Container
	explorer      *explorer.Explorer
	palette       *commandpalette.Palette
	notes         *notifications.Center

	editors     *container.DocTabs
	editorPane  fyne.CanvasObject // the tab bar and its surface; hidden when no tabs are open
	editorEmpty fyne.CanvasObject
	emptyKeys   *fyne.Container               // the empty state's shortcut list, read from the registry
	open        map[string]*container.TabItem // absolute path → tab
	settings    *container.TabItem

	dock     *container.AppTabs
	logLines []string
	logList  *widget.List

	railExplorer, railInspector, railDock   *kit.IconButton
	statusBranch, statusEditor, statusTheme *statusItem
}

// New builds the window for the workspace at root; the caller shows it.
func New(a fyne.App, root string) *Shell {
	s := &Shell{app: a, root: root, open: map[string]*container.TabItem{}}
	s.sel = loadSelection(a.Preferences())
	a.Settings().SetTheme(theme.NewFyne(s.sel))
	s.win = a.NewWindow("PyxForge · " + filepath.Base(root))
	s.content = s.build()
	s.win.SetContent(s.content)
	s.registerCommands()
	s.fillEmptyKeys()
	s.win.Resize(fyne.NewSize(1440, 900))
	s.probeEditor()
	return s
}

// Window is the shell's window.
func (s *Shell) Window() fyne.Window { return s.win }

// Commands is the registry behind the palette and keybindings.
func (s *Shell) Commands() *command.Registry { return &s.cmds }

// Selection is the current appearance choice.
func (s *Shell) Selection() theme.Selection { return s.sel }

// SetSelection applies, persists and announces a new appearance.
func (s *Shell) SetSelection(sel theme.Selection) {
	if sel == s.sel {
		return
	}
	s.sel = sel
	p := s.app.Preferences()
	p.SetString(prefPalette, sel.PaletteID)
	p.SetString(prefAccent, sel.AccentID)
	p.SetBool(prefGlass, sel.Glass)
	s.app.Settings().SetTheme(theme.NewFyne(sel))
	for _, f := range s.appearance {
		f()
	}
	s.statusTheme.SetText(s.themeLabel())
	s.content.Refresh()
	s.logf("Appearance: %s", s.themeLabel())
}

func (s *Shell) onAppearance(f func()) { s.appearance = append(s.appearance, f) }

func loadSelection(p fyne.Preferences) theme.Selection {
	return theme.Selection{
		PaletteID: p.StringWithFallback(prefPalette, theme.Default.PaletteID),
		AccentID:  p.StringWithFallback(prefAccent, theme.Default.AccentID),
		Glass:     p.BoolWithFallback(prefGlass, theme.Default.Glass),
	}
}

func (s *Shell) themeLabel() string {
	t := theme.Current()
	name := t.Name
	if s.sel.PaletteID == theme.SystemID {
		name = "System (" + t.Name + ")"
	}
	a, _ := theme.AccentByID(t.AccentID)
	return name + " · " + a.Name
}

// Notify shows a toast and records it in the Log tab.
func (s *Shell) Notify(l notifications.Level, title, body string) {
	s.notes.Post(l, title, body)
	s.logf("%s %s", title, body)
}

func (s *Shell) logf(format string, args ...any) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	s.logLines = append(s.logLines, line)
	if len(s.logLines) > maxLog {
		s.logLines = s.logLines[len(s.logLines)-maxLog:]
	}
	if s.logList != nil {
		s.logList.Refresh()
	}
}

func (s *Shell) build() *fyne.Container {
	s.notes = notifications.New()
	s.palette = commandpalette.New(s.win.Canvas())
	s.explorer = explorer.New(s.root)
	s.explorer.OnOpen = s.OpenFile
	s.explorer.OnErr = func(err error) { s.Notify(notifications.Error, "Cannot read folder", err.Error()) }

	inspector := s.buildInspector()
	s.bench = &workbench{explorerOn: true, dockOn: true, inspectorOn: true, inspector: inspector}
	// Below the breakpoint the inspector collapses; if that was automatic, it comes back when
	// the window widens again. Fyne lays out at transient sizes during start-up, so a one-way
	// collapse would lose the inspector at full size. An explicit toggle always wins.
	s.bench.onBreakpoint = func(narrow bool) {
		switch {
		case narrow && s.bench.inspectorOn:
			s.bench.inspectorOn, s.inspectorAuto = false, true
		case !narrow && s.inspectorAuto:
			s.bench.inspectorOn, s.inspectorAuto = true, false
		}
		s.syncRail()
	}

	reload := kit.NewIconButton(icons.Refresh, "Reload Explorer", s.explorer.Reload)
	reload.Side = 24
	objs := make([]fyne.CanvasObject, objCount)
	objs[objRail] = s.buildRail()
	objs[objRailRule] = kit.NewRule(true)
	objs[objExplorer] = kit.NewSurface(kit.Base,
		container.NewBorder(header("Explorer", reload), nil, nil, nil, s.explorer.Widget()))
	objs[objExplorerRule] = kit.NewRule(true)
	objs[objEditor] = s.buildEditor()
	objs[objDockRule] = kit.NewRule(false)
	objs[objDock] = s.buildDock()
	objs[objInspector] = inspector
	objs[objStatusRule] = kit.NewRule(false)
	objs[objStatus] = s.buildStatus()
	objs[objNotifications] = s.notes.Layer()
	return container.New(s.bench, objs...)
}

func (s *Shell) buildRail() fyne.CanvasObject {
	btn := func(n icons.Name, label string, run func()) *kit.IconButton {
		b := kit.NewIconButton(n, label, run)
		b.Side, b.IconSize = 40, theme.IconSizeLarge
		return b
	}
	s.railExplorer = btn(icons.FolderTree, "Explorer", s.ToggleExplorer)
	s.railInspector = btn(icons.CPU, "Inspector", s.ToggleInspector)
	s.railDock = btn(icons.PanelBottom, "Panel", s.ToggleDock)
	for _, b := range []*kit.IconButton{s.railExplorer, s.railInspector, s.railDock} {
		b.Marker = kit.LeadingMarker
	}
	s.syncRail()
	top := container.NewVBox(s.railExplorer, btn(icons.Search, "Go to File", s.QuickOpen),
		s.railInspector, s.railDock)
	bottom := container.NewVBox(btn(icons.Settings, "Settings", s.OpenSettings))
	pad := layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, 0, 0)
	return kit.NewSurface(kit.Raised, container.NewBorder(
		container.New(pad, top), container.New(pad, bottom), nil, nil))
}

func (s *Shell) syncRail() {
	s.railExplorer.SetSelected(s.bench.explorerOn)
	s.railInspector.SetSelected(s.bench.inspectorOn)
	s.railDock.SetSelected(s.bench.dockOn)
}

func (s *Shell) buildEditor() fyne.CanvasObject {
	s.editors = container.NewDocTabs()
	s.editors.OnClosed = func(it *container.TabItem) {
		if it == s.settings {
			s.settings = nil
		}
		for p, t := range s.open {
			if t == it {
				delete(s.open, p)
			}
		}
		s.syncEditor()
	}
	s.editorEmpty = kit.NewSurface(kit.Base, s.emptyState())
	s.editorPane = kit.NewSurface(kit.Base, quiet(s.editors))
	stack := container.NewStack(s.editorEmpty, s.editorPane)
	s.syncEditor()
	return stack
}

func (s *Shell) syncEditor() {
	if len(s.editors.Items) == 0 {
		s.editorPane.Hide()
		s.editorEmpty.Show()
	} else {
		s.editorEmpty.Hide()
		s.editorPane.Show()
	}
}

// emptyState is what the editor area shows with no tabs open: real shortcuts, and an honest
// note about the editor.
func (s *Shell) emptyState() fyne.CanvasObject {
	title := kit.NewText("No file open", kit.Display, kit.Primary)
	title.TextSize = theme.TextHeading
	s.emptyKeys = container.New(layout.NewFormLayout())
	note := kit.NewText("The editor is a real Neovim process. It attaches in Phase 3.", kit.Body, kit.Tertiary)
	return container.NewCenter(container.NewVBox(title, s.emptyKeys, note))
}

// emptyStateCommands are the commands the empty state teaches, in order.
var emptyStateCommands = []string{"go.file", "view.commands", "view.explorer", "view.panel", "prefs.settings"}

// fillEmptyKeys lists the empty state's commands with their bindings as registered, so the
// list cannot drift from the keymap.
func (s *Shell) fillEmptyKeys() {
	s.emptyKeys.RemoveAll()
	for _, id := range emptyStateCommands {
		c, ok := s.cmds.Get(id)
		if !ok || c.Keys == "" {
			continue
		}
		keys := kit.NewText(c.Keys, kit.Mono, kit.Tertiary)
		keys.TextSize = theme.TextCaption + 1
		s.emptyKeys.Add(kit.NewText(c.Title, kit.Body, kit.Secondary))
		s.emptyKeys.Add(keys)
	}
}

// OpenFile opens a tab for a file, or selects it if it is already open.
func (s *Shell) OpenFile(p string) {
	if t, ok := s.open[p]; ok {
		s.editors.Select(t)
		return
	}
	info, err := os.Stat(p)
	if err != nil {
		s.Notify(notifications.Error, "Cannot open file", err.Error())
		return
	}
	t := container.NewTabItem(filepath.Base(p), s.fileView(p, info))
	s.open[p] = t
	s.editors.Append(t)
	s.editors.Select(t)
	s.syncEditor()
}

// fileView holds an open file's place until Neovim renders it (Phase 3). It shows real file
// facts and says plainly that editing is not available yet.
func (s *Shell) fileView(p string, info os.FileInfo) fyne.CanvasObject {
	rel, err := filepath.Rel(s.root, p)
	if err != nil {
		rel = p
	}
	facts := container.NewVBox(
		kit.NewText(filepath.ToSlash(rel), kit.Mono, kit.Secondary),
		kit.NewText(fmt.Sprintf("%s · modified %s", humanSize(info.Size()),
			info.ModTime().Format("2006-01-02 15:04")), kit.Body, kit.Tertiary),
	)
	return container.NewBorder(container.NewPadded(facts), nil, nil, nil,
		placeholder(icons.FileCode, "Neovim is not attached",
			"This tab keeps the file's place. Editing arrives with Neovim in Phase 3."))
}

func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

func (s *Shell) buildDock() fyne.CanvasObject {
	s.logList = widget.NewList(
		func() int { return len(s.logLines) },
		func() fyne.CanvasObject {
			t := kit.NewText("", kit.Mono, kit.Secondary)
			t.TextSize = theme.TextCaption + 1
			return t
		},
		func(i widget.ListItemID, o fyne.CanvasObject) { o.(*kit.Text).SetText(s.logLines[i]) },
	)
	s.dock = container.NewAppTabs(
		container.NewTabItem("Terminal", placeholder(icons.Terminal, "No terminal sessions",
			"Shell, qemu-serial and gdb-server sessions arrive in Phase 4.")),
		container.NewTabItem("Build", placeholder(icons.Hammer, "No builds yet",
			"Profiles from pyxforge.toml run here in Phase 4.")),
		container.NewTabItem("Problems", placeholder(icons.TriangleAlert, "No problems",
			"Compiler and linker diagnostics appear here once builds run in Phase 4.")),
		container.NewTabItem("QEMU", placeholder(icons.Server, "QEMU is not running",
			"Launch, QMP state, snapshots and the monitor console arrive in Phase 5.")),
		container.NewTabItem("GDB", placeholder(icons.Bug, "No debug session",
			"GDB/MI sessions arrive in Phase 5.")),
		container.NewTabItem("Log", s.logList),
	)
	return kit.NewSurface(kit.Sunken, quiet(s.dock))
}

func (s *Shell) buildInspector() *sidePanel {
	tabs := container.NewAppTabs(
		container.NewTabItem("Registers", placeholder(icons.CPU, "No debug session",
			"Registers with change highlighting arrive with GDB/MI in Phase 5.")),
		container.NewTabItem("Flags", placeholder(icons.Flag, "No debug session",
			"Flag bits arrive with GDB/MI in Phase 5.")),
		container.NewTabItem("Hex", placeholder(icons.Binary, "No binary selected",
			"Hex, ASCII and the boot-signature check arrive in Phase 5.")),
		container.NewTabItem("Disasm", placeholder(icons.List, "No debug session",
			"Disassembly with the current instruction arrives in Phase 5.")),
		container.NewTabItem("Memory", placeholder(icons.MemoryStick, "No debug session",
			"Memory views arrive in Phase 5.")),
	)
	hide := kit.NewIconButton(icons.X, "Hide Inspector", s.ToggleInspector)
	hide.Side = 24
	return newSidePanel(container.NewBorder(header("Inspector", hide), nil, nil, nil, quiet(tabs)))
}

func (s *Shell) buildStatus() fyne.CanvasObject {
	branch := "no repository"
	if b, ok := gitBranch(s.root); ok {
		branch = b
	}
	s.statusBranch = newStatusItem(icons.GitBranch, branch, nil)
	s.statusEditor = newStatusItem(icons.FileCode, "Neovim: not attached", nil)
	s.statusTheme = newStatusItem(icons.Palette, s.themeLabel(), s.OpenSettings)
	return kit.NewSurface(kit.Sunken, container.NewHBox(
		s.statusBranch, newStatusItem(icons.Folder, filepath.Base(s.root), nil), s.statusEditor,
		layout.NewSpacer(), s.statusTheme))
}

// probeEditor records whether Neovim is installed. It never starts it: that is Phase 3.
func (s *Shell) probeEditor() {
	p, err := exec.LookPath("nvim")
	if err != nil {
		s.statusEditor.SetText("Neovim: not found")
		s.logf("Neovim was not found on PATH; the editor needs it from Phase 3.")
		return
	}
	s.logf("Neovim found at %s; it attaches in Phase 3.", p)
}

// ToggleExplorer shows or hides the explorer.
func (s *Shell) ToggleExplorer() {
	s.bench.explorerOn = !s.bench.explorerOn
	s.relayout()
}

// ToggleDock shows or hides the bottom panel.
func (s *Shell) ToggleDock() {
	s.bench.dockOn = !s.bench.dockOn
	s.relayout()
}

// ToggleInspector shows or hides the inspector, docked or floating by window width.
func (s *Shell) ToggleInspector() {
	s.inspectorAuto = false
	s.bench.inspectorOn = !s.bench.inspectorOn
	s.relayout()
}

func (s *Shell) relayout() {
	s.syncRail()
	s.content.Refresh()
}

// ShowCommands opens the command palette over every registered command.
func (s *Shell) ShowCommands() {
	s.palette.Show("Run a command", commandpalette.FromCommands(s.cmds.All()))
}

// QuickOpen opens the palette over the workspace's files.
func (s *Shell) QuickOpen() {
	files := s.explorer.Files(5000)
	items := make([]commandpalette.Item, len(files))
	for i, f := range files {
		abs := filepath.Join(s.root, filepath.FromSlash(f))
		dir := path.Dir(f)
		if dir == "." {
			dir = ""
		}
		items[i] = commandpalette.Item{Title: path.Base(f), Detail: dir, Icon: icons.File,
			Run: func() { s.OpenFile(abs) }}
	}
	s.palette.Show("Go to a file by name", items)
}

// Palette exposes the command palette, for tests and review renders.
func (s *Shell) Palette() *commandpalette.Palette { return s.palette }

// OpenSettings opens Settings → Appearance as an editor tab.
func (s *Shell) OpenSettings() {
	if s.settings == nil {
		s.settings = container.NewTabItem("Settings", s.settingsView())
		s.editors.Append(s.settings)
	}
	s.editors.Select(s.settings)
	s.syncEditor()
}

func (s *Shell) closeEditor() {
	if t := s.editors.Selected(); t != nil {
		s.editors.Remove(t)
		s.editors.OnClosed(t)
	}
}
