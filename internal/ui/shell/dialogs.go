package shell

import (
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// about shows real build facts: version, toolchain, toolkit, license, workspace.
func (s *Shell) about() {
	info := buildinfo.Read()
	revision := info.ShortRevision()
	if revision == "" {
		revision = "not recorded"
	} else if info.Modified {
		revision += " (modified)"
	}
	name := s.brandRow(44, theme.TextHeading+6, false)
	facts := container.New(layout.NewFormLayout())
	for _, row := range [][2]string{
		{"Version", info.Version},
		{"Revision", revision},
		{"Go", info.Go},
		{"Fyne", info.Fyne},
		{"Platform", info.OS + "/" + info.Arch},
		{"License", "Apache-2.0"},
		{"Workspace", s.root},
	} {
		facts.Add(kit.NewText(row[0], kit.Body, kit.Tertiary))
		facts.Add(kit.NewText(row[1], kit.Mono, kit.Primary))
	}
	content := container.NewVBox(name,
		kit.NewText("A native development environment.", kit.Body, kit.Primary),
		kit.NewText("An embedded Neovim editor, an integrated terminal, builds, Git and debugging in one offline-first desktop app.", kit.Body, kit.Secondary),
		widget.NewSeparator(), facts)
	d := escCustom("About", "Close", content, s.win)
	s.present(d, d.Hide)
}

// confirmReset returns appearance to the defaults after asking.
func (s *Shell) confirmReset() {
	reset := func() {
		s.SetSelection(theme.Default)
		s.Notify(notifications.Success, "Appearance reset", s.themeLabel())
	}
	d := escConfirm("Reset appearance",
		"Return to the System theme, the Crimson accent and glass?",
		func(ok bool) {
			if ok {
				reset()
			}
		}, s.win)
	s.present(d, func() { d.Hide(); reset() })
}
