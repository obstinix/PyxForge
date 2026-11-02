package shell

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/managed"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// The Dependency Manager (Settings, Run > Dependency Manager…) installs, updates, repairs and
// removes the toolchains PyxForge manages itself (internal/managed), and says for each tool
// they provide where PyxForge finds it now: bundled with PyxForge, installed by it, or the
// user's own on PATH. Installs run off the UI thread and stop with the window.

// depInstall is a package being installed.
type depInstall struct {
	cancel context.CancelFunc
	last   managed.Progress
}

type depsDialog struct {
	s    *Shell
	d    dialog.Dialog
	body *fyne.Container
	bars map[string]*widget.ProgressBar // installs in progress, by package
	note map[string]*kit.Text           // their phase
}

// toolchainManager is the manager this shell uses: Options.Toolchains in tests, else the user's.
func (s *Shell) toolchainManager() (*managed.Manager, error) {
	if s.opts.Toolchains != nil {
		return s.opts.Toolchains, nil
	}
	return managed.Default()
}

func (s *Shell) openDependencies() {
	dd := &depsDialog{s: s, body: container.NewVBox(), bars: map[string]*widget.ProgressBar{}, note: map[string]*kit.Text{}}
	s.deps = dd
	intro := kit.NewText("Toolchains PyxForge downloads and keeps up to date itself, in its own folder. Nothing on PATH or in your projects changes.", kit.Body, kit.Secondary)
	scroll := container.NewVScroll(dd.body)
	scroll.SetMinSize(fyne.NewSize(760, 420))
	logBtn := widget.NewButton("View Install Log", s.openToolchainLog)
	content := container.NewBorder(container.NewVBox(intro, widget.NewSeparator()),
		container.NewHBox(layout.NewSpacer(), logBtn), nil, nil, scroll)
	dd.d = dialog.NewCustom("Dependency Manager", "Close", content, s.win)
	dd.d.SetOnClosed(func() {
		if s.deps == dd {
			s.deps = nil
		}
	})
	dd.show()
	s.present(dd.d, dd.d.Hide)
}

// show rebuilds the rows from the packages' state on disk.
func (dd *depsDialog) show() {
	s := dd.s
	dd.body.Objects = nil
	clear(dd.bars)
	clear(dd.note)
	m, err := s.toolchainManager()
	pkgs := managed.ForPlatform()
	switch {
	case err != nil:
		dd.body.Add(widget.NewLabel("The toolchain folder is not available: " + err.Error()))
	case len(pkgs) == 0:
		l := widget.NewLabel("PyxForge installs no toolchains on this system. Install them with your package manager; Run > Doctor… lists the commands for each tool.")
		l.Wrapping = fyne.TextWrapWord
		dd.body.Add(l)
	}
	for _, p := range pkgs {
		if m != nil {
			dd.body.Add(dd.row(m, p))
		}
	}
	dd.body.Refresh()
}

func (dd *depsDialog) row(m *managed.Manager, p managed.Package) fyne.CanvasObject {
	s := dd.s
	st := m.Status(p)
	busy := s.depInstalls[p.ID]
	icon := kit.NewIcon(icons.Circle, kit.Tertiary)
	state := st.State.String()
	switch st.State {
	case managed.Installed:
		icon = kit.NewIcon(icons.CircleCheck, kit.Success)
	case managed.UpdateAvailable:
		icon = kit.NewIcon(icons.Info, kit.Secondary)
		state += " · installed " + st.Receipt.Version
	case managed.Broken:
		icon = kit.NewIcon(icons.TriangleAlert, kit.Warning)
	case managed.External:
		icon = kit.NewIcon(icons.CircleCheck, kit.Success)
	}
	title := kit.NewText(p.Name, kit.Strong, kit.Primary)
	ver := kit.NewText(p.Version+" · "+state, kit.Mono, kit.Secondary)
	about := widget.NewLabel(p.About)
	about.Wrapping = fyne.TextWrapWord
	lines := container.NewVBox(container.NewHBox(title, layout.NewSpacer(), ver), about)
	for _, l := range dd.toolLines(p) {
		t := kit.NewText(l, kit.Body, kit.Tertiary)
		t.TextSize = theme.TextCaption + 1
		lines.Add(t)
	}
	if st.State == managed.External {
		n := widget.NewLabel("Found in " + s.toolchainDir(p) + ". PyxForge uses it as it is and does not update or remove it.")
		n.Wrapping = fyne.TextWrapWord
		lines.Add(n)
	}
	if st.Problem != nil {
		pr := widget.NewLabel(st.Problem.Error())
		pr.Wrapping = fyne.TextWrapWord
		lines.Add(pr)
	}
	buttons := container.NewVBox()
	if busy != nil {
		bar := widget.NewProgressBar()
		phase := kit.NewText("Starting…", kit.Body, kit.Secondary)
		dd.bars[p.ID], dd.note[p.ID] = bar, phase
		dd.progress(p.ID, busy.last)
		lines.Add(container.NewBorder(nil, nil, nil, phase, bar))
		buttons.Add(widget.NewButton("Cancel", busy.cancel))
	} else {
		size := fmt.Sprintf(" (%.0f MB)", float64(p.Size)/1e6)
		switch st.State {
		case managed.NotInstalled:
			b := widget.NewButton("Install"+size, func() { s.installToolchain(p) })
			b.Importance = widget.HighImportance
			buttons.Add(b)
		case managed.UpdateAvailable:
			b := widget.NewButton("Update"+size, func() { s.installToolchain(p) })
			b.Importance = widget.HighImportance
			buttons.Add(b)
		case managed.Broken:
			b := widget.NewButton("Repair"+size, func() { s.installToolchain(p) })
			b.Importance = widget.HighImportance
			buttons.Add(b)
		case managed.Installed:
			buttons.Add(widget.NewButton("Verify", func() { s.verifyToolchain(p) }))
		}
		if st.State != managed.NotInstalled && st.State != managed.External {
			buttons.Add(widget.NewButton("Uninstall", func() { s.uninstallToolchain(p) }))
		}
	}
	row := container.NewBorder(nil, nil, container.NewCenter(icon), buttons, lines)
	return container.NewVBox(container.New(layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, theme.Space2, theme.Space2), row),
		kit.NewRule(false))
}

// toolLines say, for each tool p provides, where PyxForge finds it now (from the last check).
func (dd *depsDialog) toolLines(p managed.Package) []string {
	var out []string
	for _, id := range p.Provides {
		st, ok := toolchain.ByID(dd.s.tools, id)
		if !ok {
			continue
		}
		switch {
		case st.Found():
			out = append(out, fmt.Sprintf("%s %s: %s", st.Tool.Label, st.Version, sourceName(st.Source)))
		default:
			out = append(out, st.Tool.Label+": missing")
		}
	}
	return out
}

func sourceName(src string) string {
	switch src {
	case toolchain.SourcePath, toolchain.SourceFolder:
		return "your own installation"
	case toolchain.SourceConfigured:
		return "the path set in Doctor"
	}
	return src
}

// progress shows an install's progress in its row, when the dialog shows it.
func (dd *depsDialog) progress(id string, pr managed.Progress) {
	bar, note := dd.bars[id], dd.note[id]
	if bar == nil {
		return
	}
	if pr.Total > 0 {
		bar.SetValue(float64(pr.Done) / float64(pr.Total))
	}
	if pr.Phase != "" {
		note.SetText(pr.Phase + "…")
	}
}

// installToolchain installs, updates or repairs p in the background.
func (s *Shell) installToolchain(p managed.Package) {
	if s.depInstalls[p.ID] != nil {
		return
	}
	m, err := s.toolchainManager()
	if err != nil {
		s.Notify(notifications.Error, "Cannot install "+p.Name, err.Error())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	in := &depInstall{cancel: cancel}
	if s.depInstalls == nil {
		s.depInstalls = map[string]*depInstall{}
	}
	s.depInstalls[p.ID] = in
	s.logf("Installing %s %s", p.Name, p.Version)
	if s.deps != nil {
		s.deps.show()
	}
	go func() {
		defer cancel()
		var last time.Time // this goroutine's own: in.last belongs to the UI thread
		lastPhase := ""
		err := m.Install(ctx, p, func(pr managed.Progress) {
			if time.Since(last) < 100*time.Millisecond && pr.Phase == lastPhase {
				return // a few updates a second are enough for a progress bar
			}
			last, lastPhase = time.Now(), pr.Phase
			s.dispatch(func() {
				in.last = pr
				if s.deps != nil {
					s.deps.progress(p.ID, pr)
				}
			})
		})
		s.dispatch(func() {
			delete(s.depInstalls, p.ID)
			switch {
			case err != nil && ctx.Err() != nil:
				s.Notify(notifications.Info, p.Name+" not installed", "The install was cancelled; what was installed before is unchanged.")
			case err != nil:
				s.Notify(notifications.Error, p.Name+" not installed", err.Error())
			default:
				toolchain.ExtendPath() // what PyxForge starts from now on finds it
				s.Notify(notifications.Success, p.Name+" installed", "Version "+p.Version+". "+strings.Join(p.Provides, ", ")+" are ready.")
				s.checkTools(false)
			}
			if s.deps != nil {
				s.deps.show()
			}
		})
	}()
}

func (s *Shell) verifyToolchain(p managed.Package) {
	m, err := s.toolchainManager()
	if err == nil {
		err = m.Verify(p)
	}
	if err != nil {
		s.Notify(notifications.Error, p.Name+" needs repair", err.Error())
	} else {
		s.Notify(notifications.Success, p.Name+" is complete", "Every file it needs is in place.")
	}
	if s.deps != nil {
		s.deps.show()
	}
}

func (s *Shell) uninstallToolchain(p managed.Package) {
	m, err := s.toolchainManager()
	if err != nil {
		s.Notify(notifications.Error, "Cannot uninstall "+p.Name, err.Error())
		return
	}
	s.choose("Uninstall "+p.Name+"?",
		fmt.Sprintf("This removes %s from %s. Your own installations and your projects are not touched; you can install it again later.", p.Name, m.PackageDir(p)),
		choice{label: "Uninstall", primary: true, run: func() {
			if err := m.Uninstall(p); err != nil {
				s.Notify(notifications.Error, p.Name+" not removed", err.Error())
			} else {
				s.Notify(notifications.Info, p.Name+" removed", "Tools it provided are found elsewhere, if they are installed.")
				s.checkTools(false)
			}
			if s.deps != nil {
				s.deps.show()
			}
		}})
}

func (s *Shell) openToolchainLog() {
	m, err := s.toolchainManager()
	if err == nil {
		_, err = os.Stat(m.LogFile)
	}
	if err != nil {
		s.Notify(notifications.Info, "No install log yet", "Installs, updates and removals are logged once one has run.")
		return
	}
	s.OpenFile(m.LogFile)
}

// stopToolchainInstalls cancels installs in progress: the window is closing.
func (s *Shell) stopToolchainInstalls() {
	for _, in := range s.depInstalls {
		in.cancel()
	}
}

// toolchainFor is the package this platform can install that provides tool id, if any.
func toolchainFor(id string) (managed.Package, bool) {
	for _, p := range managed.ForPlatform() {
		for _, t := range p.Provides {
			if t == id {
				return p, true
			}
		}
	}
	return managed.Package{}, false
}

// prefSetupOffered records that the first tool check offered the Dependency Manager.
const prefSetupOffered = "setup.toolchainsOffered"

// firstRunSetup opens the Dependency Manager after the first tool check on this computer when a
// tool a workflow needs is missing and PyxForge can install it: someone new gets their
// compilers, debugger and assembler without reading Doctor first. It happens once; Settings and
// Run > Dependency Manager… open it any time after.
func (s *Shell) firstRunSetup(st []toolchain.Status) {
	p := s.app.Preferences()
	if p.Bool(prefSetupOffered) {
		return
	}
	var needed []string
	for _, t := range st {
		if t.Found() || t.Tool.Optional {
			continue
		}
		if _, ok := toolchainFor(t.Tool.ID); ok {
			needed = append(needed, t.Tool.Label)
		}
	}
	if len(needed) == 0 {
		return
	}
	p.SetBool(prefSetupOffered, true)
	s.logf("First start: %s missing; offering the Dependency Manager", strings.Join(needed, ", "))
	s.openDependencies()
	s.Notify(notifications.Info, "Set up your toolchains", strings.Join(needed, ", ")+
		" are not installed. PyxForge can install them for you here; everything else works without them.")
}

// toolchainDir is where p is, or would be, installed.
func (s *Shell) toolchainDir(p managed.Package) string {
	m, err := s.toolchainManager()
	if err != nil {
		return ""
	}
	return m.PackageDir(p)
}
