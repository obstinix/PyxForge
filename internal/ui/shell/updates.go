package shell

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/managed"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/update"
)

// Help > Check for Updates… asks GitHub Releases, only when chosen, whether a newer PyxForge
// is out. If one is, it shows its notes and can download its installer (on Windows) or package
// (on Linux), checked against the release's SHA-256. The installer is started only when the
// user says so, and PyxForge then closes so that it can be replaced; nothing is replaced
// behind the user's back, and settings and projects are kept.

// updater is the dialog's state.
type updater struct {
	s      *Shell
	d      dialog.Dialog
	body   *fyne.Container
	cancel context.CancelFunc
}

func (s *Shell) checkForUpdates() {
	if s.updates != nil {
		return
	}
	u := &updater{s: s, body: container.NewVBox()}
	s.updates = u
	u.say("Checking GitHub for a newer release…")
	scroll := container.NewVScroll(u.body)
	scroll.SetMinSize(fyne.NewSize(620, 320))
	u.d = escCustom("Check for Updates", "Close", scroll, s.win)
	u.d.SetOnClosed(func() {
		if u.cancel != nil {
			u.cancel()
		}
		if s.updates == u {
			s.updates = nil
		}
	})
	s.present(u.d, u.d.Hide)
	info := buildinfo.Read()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	u.cancel = cancel
	go func() {
		defer cancel()
		rel, ok, err := update.Check(ctx, nil, update.Current{Version: info.Version, Built: info.Time})
		s.dispatch(func() { u.show(info.Version, rel, ok, err) })
	}()
}

func (u *updater) say(text string) {
	u.body.Objects = nil
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	u.body.Add(l)
	u.body.Refresh()
}

// show says what the check found.
func (u *updater) show(current string, rel update.Release, ok bool, err error) {
	s := u.s
	u.body.Objects = nil
	u.body.Add(kit.NewText("This is PyxForge "+current+".", kit.Body, kit.Secondary))
	switch {
	case err != nil:
		u.body.Add(widget.NewLabel("The check did not complete: " + err.Error() + ". PyxForge works without it; try again later."))
	case !ok:
		u.body.Add(kit.NewText("No newer release: this is the latest.", kit.Strong, kit.Primary))
	default:
		u.body.Add(kit.NewText(fmt.Sprintf("PyxForge %s is available", rel.Version()), kit.Heading, kit.Primary))
		u.body.Add(kit.NewText("Published "+rel.Published.Format("2 January 2006"), kit.Body, kit.Tertiary))
		if notes := strings.TrimSpace(rel.Notes); notes != "" {
			n := widget.NewLabel(notes)
			n.Wrapping = fyne.TextWrapWord
			u.body.Add(n)
		}
		buttons := container.NewHBox(layout.NewSpacer(), widget.NewButton("Open Release Page", func() { s.openURL(rel.Page) }))
		if a, has := update.Installer(rel); has {
			b := widget.NewButton(fmt.Sprintf("Download (%.0f MB)", float64(a.Size)/1e6), func() { u.download(rel, a) })
			b.Importance = widget.HighImportance
			buttons.Add(b)
		}
		u.body.Add(buttons)
	}
	u.body.Refresh()
}

// download fetches the installer or package, checks it, and offers to run it.
func (u *updater) download(rel update.Release, a update.Asset) {
	s := u.s
	dir, err := managed.DataDir()
	if err != nil {
		s.Notify(notifications.Error, "Update not downloaded", err.Error())
		return
	}
	bar := widget.NewProgressBar()
	u.body.Objects = []fyne.CanvasObject{kit.NewText("Downloading "+a.Name+"…", kit.Body, kit.Secondary), bar}
	u.body.Refresh()
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	go func() {
		defer cancel()
		var last time.Time
		path, err := update.Download(ctx, nil, rel, a, filepath.Join(dir, "cache", "updates"), func(done, total int64) {
			if time.Since(last) < 100*time.Millisecond || total <= 0 {
				return
			}
			last = time.Now()
			s.dispatch(func() { bar.SetValue(float64(done) / float64(total)) })
		})
		s.dispatch(func() {
			if err != nil {
				u.say("The download did not complete: " + err.Error())
				return
			}
			u.ready(rel, path)
		})
	}()
}

// ready offers to run the checked installer (Windows) or says how to install the package.
func (u *updater) ready(rel update.Release, path string) {
	s := u.s
	u.body.Objects = nil
	u.body.Add(kit.NewText("Downloaded and checked against the release's SHA-256.", kit.Strong, kit.Primary))
	u.body.Add(widget.NewLabel(path))
	if runtime.GOOS != "windows" {
		l := widget.NewLabel("Install it with: sudo apt install " + path)
		l.Wrapping = fyne.TextWrapWord
		u.body.Add(l)
		u.body.Refresh()
		return
	}
	run := widget.NewButton("Install and Restart", func() {
		s.choose("Install PyxForge "+rel.Version()+"?",
			"PyxForge closes and its installer runs; unsaved files are asked about first. Your settings, projects and installed toolchains are kept.",
			choice{label: "Install", primary: true, run: func() { s.runUpdateInstaller(path) }})
	})
	run.Importance = widget.HighImportance
	u.body.Add(container.NewHBox(layout.NewSpacer(), run))
	u.body.Refresh()
}

// runUpdateInstaller starts the installer and quits; the installer replaces this program. With
// unsaved files the quit asks first, and the installer waits for PyxForge to close.
func (s *Shell) runUpdateInstaller(path string) {
	start := s.startInstaller
	if start == nil {
		start = func(p string) error {
			cmd := exec.Command(p)
			proc.NoWindow(cmd) // the installer shows its own window; no console behind it
			return cmd.Start()
		}
	}
	if err := start(path); err != nil {
		s.Notify(notifications.Error, "The installer did not start", err.Error())
		return
	}
	s.logf("Update: started %s", path)
	s.confirmQuit()
}

// Checking on its own (opt-in): PyxForge contacts nothing unless asked, so a weekly check is a
// setting, off until the user turns it on. Then, at start-up and at most once a week, it asks
// GitHub as Help > Check for Updates… does and, if a newer release is out, says so in a
// notification; it never downloads anything itself.
const (
	PrefUpdateCheck     = "updates.weekly"    // bool: check once a week at start-up
	prefUpdateLastCheck = "updates.lastCheck" // RFC 3339: when it last asked
	updateCheckEvery    = 7 * 24 * time.Hour
)

// autoCheckForUpdates checks if the user turned weekly checks on and a week has passed.
func (s *Shell) autoCheckForUpdates() {
	p := s.app.Preferences()
	if !p.Bool(PrefUpdateCheck) {
		return
	}
	if last, err := time.Parse(time.RFC3339, p.String(prefUpdateLastCheck)); err == nil && time.Since(last) < updateCheckEvery {
		return
	}
	p.SetString(prefUpdateLastCheck, time.Now().UTC().Format(time.RFC3339))
	check := s.updateCheck
	if check == nil {
		check = func(ctx context.Context, cur update.Current) (update.Release, bool, error) {
			return update.Check(ctx, nil, cur)
		}
	}
	info := buildinfo.Read()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rel, ok, err := check(ctx, update.Current{Version: info.Version, Built: info.Time})
		s.dispatch(func() {
			switch {
			case err != nil:
				s.logf("Update check: %v", err) // quietly: it was not asked for just now
			case ok:
				s.Notify(notifications.Info, "PyxForge "+rel.Version()+" is available", "Help > Check for Updates… shows what is new and downloads it.")
			}
		})
	}()
}

// updateCheckSetting is the Settings check box for weekly checks.
func updateCheckSetting(s *Shell) *widget.Check {
	c := widget.NewCheck("Check for updates once a week", func(on bool) { s.app.Preferences().SetBool(PrefUpdateCheck, on) })
	c.Checked = s.app.Preferences().Bool(PrefUpdateCheck)
	return c
}
