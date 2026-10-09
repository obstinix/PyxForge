// Package shell assembles PyxForge's main window: rail, explorer, editor tabs, bottom dock,
// inspector, status bar, command palette, notifications and dialogs.
package shell

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/command"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/explorer"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/workspace"
)

const (
	prefPalette = "appearance.palette"
	prefAccent  = "appearance.accent"
	prefGlass   = "appearance.glass"
	maxLog      = 500
	// quickOpenLimit bounds the file walk so Go to File opens instantly in huge trees.
	quickOpenLimit = 20000
)

// Shell is one PyxForge window and the state of its workbench.
type Shell struct {
	app  fyne.App
	win  fyne.Window
	root string
	cmds command.Registry
	sel  theme.Selection

	shortcuts []*desktop.CustomShortcut // every keybinding registered on the window

	probe func(context.Context) []toolchain.Status
	tools []toolchain.Status // the last toolchain detection, for the Log and later the build service

	// settingsHooks refresh the open Settings view after an appearance change. They are
	// rebuilt with the view, so reopening Settings never keeps the old view's widgets alive.
	settingsHooks []func()

	bench         *workbench
	inspectorAuto bool // the inspector was collapsed by the breakpoint, not by the user
	content       *fyne.Container
	explorer      *explorer.Explorer
	explorerFrame *kit.FocusFrame
	palette       *commandpalette.Palette
	notes         *notifications.Center

	active    region // holds keyboard focus or was last used; its active tab takes the accent
	tabThemes map[region]*container.ThemeOverride

	editors     *container.DocTabs
	editorPane  fyne.CanvasObject // the tab bar and its surface; hidden when no tabs are open
	editorEmpty fyne.CanvasObject
	emptyKeys   *fyne.Container               // the empty state's shortcut list, read from the registry
	open        map[string]*container.TabItem // absolute path → tab
	settings    *container.TabItem
	themeCards  []*themeCard

	dock          *container.AppTabs
	logLines      []string
	logList       *logList
	logFrame      *kit.FocusFrame
	logTab        *container.TabItem
	inspectorTabs *container.AppTabs

	railExplorer, railInspector, railDock *kit.IconButton
	statusBranch, statusEditor            *statusItem
	statusTheme                           *statusAction

	// The editor: one embedded Neovim for the workspace, started on the first open.
	ed            *editorHost
	editorEnabled bool
	nvimPath      string
	nvimRuntime   string
	nvimEnv       []string
	chordRuns     map[string]func() // shell commands by shortcut name, for keys the editor hands back
	dispatch      func(func())      // runs work on the UI thread
	lastDialog    dialog.Dialog     // the most recent confirmation, for tests

	problems      []problem
	problemList   *widget.List
	problemsEmpty fyne.CanvasObject
	problemsTab   *container.TabItem

	term   *terminalHost
	buildp *buildPanel
	states *workspace.StateStore
}

// Options adjust a shell for tests and review renders.
type Options struct {
	// Probe detects the toolchain in the background after the window is built. Nil skips
	// detection, so tests and renders stay deterministic.
	Probe func(context.Context) []toolchain.Status
	// Editor opens files in an embedded Neovim. Off, files open as placeholders.
	Editor bool
	// NvimPath is the Neovim executable; empty searches PATH.
	NvimPath string
	// NvimRuntime is where PyxForge's Neovim configuration is installed; empty means the
	// user cache folder. NvimEnv is extra environment for Neovim (tests isolate XDG folders).
	NvimRuntime string
	NvimEnv     []string
	// Dispatch runs work on the UI thread; nil means fyne.Do. Tests pass a queue they drain,
	// because Fyne's test driver runs fyne.Do on the calling goroutine.
	Dispatch func(func())
	// State remembers the workspace's layout and open files between sessions; nil keeps none.
	State *workspace.StateStore
	// WatchFiles makes the explorer follow changes on disk.
	WatchFiles bool
}

// New builds the window for the workspace at root; the caller shows it. It checks the
// toolchain in the background and reports what it finds in the status bar and the Log, and
// opens files in an embedded Neovim.
func New(a fyne.App, root string) *Shell {
	o := Options{Probe: toolchain.Detect, Editor: true, WatchFiles: true}
	if st, err := workspace.DefaultStateStore(); err == nil {
		o.State = &st
	}
	return NewWithOptions(a, root, o)
}

// NewWithOptions is New with explicit options.
func NewWithOptions(a fyne.App, root string, opts Options) *Shell {
	s := &Shell{app: a, root: root, open: map[string]*container.TabItem{},
		tabThemes: map[region]*container.ThemeOverride{}, probe: opts.Probe,
		editorEnabled: opts.Editor, nvimPath: opts.NvimPath, nvimRuntime: opts.NvimRuntime, nvimEnv: opts.NvimEnv,
		dispatch: opts.Dispatch, chordRuns: map[string]func(){}, states: opts.State}
	if s.dispatch == nil {
		s.dispatch = fyne.Do
	}
	s.sel = loadSelection(a.Preferences())
	a.Settings().SetTheme(theme.NewFyne(s.sel))
	s.win = a.NewWindow("PyxForge · " + filepath.Base(root))
	s.content = s.build()
	s.win.SetContent(s.content)
	s.registerCommands()
	s.fillEmptyKeys()
	s.win.Resize(fyne.NewSize(1440, 900))
	s.win.SetCloseIntercept(s.confirmQuit)
	if opts.WatchFiles {
		if err := s.explorer.Watch(s.dispatch); err != nil {
			s.logf("The explorer will not follow changes on disk: %v", err)
		}
	}
	s.restoreState()
	s.checkTools(false)
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
	s.syncEditorTheme()
	for _, f := range s.settingsHooks {
		f()
	}
	s.statusTheme.SetText(s.themeLabel())
	s.content.Refresh()
	s.logf("Appearance: %s", s.themeLabel())
}

func (s *Shell) onAppearance(f func()) { s.settingsHooks = append(s.settingsHooks, f) }

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
	s.notes.Dispatch = s.dispatch
	s.palette = commandpalette.New(s.win.Canvas())
	s.explorer = explorer.New(s.root)
	s.explorerFrame = kit.NewFocusFrame(s.explorer.Widget())
	s.explorer.OnOpen = func(p string) {
		s.OpenFile(p)
		if s.explorerFrame.Focused() { // opening from the tree keeps the user in the tree
			s.activate(regionExplorer)
		}
	}
	s.explorer.OnErr = func(err error) { s.Notify(notifications.Error, "Cannot read folder", err.Error()) }
	s.explorer.OnFocus = func(on bool) {
		s.explorerFrame.SetFocused(on)
		switch {
		case on:
			s.activate(regionExplorer)
		case s.active == regionExplorer:
			s.activate(regionEditor)
		}
	}

	inspector := s.buildInspector()
	s.bench = &workbench{explorerOn: true, dockOn: true, inspectorOn: true, inspector: inspector}
	// Below the breakpoint the inspector collapses; if that was automatic, it comes back when
	// the window widens again. Fyne lays out at transient sizes during start-up, so a one-way
	// collapse would lose the inspector at full size. An explicit toggle always wins.
	s.bench.onBreakpoint = func(narrow bool) {
		switch {
		case narrow && s.bench.inspectorOn:
			s.bench.inspectorOn, s.inspectorAuto = false, true
			if s.active == regionInspector {
				s.activate(regionEditor)
			}
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
		container.NewBorder(header("Explorer", reload), nil, nil, nil, s.explorerFrame))
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
	s.editors.OnSelected = func(it *container.TabItem) {
		s.activate(regionEditor)
		if s.ed != nil && s.ed.sess != nil {
			s.ed.selected(it)
		}
	}
	// A buffer tab closes through Neovim, which asks about unsaved changes first; other tabs
	// (Settings, placeholders) close at once.
	s.editors.CloseIntercept = func(it *container.TabItem) {
		if s.ed != nil && s.ed.sess != nil {
			if bt := s.ed.byTab[it]; bt != nil {
				s.ed.close(bt)
				return
			}
		}
		s.editors.Remove(it)
		s.editors.OnClosed(it)
	}
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
	s.editorPane = kit.NewSurface(kit.Base, s.regionTabs(regionEditor, s.editors))
	stack := container.NewStack(s.editorEmpty, s.editorPane)
	s.syncEditor()
	return newRegionArea(stack, func() { s.activate(regionEditor) })
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
	note := kit.NewText("Files open in an embedded Neovim: every key but the Ctrl+Shift chords goes to Vim.", kit.Body, kit.Tertiary)
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

// OpenFile opens a file in the editor, or selects its tab if it is already open. Without
// Neovim the file gets a placeholder tab that says how to enable editing.
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
	if s.startEditor() {
		s.activate(regionEditor)
		s.ed.open(p)
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
	why := "Install Neovim 0.9 or newer to edit files; pyxforge doctor shows how."
	switch {
	case !s.editorEnabled:
		why = "Editing is turned off in this window."
	case s.ed != nil && s.ed.err != nil:
		why = "Neovim could not start: " + s.ed.err.Error()
	}
	return container.NewBorder(container.NewPadded(facts), nil, nil, nil,
		placeholder(icons.FileCode, "Neovim is not available", why))
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
	s.logList = &logList{onFocus: func(on bool) {
		s.logFrame.SetFocused(on)
		if on {
			s.activate(regionPanel)
		}
	}}
	s.logList.Length = func() int { return len(s.logLines) }
	s.logList.CreateItem = func() fyne.CanvasObject {
		t := kit.NewText("", kit.Mono, kit.Secondary)
		t.TextSize = theme.TextCaption + 1
		// Inset clear of the focus ring, on the same 8 px edge as the panel headers.
		return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space2), t)
	}
	s.logList.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) {
		o.(*fyne.Container).Objects[0].(*kit.Text).SetText(s.logLines[i])
	}
	s.logList.ExtendBaseWidget(s.logList)
	s.logFrame = kit.NewFocusFrame(s.logList)
	s.logTab = container.NewTabItem("Log", s.logFrame)
	s.dock = container.NewAppTabs(
		s.buildTerminal(),
		s.buildBuildPanel(),
		s.buildProblems(),
		container.NewTabItem("QEMU", placeholder(icons.Server, "QEMU is not running",
			"Launch, QMP state, snapshots and the monitor console arrive in Phase 5.")),
		container.NewTabItem("GDB", placeholder(icons.Bug, "No debug session",
			"GDB/MI sessions arrive in Phase 5.")),
		s.logTab,
	)
	s.dock.OnSelected = func(it *container.TabItem) {
		s.activate(regionPanel)
		switch it {
		case s.term.tab:
			s.term.start()
			s.term.focus()
		case s.buildp.tab:
			if !s.buildp.running {
				s.buildp.reload()
			}
		}
	}
	return kit.NewSurface(kit.Sunken,
		newRegionArea(s.regionTabs(regionPanel, s.dock), func() { s.activate(regionPanel) }))
}

func (s *Shell) buildInspector() *sidePanel {
	s.inspectorTabs = container.NewAppTabs(
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
	s.inspectorTabs.OnSelected = func(*container.TabItem) { s.activate(regionInspector) }
	hide := kit.NewIconButton(icons.X, "Hide Inspector", s.ToggleInspector)
	hide.Side = 24
	body := container.NewBorder(header("Inspector", hide), nil, nil, nil,
		s.regionTabs(regionInspector, s.inspectorTabs))
	return newSidePanel(newRegionArea(body, func() { s.activate(regionInspector) }))
}

func (s *Shell) buildStatus() fyne.CanvasObject {
	branch := "no repository"
	if b, ok := workspace.GitBranch(s.root); ok {
		branch = b
	}
	s.statusBranch = newStatusItem(icons.GitBranch, branch)
	s.statusEditor = newStatusItem(icons.FileCode, "Neovim: not attached")
	s.statusTheme = newStatusAction(icons.Palette, s.themeLabel(), s.openSettingsFocused)
	return kit.NewSurface(kit.Sunken, container.NewHBox(
		s.statusBranch, newStatusItem(icons.Folder, filepath.Base(s.root)), s.statusEditor,
		layout.NewSpacer(), s.statusTheme))
}

// checkTools detects the toolchain off the UI thread, as `pyxforge doctor` does, and reports
// it in the status bar and the Log. With notify it also posts a summary notification, for
// when the user asked.
func (s *Shell) checkTools(notify bool) {
	if s.probe == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		st := s.probe(ctx)
		s.dispatch(func() { s.applyTools(st, notify) })
	}()
}

// applyTools records detected tools. It runs on the UI thread.
func (s *Shell) applyTools(st []toolchain.Status, notify bool) {
	s.tools = st
	if nv, ok := toolchain.ByID(st, "nvim"); ok {
		if nv.Found() {
			s.statusEditor.SetText("Neovim " + nv.Version + ": not attached")
		} else {
			s.statusEditor.SetText("Neovim: not found")
		}
	}
	var missing []string
	for _, t := range st {
		switch {
		case !t.Found() && t.Tool.Optional:
			s.logf("%s: not found (optional). Install: %s", t.Tool.Label, t.Hint())
		case !t.Found():
			s.logf("%s: not found. Install: %s", t.Tool.Label, t.Hint())
			missing = append(missing, t.Tool.Label)
		case t.Err != nil:
			s.logf("%s: %s, version unknown (%v)", t.Tool.Label, t.Path, t.Err)
		default:
			s.logf("%s %s: %s", t.Tool.Label, t.Version, t.Path)
		}
	}
	if !notify {
		return
	}
	if len(missing) == 0 {
		s.Notify(notifications.Success, "Toolchain ready", fmt.Sprintf("All required tools found (%d checked).", len(st)))
	} else {
		s.Notify(notifications.Warning, "Tools missing", strings.Join(missing, ", ")+". Install hints are in the Log.")
	}
}

// ToggleExplorer shows or hides the explorer.
func (s *Shell) ToggleExplorer() {
	s.bench.explorerOn = !s.bench.explorerOn
	if !s.bench.explorerOn && s.active == regionExplorer {
		s.win.Canvas().Unfocus()
		s.activate(regionEditor)
	}
	s.relayout()
}

// ToggleDock shows or hides the bottom panel. Showing it makes it the active region.
func (s *Shell) ToggleDock() {
	s.bench.dockOn = !s.bench.dockOn
	s.afterToggle(s.bench.dockOn, regionPanel)
}

// ToggleInspector shows or hides the inspector, docked or floating by window width. Showing it
// makes it the active region.
func (s *Shell) ToggleInspector() {
	s.inspectorAuto = false
	s.bench.inspectorOn = !s.bench.inspectorOn
	s.afterToggle(s.bench.inspectorOn, regionInspector)
}

func (s *Shell) afterToggle(shown bool, r region) {
	switch {
	case shown:
		s.activate(r)
	case s.active == r:
		s.win.Canvas().Unfocus()
		s.activate(regionEditor)
	}
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
	files, truncated := s.explorer.Files(quickOpenLimit)
	items := make([]commandpalette.Item, len(files))
	for i, f := range files {
		abs := filepath.Join(s.root, filepath.FromSlash(f))
		dir := path.Dir(f)
		if dir == "." {
			dir = ""
		}
		items[i] = commandpalette.Item{Title: path.Base(f), Detail: dir, Icon: icons.File,
			Run: func() {
				s.OpenFile(abs)
				s.FocusEditor()
			}}
	}
	prompt := "Go to a file by name"
	if truncated {
		prompt = fmt.Sprintf("Go to a file by name (first %d files)", quickOpenLimit)
		s.logf("Go to File lists the first %d files of %s; the rest are not searched.", quickOpenLimit, s.root)
	}
	s.palette.Show(prompt, items)
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
	s.activate(regionEditor)
}

// openSettingsFocused opens Settings from the keyboard (the command, the status-bar item) and
// puts keyboard focus on the first theme card, so Tab and Space work straight away.
func (s *Shell) openSettingsFocused() {
	s.OpenSettings()
	s.FocusEditor()
}

func (s *Shell) closeEditor() {
	if t := s.editors.Selected(); t != nil {
		s.editors.CloseIntercept(t)
	}
}

// saveCurrent writes the current buffer; saveAll writes every modified one.
func (s *Shell) saveCurrent() { s.editorCommand("write") }
func (s *Shell) saveAll()     { s.editorCommand("wall") }

func (s *Shell) editorCommand(cmd string) {
	if s.ed == nil || s.ed.sess == nil {
		s.Notify(notifications.Info, "Nothing to save", "No file is open in the editor.")
		return
	}
	sess := s.ed.sess
	go func() {
		if err := sess.Command(cmd); err != nil {
			s.dispatch(func() { s.Notify(notifications.Error, "Save failed", err.Error()) })
		}
	}()
}
