package shell

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// doctor is the Tools: Doctor dialog: every external tool PyxForge can use, whether it was
// found and where, what PyxForge does with it, and how to get it. A tool's executable can be
// chosen by hand; the choice is saved per user (toolchain.SaveOverride) and used by the
// desktop app and the command line from then on. Nothing here installs anything.
type doctor struct {
	s       *Shell
	d       dialog.Dialog
	summary *kit.Text
	body    *fyne.Container
	checks  int // detections finished, for tests
}

// openDoctor shows the dialog with the last detection, then detects again.
func (s *Shell) openDoctor() {
	dc := &doctor{s: s}
	s.doctor = dc
	dc.summary = kit.NewText("Checking tools…", kit.Body, kit.Secondary)
	dc.body = container.NewVBox()
	scroll := container.NewVScroll(dc.body)
	scroll.SetMinSize(fyne.NewSize(760, 460))
	again := widget.NewButton("Check Again", dc.check)
	note := kit.NewText("Missing tools limit only the features they serve; PyxForge itself runs without them.", kit.Body, kit.Tertiary)
	note.TextSize = theme.TextCaption + 1
	content := container.NewBorder(container.NewVBox(dc.summary, note, widget.NewSeparator()),
		container.NewHBox(layout.NewSpacer(), again), nil, nil, scroll)
	dc.d = dialog.NewCustom("Doctor", "Close", content, s.win)
	s.present(dc.d, dc.d.Hide)
	if s.tools != nil {
		dc.show(s.tools)
	}
	dc.check()
}

// check detects the tools off the UI thread and shows the result.
func (dc *doctor) check() {
	probe := dc.s.probe
	if probe == nil {
		probe = toolchain.Detect
	}
	dc.summary.SetText("Checking tools…")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		st := probe(ctx)
		dc.s.dispatch(func() {
			dc.s.applyTools(st, false)
			dc.show(st)
			dc.checks++
		})
	}()
}

// show lays out one row per tool, grouped by area.
func (dc *doctor) show(st []toolchain.Status) {
	found, missing := 0, 0
	for _, t := range st {
		switch {
		case t.Found():
			found++
		case !t.Tool.Optional:
			missing++
		}
	}
	text := fmt.Sprintf("%d of %d tools found", found, len(st))
	if missing > 0 {
		text += fmt.Sprintf(" · %d that a workflow needs are missing", missing)
	}
	dc.summary.SetText(text)
	dc.body.Objects = nil
	for _, area := range toolchain.Areas {
		heading := kit.NewText(string(area), kit.Label, kit.Secondary)
		added := false
		for _, t := range st {
			if t.Tool.Area != area {
				continue
			}
			if !added {
				dc.body.Add(container.New(layout.NewCustomPaddedLayout(theme.Space3, theme.Space1, 0, 0), heading))
				added = true
			}
			dc.body.Add(dc.row(t))
		}
	}
	dc.body.Refresh()
}

// doctorLines is what a row says about a tool, apart from its name.
func doctorLines(t toolchain.Status) (state, where, detail string) {
	switch {
	case t.Found() && t.Err != nil:
		state, where = "found, version unknown", t.Path
		detail = t.Err.Error()
	case t.Found():
		state, where = t.Version, t.Path
		if t.Source == toolchain.SourceBundled || t.Source == toolchain.SourceConfigured {
			where += "  (" + t.Source + ")"
		}
	case t.Err != nil: // a path set here that is gone
		state, where = "missing", t.Err.Error()
	default:
		state, where = "missing", "not on PATH"
		if h := t.Hint(); h != "" {
			detail = "Install: " + h
		}
	}
	return state, where, detail
}

func (dc *doctor) row(t toolchain.Status) fyne.CanvasObject {
	icon := kit.NewIcon(icons.CircleCheck, kit.Success)
	switch {
	case !t.Found() && t.Tool.Optional:
		icon = kit.NewIcon(icons.Circle, kit.Tertiary)
	case !t.Found():
		icon = kit.NewIcon(icons.TriangleAlert, kit.Warning)
	case t.Err != nil:
		icon = kit.NewIcon(icons.Info, kit.Secondary)
	}
	name := t.Tool.Label
	if t.Tool.Optional {
		name += " (optional)"
	}
	state, where, detail := doctorLines(t)
	title := kit.NewText(name, kit.Strong, kit.Primary)
	ver := kit.NewText(state, kit.Mono, kit.Secondary)
	path := widget.NewLabel(where)
	path.Truncation = fyne.TextTruncateEllipsis
	path.TextStyle.Monospace = true
	used := kit.NewText("Used for "+t.Tool.Feature, kit.Body, kit.Tertiary)
	used.TextSize = theme.TextCaption + 1
	lines := container.NewVBox(container.NewHBox(title, layout.NewSpacer(), ver), path, used)
	if detail != "" {
		d := widget.NewLabel(detail)
		d.Wrapping = fyne.TextWrapWord
		lines.Add(d)
	}
	choose := widget.NewButton("Choose…", func() { dc.choose(t.Tool) })
	buttons := container.NewVBox(choose)
	if t.Source == toolchain.SourceConfigured {
		buttons.Add(widget.NewButton("Use Default", func() { dc.set(t.Tool, "") }))
	}
	row := container.NewBorder(nil, nil, container.NewCenter(icon), buttons, lines)
	return container.NewVBox(container.New(layout.NewCustomPaddedLayout(theme.Space1, theme.Space1, theme.Space2, theme.Space2), row),
		kit.NewRule(false))
}

// choose asks for a tool's executable.
func (dc *doctor) choose(t toolchain.Tool) {
	open := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		p := filepath.FromSlash(r.URI().Path())
		r.Close()
		dc.set(t, p)
	}, dc.s.win)
	if runtime.GOOS == "windows" {
		open.SetFilter(storage.NewExtensionFileFilter([]string{".exe", ".com", ".bat", ".cmd"}))
	}
	open.SetTitleText("Choose " + t.Label)
	dc.s.present(open, nil)
}

// set saves a tool's path (or, empty, goes back to finding it) and checks again.
func (dc *doctor) set(t toolchain.Tool, path string) {
	if err := toolchain.SaveOverride(t.ID, path); err != nil {
		dc.s.Notify(notifications.Error, "Path not saved", err.Error())
		return
	}
	if path == "" {
		dc.s.logf("%s: found automatically again", t.Label)
	} else {
		dc.s.logf("%s: set to %s", t.Label, path)
	}
	dc.check()
}
