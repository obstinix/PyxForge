package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// Workspace trust (#31, #30; workspace.TrustStore). Build, Run and Debug in QEMU, Run and Debug
// Current File, and a Git operation running a repository's own hooks folder, run programs the
// project's files name. In a folder the user hasn't trusted, PyxForge first shows what would run
// and asks; declining runs nothing, trusting runs it and is remembered.

// maxListed is how many commands the trust dialog lists before saying how many more.
const maxListed = 8

// trustRootFor is the folder whose trust covers running path: the workspace, or the file's own
// folder when none is open.
func (s *Shell) trustRootFor(path string) string {
	if s.root != "" {
		return s.root
	}
	return filepath.Dir(path)
}

// trusted reports whether dir's own commands may run. Without a store (tests that are not about
// trust), every folder is.
func (s *Shell) trusted(dir string) bool {
	return s.opts.Trust == nil || s.opts.Trust.Trusted(dir)
}

// withTrust runs run when dir is trusted, and otherwise asks first (askTrust).
func (s *Shell) withTrust(dir, action string, commands [][]string, run func()) {
	if s.trusted(dir) {
		run()
		return
	}
	s.askTrust(dir, action, commands, run)
}

// askTrust shows what action would run in dir and asks to trust the folder; trusting it records
// that and runs run. Cancel runs nothing.
func (s *Shell) askTrust(dir, action string, commands [][]string, run func()) {
	var d dialog.Dialog
	intro := widget.NewLabel(fmt.Sprintf("PyxForge runs nothing a project names until you trust its folder. %s would run:", action))
	intro.Wrapping = fyne.TextWrapWord
	var lines []string
	for i, c := range commands {
		if i == maxListed {
			lines = append(lines, fmt.Sprintf("… and %d more", len(commands)-maxListed))
			break
		}
		lines = append(lines, commandLine(c))
	}
	if len(lines) == 0 {
		lines = []string{"(the project's own tools)"}
	}
	list := widget.NewLabel(strings.Join(lines, "\n"))
	list.TextStyle = fyne.TextStyle{Monospace: true}
	list.Wrapping = fyne.TextWrapBreak
	note := widget.NewLabel(fmt.Sprintf("Trusting %s lets PyxForge run its builds, machine, files and Git hooks from now on. "+
		"Settings lists the folders you trust; Revoke Trust for This Folder in the palette undoes it.", dir))
	note.Wrapping = fyne.TextWrapWord
	trust := widget.NewButton("Trust and "+action, func() {
		d.Hide()
		s.trustPrompt = nil
		if err := s.opts.Trust.Trust(dir); err != nil {
			s.Notify(notifications.Error, "Trust not remembered", err.Error())
			return
		}
		s.logf("Trusted %s (for %s)", dir, action)
		run()
	})
	trust.Importance = widget.HighImportance
	cancel := widget.NewButton("Cancel", func() {
		d.Hide()
		s.trustPrompt = nil
		s.logf("%s not run: %s is not trusted", action, dir)
	})
	body := container.NewVBox(intro, container.NewPadded(list), note)
	content := container.NewBorder(nil, container.NewHBox(cancel, trust), nil, nil, body)
	d = dialog.NewCustomWithoutButtons("Trust "+filepath.Base(dir)+"?", content, s.win)
	d.Resize(fyne.NewSize(560, content.MinSize().Height+80))
	s.present(d, nil) // Enter does not trust: that takes the button
	s.lastDialog = d
	s.trustPrompt = &trustPrompt{dir: dir, action: action, trust: trust, cancel: cancel, content: content}
}

// trustPrompt is the trust dialog showing, for tests.
type trustPrompt struct {
	dir, action   string
	trust, cancel *widget.Button
	content       fyne.CanvasObject
}

// commandLine shows argv as one line, quoting the arguments that hold spaces.
func commandLine(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		if a == "" || strings.ContainsAny(a, " \t\"") {
			a = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		}
		parts[i] = a
	}
	return strings.Join(parts, " ")
}

// revokeTrust stops trusting the workspace (or the folder that makes it trusted).
func (s *Shell) revokeTrust() {
	if s.opts.Trust == nil || s.root == "" {
		s.Notify(notifications.Info, "No folder to revoke", "Open a folder first.")
		return
	}
	by := s.opts.Trust.TrustedBy(s.root)
	if by == "" {
		s.Notify(notifications.Info, "Not trusted", s.root+" isn't trusted: PyxForge asks before running its commands.")
		return
	}
	if err := s.opts.Trust.Revoke(by); err != nil {
		s.Notify(notifications.Error, "Trust not revoked", err.Error())
		return
	}
	s.logf("Revoked trust in %s", by)
	s.Notify(notifications.Info, "Trust revoked", "PyxForge asks again before running "+filepath.Base(by)+"'s commands.")
	s.refreshTrustSettings()
}

// trustFolder trusts the workspace without running anything.
func (s *Shell) trustFolder() {
	if s.opts.Trust == nil || s.root == "" {
		s.Notify(notifications.Info, "No folder to trust", "Open a folder first.")
		return
	}
	if err := s.opts.Trust.Trust(s.root); err != nil {
		s.Notify(notifications.Error, "Trust not remembered", err.Error())
		return
	}
	s.logf("Trusted %s", s.root)
	s.refreshTrustSettings()
}

// defaultTrust is the trust store for the app: the user's configuration folder, or, when there
// is none, a file in the temporary folder, so trust still works for the session.
func defaultTrust() *workspace.TrustStore {
	st, err := workspace.DefaultTrustStore()
	if err != nil {
		st = workspace.TrustStore{Path: filepath.Join(os.TempDir(), "PyxForge-trusted-folders.json")}
	}
	return &st
}

// trustSettings is Settings' Trusted folders section: each folder with Revoke, and Trust This
// Folder for an open workspace that isn't trusted.
func (s *Shell) trustSettings() fyne.CanvasObject {
	s.trustBox = container.NewVBox()
	s.refreshTrustSettings()
	return s.trustBox
}

func (s *Shell) refreshTrustSettings() {
	box := s.trustBox
	if box == nil {
		return
	}
	box.Objects = nil
	if s.opts.Trust == nil {
		box.Refresh()
		return
	}
	list, err := s.opts.Trust.List()
	switch {
	case err != nil:
		box.Add(kit.NewText("The list of trusted folders can't be read: "+err.Error(), kit.Body, kit.Warning))
	case len(list) == 0:
		box.Add(kit.NewText("No folder is trusted yet.", kit.Body, kit.Tertiary))
	}
	for _, dir := range list {
		dir := dir
		revoke := widget.NewButton("Revoke", func() {
			if err := s.opts.Trust.Revoke(dir); err != nil {
				s.Notify(notifications.Error, "Trust not revoked", err.Error())
				return
			}
			s.logf("Revoked trust in %s", dir)
			s.refreshTrustSettings()
		})
		box.Add(container.NewBorder(nil, nil, nil, revoke, kit.NewText(dir, kit.Mono, kit.Primary)))
	}
	if s.root != "" && !s.opts.Trust.Trusted(s.root) {
		box.Add(container.NewHBox(widget.NewButton("Trust This Folder", s.trustFolder)))
	}
	box.Refresh()
}
