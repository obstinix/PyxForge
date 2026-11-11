package shell

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

const prefWelcome = "welcome.showOnStartup"

func (s *Shell) showWelcome() {
	if s.welcome == nil {
		s.welcome = container.NewTabItem("Welcome", s.welcomeView())
		s.editors.Append(s.welcome)
	}
	s.refreshWelcomeRecent()
	s.editors.Select(s.welcome)
	s.syncEditor()
}

func welcomeLink(label string, run func()) *widget.Hyperlink {
	link := widget.NewHyperlink(label, nil)
	link.OnTapped = run
	return link
}

func (s *Shell) welcomeView() fyne.CanvasObject {
	title := kit.NewText("PyxForge", kit.Display, kit.Primary)
	title.TextSize = theme.TextHeading * 2
	subtitle := kit.NewText("Write, run and ship code in one place", kit.Body, kit.Secondary)
	start := container.NewVBox(kit.Title("Start"),
		welcomeLink("New File", s.newUntitled),
		welcomeLink("Open File", s.openFileDialog),
		welcomeLink("Open Folder", s.openFolder),
		welcomeLink("New Project", s.newProject),
		welcomeLink("Clone Git Repository", s.cloneRepository))
	guide := container.NewVBox(kit.Title("Get Going"),
		welcomeLink("Find Any Command (Ctrl+Shift+P)", s.ShowCommands),
		welcomeLink("Set Up Languages and Tools", func() { s.cmds.Run("tools.dependencies") }),
		welcomeLink("Themes, Fonts and Settings", s.OpenSettings),
		welcomeLink("Documentation", func() { s.openURL("https://github.com/obstinix/PyxForge/tree/main/docs") }))
	s.welcomeRecent = container.NewVBox()
	check := widget.NewCheck("Show Welcome Page on Startup", func(on bool) { s.app.Preferences().SetBool(prefWelcome, on) })
	check.Checked = s.app.Preferences().BoolWithFallback(prefWelcome, true)
	body := container.NewVBox(title, subtitle,
		container.New(&welcomeColumns{}, start, guide),
		kit.Title("Recent Projects"), s.welcomeRecent, check)
	if s.root != "" {
		body.Add(welcomeLink("Close Workspace", s.closeWorkspace))
	}
	return container.NewVScroll(container.New(&welcomeLayout{}, body))
}

func (s *Shell) refreshWelcomeRecent() {
	if s.welcomeRecent == nil {
		return
	}
	s.welcomeRecent.RemoveAll()
	paths := s.app.Preferences().StringList(prefRecentFolders)
	if len(paths) == 0 {
		s.welcomeRecent.Add(kit.NewText("No recent projects", kit.Body, kit.Secondary))
		return
	}
	for _, path := range paths[:min(len(paths), maxRecent)] {
		name := welcomeLink(filepath.Base(path), func() { s.openRecent(prefRecentFolders, path) })
		detail := widget.NewLabel(path)
		detail.Importance, detail.Truncation = widget.LowImportance, fyne.TextTruncateEllipsis
		remove := kit.NewIconButton(icons.X, "Remove "+filepath.Base(path)+" from Recent Projects", func() { s.dropRecent(prefRecentFolders, path) })
		s.welcomeRecent.Add(container.NewBorder(nil, nil, nil, remove, container.NewVBox(name, detail)))
	}
}

// These layouts affect only the Welcome tab. Scrolling keeps ten recent projects usable
// in a short editor; the two action groups stack when the editor is narrow.
type welcomeLayout struct{}

func (*welcomeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	w := max(float32(0), min(float32(640), size.Width-2*theme.Space4))
	o := objects[0]
	o.Resize(fyne.NewSize(w, o.MinSize().Height))
	h := o.MinSize().Height
	o.Resize(fyne.NewSize(w, h))
	o.Move(fyne.NewPos((size.Width-w)/2, max(theme.Space4, (size.Height-h)/2)))
}
func (*welcomeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, objects[0].MinSize().Height+2*theme.Space4)
}

type welcomeColumns struct{}

func (l *welcomeColumns) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if size.Width < 480 {
		y := theme.Space4
		for _, o := range objects {
			o.Move(fyne.NewPos(0, y))
			o.Resize(fyne.NewSize(size.Width, o.MinSize().Height))
			y += o.MinSize().Height + theme.Space4
		}
		return
	}
	w := (size.Width - theme.Space4) / 2
	for i, o := range objects {
		o.Move(fyne.NewPos(float32(i)*(w+theme.Space4), theme.Space4))
		o.Resize(fyne.NewSize(w, o.MinSize().Height))
	}
}
func (l *welcomeColumns) MinSize(objects []fyne.CanvasObject) fyne.Size {
	// MinSize cannot depend on the last Layout pass: Fyne may ask before laying out at
	// the current width. Reserve the stacked height so a narrow first pass never overlaps.
	h := objects[0].MinSize().Height + objects[1].MinSize().Height + 3*theme.Space4
	return fyne.NewSize(0, h)
}
