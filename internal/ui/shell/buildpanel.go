package shell

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/build"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

const (
	maxBuildLines  = 5000
	defaultTargets = "Default targets"
	prefBuildPick  = "build.profile"
)

// buildPanel is the Build tab: choose a profile from pyxforge.toml, run it, watch its output.
// Diagnostics the tools print join the editor's in Problems.
type buildPanel struct {
	s       *Shell
	tab     *container.TabItem
	body    *fyne.Container // the output, or a message when there is nothing to build
	output  fyne.CanvasObject
	list    *logList
	frame   *kit.FocusFrame
	pick    *widget.Select
	run     *kit.IconButton
	stop    *kit.IconButton
	state   *kit.Text
	lines   []string
	diags   []problem
	cfg     *config.Config
	cancel  context.CancelFunc
	running bool
	done    chan struct{} // closed when the running build ends; for tests

	mu      sync.Mutex // guards pending, written by the build goroutine
	pending []string
	queued  atomic.Bool
}

func (s *Shell) buildBuildPanel() *container.TabItem {
	b := &buildPanel{s: s}
	s.buildp = b
	b.list = &logList{onFocus: func(on bool) {
		b.frame.SetFocused(on)
		if on {
			s.activate(regionPanel)
		}
	}}
	b.list.Length = func() int { return len(b.lines) }
	b.list.CreateItem = func() fyne.CanvasObject {
		t := kit.NewText("", kit.Mono, kit.Secondary)
		t.TextSize = theme.TextCaption + 1
		return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space2), t)
	}
	b.list.UpdateItem = func(i widget.ListItemID, o fyne.CanvasObject) {
		t := o.(*fyne.Container).Objects[0].(*kit.Text)
		line := b.lines[i]
		t.Role = kit.Secondary
		if strings.HasPrefix(line, "==> ") {
			t.Role = kit.Primary
		}
		t.SetText(line)
	}
	b.list.ExtendBaseWidget(b.list)
	b.frame = kit.NewFocusFrame(b.list)

	b.pick = widget.NewSelect(nil, func(v string) { s.app.Preferences().SetString(prefBuildPick, v) })
	b.pick.PlaceHolder = defaultTargets
	b.run = kit.NewIconButton(icons.Play, "Build (Ctrl+Shift+B)", b.start)
	b.stop = kit.NewIconButton(icons.Square, "Stop build", b.halt)
	b.stop.Disable()
	b.state = kit.NewText("Not built yet", kit.Body, kit.Secondary)
	b.state.TextSize = theme.TextCaption + 1
	bar := kit.Row(theme.TabBarHeight, container.NewHBox(b.state, layout.NewSpacer(), b.pick, b.run, b.stop))
	inset := container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space1), bar)
	b.output = container.NewBorder(container.NewVBox(inset, kit.NewRule(false)), nil, nil, nil, b.frame)
	b.body = container.NewStack(b.output)
	b.tab = container.NewTabItem("Build", b.body)
	return b.tab
}

// reload reads pyxforge.toml again, so edits made since the last build count. It reports
// whether there is something to build, and otherwise shows why not.
func (b *buildPanel) reload() bool {
	c, err := config.Load(b.s.root)
	if err == nil {
		err = c.Validate()
	}
	switch {
	case errors.Is(err, config.ErrNotFound) || errors.Is(err, fs.ErrNotExist):
		b.show(placeholder(icons.Hammer, "No pyxforge.toml",
			"Builds run the profiles in a pyxforge.toml at the project root. Add one with a [profiles.<name>] table naming the tool and its arguments."))
		return false
	case err != nil:
		b.show(placeholder(icons.TriangleAlert, "pyxforge.toml has an error", err.Error()))
		return false
	case len(c.Profiles) == 0:
		b.show(placeholder(icons.Hammer, "No build profiles", "pyxforge.toml has no [profiles.<name>] tables yet."))
		return false
	}
	b.cfg = c
	opts := append([]string{defaultTargets}, c.ProfileNames()...)
	b.pick.Options = opts
	want := b.s.app.Preferences().StringWithFallback(prefBuildPick, defaultTargets)
	if !slices.Contains(opts, want) {
		want = defaultTargets
	}
	b.pick.Selected = want
	b.pick.Refresh()
	b.show(b.output)
	return true
}

func (b *buildPanel) show(o fyne.CanvasObject) {
	if len(b.body.Objects) == 1 && b.body.Objects[0] == o {
		return
	}
	b.body.Objects = []fyne.CanvasObject{o}
	b.body.Refresh()
}

// start saves every modified buffer, then runs the chosen profile in the background.
func (b *buildPanel) start() {
	if b.running || !b.reload() {
		return
	}
	var names []string
	if p := b.pick.Selected; p != "" && p != defaultTargets {
		names = []string{p}
	}
	order, err := build.Order(b.cfg, orAll(b.cfg, names)...)
	if err != nil {
		b.s.Notify(notifications.Error, "Cannot build", err.Error())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel, b.running, b.done = cancel, true, make(chan struct{})
	b.lines = b.lines[:0]
	b.diags = nil
	b.s.refreshProblems()
	b.list.Refresh()
	b.run.Disable()
	b.stop.Enable()
	b.setState("Building " + strings.Join(order, ", ") + "…")
	b.s.logf("Build started: %s", strings.Join(order, ", "))

	var sess *neovim.Session
	if b.s.ed != nil {
		sess = b.s.ed.sess
	}
	cfg, root, done := b.cfg, b.s.root, b.done
	go func() {
		defer close(done)
		if sess != nil {
			_ = sess.Command("silent! wall") // build what the user sees
		}
		started := time.Now()
		res, err := build.Run(ctx, cfg, names, build.Options{Root: root,
			OnStep: func(p string, argv []string) { b.add("==> " + p + ": " + strings.Join(argv, " ")) },
			OnLine: func(_, line string) { b.add(line) },
		})
		elapsed := time.Since(started)
		b.s.dispatch(func() { b.finished(res, err, elapsed) })
	}()
}

func orAll(c *config.Config, names []string) []string {
	if len(names) == 0 {
		return build.Targets(c)
	}
	return names
}

// add queues an output line from the build goroutine; lines reach the list in batches.
func (b *buildPanel) add(line string) {
	b.mu.Lock()
	b.pending = append(b.pending, line)
	b.mu.Unlock()
	if b.queued.Swap(true) {
		return
	}
	b.s.dispatch(b.flush)
}

func (b *buildPanel) flush() {
	b.queued.Store(false)
	b.mu.Lock()
	more := b.pending
	b.pending = nil
	b.mu.Unlock()
	if len(more) == 0 {
		return
	}
	b.lines = append(b.lines, more...)
	if over := len(b.lines) - maxBuildLines; over > 0 {
		b.lines = append(b.lines[:0], b.lines[over:]...)
	}
	b.list.Refresh()
	b.list.ScrollToBottom()
}

func (b *buildPanel) finished(res build.Result, err error, elapsed time.Duration) {
	b.flush()
	b.running, b.cancel = false, nil
	b.run.Enable()
	b.stop.Disable()
	if err != nil {
		b.setState("Build could not start")
		b.s.Notify(notifications.Error, "Cannot build", err.Error())
		return
	}
	for _, st := range res.Steps {
		switch {
		case st.Err != "":
			b.lines = append(b.lines, "FAILED "+st.Profile+": "+st.Err)
		case st.Exit != 0:
			b.lines = append(b.lines, fmt.Sprintf("FAILED %s: exit status %d", st.Profile, st.Exit))
		}
		for _, d := range st.Diagnostics {
			b.diags = append(b.diags, problem{path: d.File, d: neovim.Diagnostic{
				Line: max(d.Line-1, 0), Col: max(d.Column-1, 0), Severity: severityOf(d.Severity),
				Message: d.Message, Source: st.Profile}})
		}
	}
	b.list.Refresh()
	b.list.ScrollToBottom()
	b.s.refreshProblems()
	errs, warns := 0, 0
	for _, p := range b.diags {
		switch p.d.Severity {
		case 1:
			errs++
		case 2:
			warns++
		}
	}
	took := elapsed.Round(10 * time.Millisecond)
	counts := fmt.Sprintf("%s, %s", countOf(errs, "error"), countOf(warns, "warning"))
	switch {
	case res.OK():
		b.setState(fmt.Sprintf("Built in %v · %s", took, counts))
		b.s.Notify(notifications.Success, "Build succeeded", counts)
	case len(res.Steps) > 0 && res.Steps[len(res.Steps)-1].Err == "stopped":
		b.setState("Build stopped")
	default:
		b.setState(fmt.Sprintf("Build failed · %s", counts))
		b.s.Notify(notifications.Error, "Build failed", counts+". Problems lists them.")
	}
	b.s.logf("Build finished: %s", b.state.Text)
}

func (b *buildPanel) setState(t string) { b.state.SetText(t) }

// halt stops the running build and every process it started.
func (b *buildPanel) halt() {
	if b.cancel != nil {
		b.cancel()
	}
}

func severityOf(s string) int {
	switch s {
	case "error":
		return 1
	case "warning":
		return 2
	case "note":
		return 3
	}
	return 4
}

func countOf(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// BuildRunning reports whether a build is running, for review renders.
func (s *Shell) BuildRunning() bool { return s.buildp.running }
