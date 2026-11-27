package shell

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Settings is an in-window modal with categories and search (decision D22), as Obsidian's is:
// a search field and the pages on the left, the page's groups of rows on the right, over a scrim.
// Esc, the close button and a click on the scrim close it. Only settings that exist have rows;
// a Core features page is added in the phase that builds its feature.

// Modal size: 90 % of the window, at most this.
const (
	settingsMaxW float32 = 1100
	settingsMaxH float32 = 780
	settingsNavW float32 = 240
)

// settingsPage is one page of Settings.
type settingsPage struct {
	id, title string
	icon      icons.Name
	content   fyne.CanvasObject
	rows      []*kit.SettingRow // the rows search looks through, in page order
}

// settingsModal is Settings while it is open.
type settingsModal struct {
	s             *Shell
	layer         *fyne.Container // the overlay: scrim, then panel
	panel         *kit.Float
	search        *kit.SearchField
	nav           *kit.NavList
	title         *kit.Text
	scroll        *container.Scroll // the page
	results       *fyne.Container   // search results, in resultsScroll
	resultsScroll *container.Scroll
	close         *kit.Button
	pages         []*settingsPage
	current       string
	// rows are the setting rows by the preference each one sets: TestEverySettingHasARow.
	rows  map[string]*kit.SettingRow
	modal *modal
}

// OpenSettings opens Settings at General, or brings it to the front.
func (s *Shell) OpenSettings() { s.openSettingsAt("general") }

// openSettingsFocused opens Settings from the keyboard (the command, the status-bar item) with
// keyboard focus in its search field, so typing searches at once.
func (s *Shell) openSettingsFocused() {
	s.openSettingsAt("general")
	s.win.Canvas().Focus(s.settingsUI.search)
}

// OpenSettingsAt opens Settings on a page ("general", "appearance", "hotkeys"…); for review
// renders.
func (s *Shell) OpenSettingsAt(page string) { s.openSettingsAt(page) }

// SearchSettings types a query into the open Settings' search field; for review renders.
func (s *Shell) SearchSettings(q string) {
	if s.settingsUI != nil {
		s.settingsUI.search.SetText(q)
	}
}

// openAppearanceFocused opens Settings at Appearance with keyboard focus on the first theme
// card (the status bar's theme item).
func (s *Shell) openAppearanceFocused() {
	s.openSettingsAt("appearance")
	if len(s.themeCards) > 0 {
		s.win.Canvas().Focus(s.themeCards[0])
	}
}

// openSettingsAt opens Settings on a page.
func (s *Shell) openSettingsAt(page string) {
	if s.settingsUI == nil {
		s.settingsUI = s.newSettingsModal()
		s.win.Canvas().Overlays().Add(s.settingsUI.layer)
		s.settingsUI.modal = &modal{hide: s.closeSettings}
		s.modals = append(s.modals, s.settingsUI.modal)
	}
	s.settingsUI.nav.Select(page)
}

// closeSettings closes Settings.
func (s *Shell) closeSettings() {
	m := s.settingsUI
	if m == nil {
		return
	}
	s.settingsUI = nil
	s.settingsHooks = nil
	s.themeCards = nil
	s.trustBox = nil
	s.modals = slices.DeleteFunc(s.modals, func(x *modal) bool { return x == m.modal })
	s.win.Canvas().Overlays().Remove(m.layer)
	s.FocusEditor()
}

// settingsKey sees every key while Settings is open (the window's key-down hook): Esc closes
// it whatever has focus, except in a search field with text, where Esc clears the text first.
func (s *Shell) settingsKey(ev *fyne.KeyEvent) {
	m := s.settingsUI
	if m == nil || ev.Name != fyne.KeyEscape {
		return
	}
	if f, ok := s.win.Canvas().Focused().(*kit.SearchField); ok && f.Text != "" {
		return
	}
	s.closeSettings()
}

func (s *Shell) newSettingsModal() *settingsModal {
	m := &settingsModal{s: s, rows: map[string]*kit.SettingRow{}}
	s.settingsHooks = nil
	m.pages = []*settingsPage{
		m.generalPage(), m.editorPage(), m.appearancePage(), m.terminalPage(), m.hotkeysPage(),
		m.toolchainsPage(), m.trustPage(), m.aboutPage(),
	}
	items := []kit.NavItem{{Label: "Options", Heading: true}}
	for _, p := range m.pages {
		items = append(items, kit.NavItem{ID: p.id, Label: p.title, Icon: p.icon})
	}
	m.nav = kit.NewNavList(items, m.show)
	m.search = kit.NewSearchField("Search settings…", m.find)
	m.search.OnEscapeEmpty = s.closeSettings
	m.title = kit.NewText("", kit.Heading, kit.Primary)
	m.close = kit.NewButtonWithIcon("Close Settings", icons.X, s.closeSettings)
	m.close.Variant = kit.ButtonIcon
	m.scroll = container.NewVScroll(container.NewStack())
	m.results = container.NewVBox()
	m.resultsScroll = container.NewVScroll(container.New(layout.NewCustomPaddedLayout(0, theme.Space4, theme.Space6, theme.Space6), m.results))
	m.resultsScroll.Hide()

	left := container.New(layout.NewCustomPaddedLayout(theme.Space4, theme.Space4, theme.Space3, theme.Space3),
		// The search field first in the container, so Tab goes from it to the page list
		// (NewBorder would put the centre object before the top one).
		container.New(layout.NewBorderLayout(m.search, nil, nil, nil), m.search,
			container.NewVScroll(container.New(layout.NewCustomPaddedLayout(theme.Space3, 0, 0, 0), m.nav))))
	leftPane := container.NewStack(kit.NewSurface(kit.Raised, nil), left)
	head := container.New(layout.NewCustomPaddedLayout(theme.Space4, theme.Space3, theme.Space6, theme.Space4),
		container.NewBorder(nil, nil, nil, m.close, container.NewVBox(layout.NewSpacer(), m.title, layout.NewSpacer())))
	right := container.NewBorder(head, nil, nil, nil, container.NewStack(m.scroll, m.resultsScroll))
	body := container.New(&fixedLeft{width: settingsNavW}, leftPane, kit.NewRule(true), right)
	m.panel = kit.NewFloat(body)
	m.panel.Padding = 0
	m.layer = container.New(&settingsLayout{}, newScrim(s.closeSettings), m.panel)
	return m
}

// show puts a page in the right column and clears the search.
func (m *settingsModal) show(id string) {
	p := m.page(id)
	if p == nil {
		return
	}
	m.current = id
	m.title.SetText(p.title)
	m.scroll.Content = container.New(layout.NewCustomPaddedLayout(0, theme.Space6, theme.Space6, theme.Space6), p.content)
	m.scroll.Offset = fyne.NewPos(0, 0)
	m.scroll.Refresh()
	if m.search.Text != "" {
		m.search.SetText("")
	}
	m.scroll.Show()
	m.resultsScroll.Hide()
}

func (m *settingsModal) page(id string) *settingsPage {
	for _, p := range m.pages {
		if p.id == id {
			return p
		}
	}
	return nil
}

// find shows the rows of every page that match query, grouped under their pages; an empty
// query shows the current page again.
func (m *settingsModal) find(query string) {
	if strings.TrimSpace(query) == "" {
		m.title.SetText(m.page(m.current).title)
		m.resultsScroll.Hide()
		m.scroll.Show()
		return
	}
	m.results.Objects = nil
	hits := 0
	for _, p := range m.pages {
		var rows []*kit.SettingRow
		for _, r := range p.rows {
			if r.Matches(query) {
				rows = append(rows, m.resultRow(p, r))
				hits++
			}
		}
		if len(rows) > 0 {
			m.results.Add(kit.NewSettingGroup(p.title, rows...))
		}
	}
	if hits == 0 {
		m.results.Add(kit.NewText("No setting matches “"+query+"”.", kit.Body, kit.Tertiary))
	}
	m.title.SetText(fmt.Sprintf("Search: %d %s", hits, plural(hits, "setting", "settings")))
	m.results.Refresh()
	m.scroll.Hide()
	m.resultsScroll.Show()
}

// resultRow is a search hit: the row's title and description, and Show, which opens its page
// at the row and flashes it.
func (m *settingsModal) resultRow(p *settingsPage, r *kit.SettingRow) *kit.SettingRow {
	show := kit.NewButton("Show", func() { m.jump(p.id, r) })
	show.Compact = true
	return kit.NewSettingRow(r.Title, r.Description, show)
}

// jump opens a page, scrolls to a row and flashes it.
func (m *settingsModal) jump(page string, r *kit.SettingRow) {
	m.nav.Select(page)
	m.s.dispatch(func() { // once the page is laid out
		d := fyne.CurrentApp().Driver()
		y := d.AbsolutePositionForObject(r).Y - d.AbsolutePositionForObject(m.scroll.Content).Y
		m.scroll.Offset = fyne.NewPos(0, max(y-theme.Space4, 0))
		m.scroll.Refresh()
		r.Flash()
	})
}

// row makes a setting row and registers it for search; pref names the preference it sets ("" for
// a row that only acts or informs).
func (m *settingsModal) row(p *settingsPage, pref, title, desc string, control fyne.CanvasObject, keywords ...string) *kit.SettingRow {
	r := kit.NewSettingRow(title, desc, control).WithKeywords(keywords...)
	p.rows = append(p.rows, r)
	if pref != "" {
		m.rows[pref] = r
	}
	return r
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// --- Pages

func (m *settingsModal) generalPage() *settingsPage {
	s := m.s
	p := &settingsPage{id: "general", title: "General", icon: icons.Settings}
	info := buildinfo.Read()
	built := "Go " + info.Go + " · Fyne " + info.Fyne + " · " + info.OS + "/" + info.Arch
	if r := info.ShortRevision(); r != "" {
		built = "Revision " + r + " · " + built
	}
	updates := kit.NewSwitch(s.app.Preferences().Bool(PrefUpdateCheck), func(on bool) { s.app.Preferences().SetBool(PrefUpdateCheck, on) })
	updates.Label = "Check for updates automatically"
	welcome := kit.NewSwitch(s.app.Preferences().BoolWithFallback(prefWelcome, true), func(on bool) { s.app.Preferences().SetBool(prefWelcome, on) })
	welcome.Label = "Welcome page at start-up"
	open := func(name string) *kit.Button { return kit.NewButton("Open", func() { s.openDoc(name) }) }
	p.content = container.New(layout.NewCustomPaddedVBoxLayout(theme.Space6),
		kit.NewSettingGroup("",
			m.row(p, "", "Version "+info.Version, built,
				kit.NewButton("Check for Updates", s.checkForUpdates), "update", "release", "build").
				WithLink("Read the changelog", func() { s.openDoc("CHANGELOG.md") }),
			m.row(p, PrefUpdateCheck, "Check for updates automatically",
				"At start-up, at most once a week, asks GitHub whether a newer PyxForge is out and says so; it downloads nothing. Off, PyxForge contacts nothing unless you choose Check for Updates.",
				updates, "network", "offline", "weekly"),
		),
		kit.NewSettingGroup("Start-up",
			m.row(p, prefWelcome, "Welcome page", "Shows the Welcome page when PyxForge starts without a folder: recent files and folders, and how to begin.",
				welcome, "start", "recent"),
		),
		kit.NewSettingGroup("Help",
			m.row(p, "", "Keyboard shortcuts", "Every key PyxForge takes and why, as shipped with this copy.", open("KEYMAP.md"), "keymap", "keys", "hotkeys"),
			m.row(p, "", "Features and their status", "What PyxForge does on Windows and Linux, and the test that shows it.", open("FEATURES.md"), "help", "documentation"),
			m.row(p, "", "Installing and updating", "Installers, the portable archive, toolchains and updates.", open("INSTALL.md"), "help", "install"),
			m.row(p, "", "Accessibility", "Keyboard operation, contrast and what works with a screen reader.", open("ACCESSIBILITY.md"), "help", "screen reader"),
		),
	)
	return p
}

func (m *settingsModal) editorPage() *settingsPage {
	p := &settingsPage{id: "editor", title: "Editor", icon: icons.FileCode}
	p.content = kit.NewSettingGroup("Text",
		m.row(p, prefEditorTextSize, "Text size", "The size of code in the editor, in points. Set in JetBrains Mono, which comes with PyxForge.",
			m.s.textSizeControl(prefEditorTextSize, "for (int i = 0; i < n; i++) total += a[i];"), "font", "zoom"))
	return p
}

func (m *settingsModal) appearancePage() *settingsPage {
	s := m.s
	p := &settingsPage{id: "appearance", title: "Appearance", icon: icons.Palette}
	s.themeCards = []*themeCard{newThemeCard(s, theme.SystemID, "System", "follows the OS")}
	for _, pl := range theme.Palettes {
		pol := "dark"
		if pl.Polarity == theme.Light {
			pol = "light"
		}
		s.themeCards = append(s.themeCards, newThemeCard(s, pl.ID, pl.Name, pol))
	}
	var cards []fyne.CanvasObject
	for _, c := range s.themeCards {
		cards = append(cards, c)
	}
	var accents []fyne.CanvasObject
	for _, a := range theme.Accents {
		accents = append(accents, newAccentCard(s, a))
	}
	grid := kit.NewWrapGrid(fyne.NewSize(240, 168), func() {
		s.dispatch(func() {
			if s.settingsUI != nil {
				s.settingsUI.scroll.Content.Refresh()
			}
		})
	})
	motion := kit.NewSwitch(ReduceMotion(s.app), func(on bool) { s.app.Preferences().SetBool(PrefReduceMotion, on) })
	motion.Label = "Reduce motion"
	p.content = container.New(layout.NewCustomPaddedVBoxLayout(theme.Space4),
		kit.NewSettingGroup("",
			m.row(p, prefPalette, "Theme", "System follows the operating system: light uses Ink & Paper, dark uses Smoked Kraft.", nil, "dark", "light", "colours", "palette")),
		container.New(grid, cards...),
		kit.NewSettingGroup("",
			m.row(p, prefAccent, "Accent", "The colour of the active tab, keyboard focus and the one primary action on a screen.",
				container.NewHBox(accents...), "colour", "crimson", "amber"),
			m.row(p, PrefReduceMotion, "Reduce motion", "Switches and the start-up screen change at once instead of moving. Windows' own Animation effects setting does the same for the start-up screen.",
				motion, "animation"),
		),
	)
	return p
}

func (m *settingsModal) terminalPage() *settingsPage {
	s := m.s
	p := &settingsPage{id: "terminal", title: "Terminal", icon: icons.Terminal}
	shells := pty.Shells()
	var names []string
	for _, sh := range shells {
		names = append(names, sh.Name)
	}
	current := s.terminalShell()
	choose := kit.NewDropdown(names, current.Name, func(name string) {
		for _, sh := range shells {
			if sh.Name == name {
				s.app.Preferences().SetString(prefTerminalShell, sh.Argv[0])
			}
		}
	})
	choose.Label = "Shell"
	p.content = kit.NewSettingGroup("",
		m.row(p, prefTerminalShell, "Shell", "The program new terminal sessions run; sessions already open keep theirs. Command blocks (planned) need PowerShell, bash, zsh or fish.",
			choose, "powershell", "pwsh", "cmd", "bash", "zsh", "fish"),
		m.row(p, prefTerminalTextSize, "Text size", "The size of text in the terminal, in points.",
			s.textSizeControl(prefTerminalTextSize, "$ git status --short && make -j8"), "font", "zoom"),
	)
	return p
}

// hotkeysPage lists every command and its keys; changing them is planned (D22).
func (m *settingsModal) hotkeysPage() *settingsPage {
	s := m.s
	p := &settingsPage{id: "hotkeys", title: "Hotkeys", icon: icons.Command}
	list := container.NewVBox()
	fill := func(filter string) {
		var rows []*kit.SettingRow
		for _, c := range s.cmds.All() {
			keys := c.Keys
			if keys == "" {
				keys = "—"
			}
			r := kit.NewSettingRow(c.Title, c.Category, kit.NewText(keys, kit.Mono, kit.Secondary))
			if filter == "" || r.Matches(filter) || strings.Contains(strings.ToLower(keys), strings.ToLower(filter)) {
				rows = append(rows, r)
			}
		}
		list.Objects = []fyne.CanvasObject{kit.NewSettingGroup("", rows...)}
		list.Refresh()
	}
	filter := kit.NewSearchField("Filter commands or keys…", fill)
	fill("")
	note := m.row(p, "", "Changing keys", "Shown here as registered. Changing a key is planned; the keymap document explains which keys PyxForge takes and why.", nil, "rebind", "shortcut")
	p.content = container.New(layout.NewCustomPaddedVBoxLayout(theme.Space4), kit.NewSettingGroup("", note), filter, list)
	return p
}

func (m *settingsModal) toolchainsPage() *settingsPage {
	s := m.s
	p := &settingsPage{id: "toolchains", title: "Toolchains", icon: icons.Hammer}
	p.content = kit.NewSettingGroup("",
		m.row(p, "", "Dependency Manager", "Compilers, debuggers, assemblers and Git that PyxForge can install and update itself, each pinned and checked against its publisher's checksum.",
			kit.NewButton("Open…", s.openDependencies), "install", "gcc", "gdb", "nasm", "qemu"),
		m.row(p, "", "Doctor", "Which tools PyxForge found, their versions, and where to choose another copy.",
			kit.NewButton("Open…", s.openDoctor), "path", "tools", "version"),
		m.row(p, "", "Setup wizard", "The first-start steps again: what you build, which tools you have, installing the missing ones.",
			kit.NewButton("Run…", s.openSetupWizard), "first", "profile"),
	)
	return p
}

func (m *settingsModal) trustPage() *settingsPage {
	p := &settingsPage{id: "trust", title: "Trusted Folders", icon: icons.Flag}
	p.content = container.New(layout.NewCustomPaddedVBoxLayout(theme.Space4),
		kit.NewSettingGroup("", m.row(p, "", "Trusted folders",
			"PyxForge runs a project's builds, machine, files and Git hooks only in folders you trust, and asks the first time; folders inside a trusted one are trusted too.",
			nil, "security", "trust", "revoke")),
		m.s.trustSettings())
	return p
}

func (m *settingsModal) aboutPage() *settingsPage {
	p := &settingsPage{id: "about", title: "About", icon: icons.Info}
	info := buildinfo.Read()
	revision := info.ShortRevision()
	if revision == "" {
		revision = "not recorded"
	} else if info.Modified {
		revision += " (modified)"
	}
	fact := func(title, value string) *kit.SettingRow {
		return m.row(p, "", title, "", kit.NewText(value, kit.Mono, kit.Primary), "about", "version")
	}
	p.content = kit.NewSettingGroup("PyxForge",
		fact("Version", info.Version), fact("Revision", revision), fact("Go", info.Go), fact("Fyne", info.Fyne),
		fact("Platform", info.OS+"/"+info.Arch), fact("License", "Apache-2.0"), fact("Workspace", m.s.root))
	return p
}

// --- Layout

// settingsLayout puts the scrim over the whole window and the panel in its middle: 90 % of the
// window, at most settingsMaxW × settingsMaxH.
type settingsLayout struct{}

func (settingsLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(s)
	w, h := min(s.Width*0.9, settingsMaxW), min(s.Height*0.9, settingsMaxH)
	objs[1].Move(fyne.NewPos((s.Width-w)/2, (s.Height-h)/2))
	objs[1].Resize(fyne.NewSize(w, h))
}

func (settingsLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

// fixedLeft gives its first object a fixed width, the second (a rule) its own, and the third the
// rest.
type fixedLeft struct{ width float32 }

func (l *fixedLeft) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(fyne.NewSize(l.width, s.Height))
	rw := objs[1].MinSize().Width
	objs[1].Move(fyne.NewPos(l.width, 0))
	objs[1].Resize(fyne.NewSize(rw, s.Height))
	objs[2].Move(fyne.NewPos(l.width+rw, 0))
	objs[2].Resize(fyne.NewSize(max(s.Width-l.width-rw, 0), s.Height))
}

func (l *fixedLeft) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.width+1+objs[2].MinSize().Width, max(objs[0].MinSize().Height, objs[2].MinSize().Height))
}

// scrim dims the workspace behind Settings; a click on it closes Settings.
type scrim struct {
	widget.BaseWidget
	onTap func()
}

func newScrim(onTap func()) *scrim {
	s := &scrim{onTap: onTap}
	s.ExtendBaseWidget(s)
	return s
}

func (s *scrim) Tapped(*fyne.PointEvent) { s.onTap() }

func (s *scrim) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(color.Transparent)
	return &scrimRenderer{r: r}
}

type scrimRenderer struct{ r *canvas.Rectangle }

func (r *scrimRenderer) Refresh()                     { r.r.FillColor = theme.Current().Float.Scrim; r.r.Refresh() }
func (r *scrimRenderer) Layout(s fyne.Size)           { r.r.Resize(s); r.Refresh() }
func (r *scrimRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *scrimRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.r} }
func (r *scrimRenderer) Destroy()                     {}
