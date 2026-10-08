package shell

import (
	"runtime"
	"runtime/debug"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// about shows real build facts: version, toolchain, toolkit, license, workspace.
func (s *Shell) about() {
	fyneVersion := "unknown"
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "fyne.io/fyne/v2" {
				fyneVersion = d.Version
			}
		}
	}
	name := kit.NewText("PyxForge", kit.Display, kit.Primary)
	name.TextSize = theme.TextHeading
	facts := container.New(layout.NewFormLayout())
	for _, row := range [][2]string{
		{"Version", Version},
		{"Go", runtime.Version()},
		{"Fyne", fyneVersion},
		{"License", "Apache-2.0"},
		{"Workspace", s.root},
	} {
		facts.Add(kit.NewText(row[0], kit.Body, kit.Tertiary))
		facts.Add(kit.NewText(row[1], kit.Mono, kit.Primary))
	}
	content := container.NewVBox(name,
		kit.NewText("A native environment for bootloader, kernel and bare-metal work.", kit.Body, kit.Secondary),
		widget.NewSeparator(), facts)
	dialog.NewCustom("About", "Close", content, s.win).Show()
}

// confirmReset returns appearance to the defaults after asking.
func (s *Shell) confirmReset() {
	dialog.ShowConfirm("Reset appearance",
		"Return to the System theme, the Crimson accent and opaque overlays?",
		func(ok bool) {
			if ok {
				s.SetSelection(theme.Default)
				s.Notify(notifications.Success, "Appearance reset", s.themeLabel())
			}
		}, s.win)
}
