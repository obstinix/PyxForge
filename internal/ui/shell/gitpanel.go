package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/git"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// gitPanel is the Git tab: the branch, the changed files grouped as Git groups them, staging,
// committing, and diffs against HEAD opened in the editor. Everything runs the local git
// command; nothing touches the network.
type gitPanel struct {
	s       *Shell
	tab     *container.TabItem
	body    *fyne.Container
	view    fyne.CanvasObject
	list    *widget.List
	rows    []gitRow
	branch  *kit.Text
	message *widget.Entry
	commit  *widget.Button
	history *fyne.Container

	repo    *git.Repo
	status  git.Status
	loaded  bool // a status has been read at least once
	running bool // a refresh is in flight
	again   bool // another refresh was asked for while one ran
	done    chan struct{}
}

// gitRow is a group heading or a file in the list.
type gitRow struct {
	heading string
	file    git.File
	staged  bool // the row is in the Staged group: its action unstages
}

func (s *Shell) buildGitPanel() *container.TabItem {
	g := &gitPanel{s: s}
	s.gitp = g
	g.branch = kit.NewText("", kit.Body, kit.Secondary)
	g.branch.TextSize = theme.TextCaption + 1
	refresh := kit.NewIconButton(icons.Refresh, "Refresh Git status", g.refresh)
	stageAll := kit.NewIconButton(icons.Plus, "Stage all changes", func() { g.apply("stage", true, nil) })
	unstageAll := kit.NewIconButton(icons.Minus, "Unstage all", func() { g.apply("unstage", true, nil) })
	bar := kit.Row(theme.TabBarHeight, container.NewHBox(kit.NewIcon(icons.GitBranch, kit.Secondary), g.branch,
		layout.NewSpacer(), stageAll, unstageAll, refresh))
	top := container.NewVBox(container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space1), bar), kit.NewRule(false))

	g.list = widget.NewList(func() int { return len(g.rows) }, g.createRow, g.updateRow)
	g.list.OnSelected = func(i widget.ListItemID) {
		g.list.UnselectAll()
		if i < len(g.rows) && g.rows[i].heading == "" {
			g.diff(g.rows[i].file)
		}
	}

	g.message = widget.NewMultiLineEntry()
	g.message.SetPlaceHolder("Commit message")
	g.message.Wrapping = fyne.TextWrapWord
	g.message.SetMinRowsVisible(4)
	g.commit = widget.NewButtonWithIcon("Commit", icons.Get(icons.GitCommit, theme.Current().Accent.OnPrimary), g.doCommit)
	g.commit.Importance = widget.HighImportance
	g.commit.Disable()
	g.history = container.NewVBox()
	recent := kit.Title("Recent commits")
	side := container.NewVBox(g.message, g.commit, container.NewPadded(recent), g.history)
	right := container.New(layout.NewCustomPaddedLayout(theme.Space2, theme.Space2, theme.Space2, theme.Space2), side)

	split := container.NewHSplit(g.list, container.NewVScroll(right))
	split.Offset = 0.62
	g.view = container.NewBorder(top, nil, nil, nil, split)
	g.body = container.NewStack(placeholder(icons.GitBranch, "Reading Git status", "The Git tab shows the files this checkout has changed."))
	g.tab = container.NewTabItem("Git", g.body)
	return g.tab
}

func (g *gitPanel) createRow() fyne.CanvasObject {
	letter := kit.NewText("", kit.Mono, kit.Secondary)
	letter.TextSize = theme.TextCaption + 1
	name := kit.NewText("", kit.Body, kit.Primary)
	// The folder gives way first in a narrow panel: it is cut with an ellipsis.
	where := widget.NewLabel("")
	where.Truncation = fyne.TextTruncateEllipsis
	where.SizeName = fynetheme.SizeNameCaptionText
	where.Importance = widget.LowImportance
	action := kit.NewIconButton(icons.Plus, "", nil)
	action.Side = 24
	row := container.NewBorder(nil, nil, container.NewHBox(letter, name), action, where)
	return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space2), row)
}

func (g *gitPanel) updateRow(i widget.ListItemID, o fyne.CanvasObject) {
	row := o.(*fyne.Container).Objects[0].(*fyne.Container)
	where := row.Objects[0].(*widget.Label)
	lead := row.Objects[1].(*fyne.Container)
	letter, name := lead.Objects[0].(*kit.Text), lead.Objects[1].(*kit.Text)
	action := row.Objects[2].(*kit.IconButton)
	r := g.rows[i]
	if r.heading != "" {
		letter.SetText("")
		name.Face, name.Role, name.Upper, name.TextSize = kit.Display, kit.Secondary, true, theme.TextCaption+1
		name.SetText(r.heading)
		where.SetText("")
		action.Hide()
		row.Refresh()
		return
	}
	name.Face, name.Role, name.Upper, name.TextSize = kit.Body, kit.Primary, false, 0
	code := r.file.Work
	if r.staged {
		code = r.file.Index
	}
	letter.Role = letterRole(code)
	letter.SetText(string(code))
	name.SetText(filepath.Base(filepath.FromSlash(r.file.Path)))
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(r.file.Path)))
	if dir == "." {
		dir = ""
	}
	if r.file.OrigPath != "" {
		dir = strings.TrimSpace(dir + "  ← " + r.file.OrigPath)
	}
	where.SetText(dir)
	file, staged := r.file, r.staged
	if staged {
		action.Icon, action.Label = icons.Minus, "Unstage "+file.Path
		action.OnTapped = func() { g.apply("unstage", false, []string{file.Path}) }
	} else {
		action.Icon, action.Label = icons.Plus, "Stage "+file.Path
		action.OnTapped = func() { g.apply("stage", false, []string{file.Path}) }
	}
	action.Show()
	action.Refresh()
	row.Refresh() // the texts changed width: lay the row out again
}

func letterRole(code byte) kit.Role {
	switch code {
	case 'A', '?':
		return kit.Success
	case 'D', 'U':
		return kit.Error
	case 'M', 'R', 'C', 'T':
		return kit.Warning
	}
	return kit.Secondary
}

// refresh reads the status again in the background. Requests made while one runs coalesce
// into one more read.
func (g *gitPanel) refresh() {
	if g.running {
		g.again = true
		return
	}
	g.running, g.done = true, make(chan struct{})
	repo, root, done := g.repo, g.s.root, g.done
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if repo == nil {
			repo, err = git.Open(ctx, root)
		}
		var st git.Status
		var log []string
		if err == nil {
			st, err = repo.Status(ctx)
		}
		if err == nil {
			log, _ = repo.Log(ctx, 8)
		}
		g.s.dispatch(func() { g.show(repo, st, log, err) })
	}()
}

func (g *gitPanel) show(repo *git.Repo, st git.Status, log []string, err error) {
	g.running = false
	defer func() {
		if g.again {
			g.again = false
			g.refresh()
		}
	}()
	switch {
	case errors.Is(err, git.ErrNoRepo):
		g.setBody(placeholder(icons.GitBranch, "Not a Git repository",
			"This folder is not in a Git working tree. To start one, run git init in the Terminal."))
		g.s.statusBranch.SetText("no repository")
		return
	case err != nil:
		g.setBody(placeholder(icons.TriangleAlert, "Git status unavailable", err.Error()))
		return
	}
	g.repo, g.status, g.loaded = repo, st, true
	g.rows = g.rows[:0]
	group := func(title string, staged bool, keep func(git.File) bool) {
		var files []git.File
		for _, f := range st.Files {
			if keep(f) {
				files = append(files, f)
			}
		}
		if len(files) == 0 {
			return
		}
		g.rows = append(g.rows, gitRow{heading: fmt.Sprintf("%s (%d)", title, len(files))})
		for _, f := range files {
			g.rows = append(g.rows, gitRow{file: f, staged: staged})
		}
	}
	group("Conflicts", false, git.File.Conflicted)
	group("Staged", true, func(f git.File) bool { return f.Staged() && !f.Conflicted() })
	group("Changes", false, func(f git.File) bool { return f.Unstaged() && !f.Untracked() && !f.Conflicted() })
	group("Untracked", false, git.File.Untracked)
	g.list.Refresh()

	branch := st.Branch
	if branch == "" {
		branch = "detached at " + shortHash(st.Head)
	}
	label := branch
	if st.Upstream != "" {
		label += fmt.Sprintf(" · %s ↑%d ↓%d", st.Upstream, st.Ahead, st.Behind)
	}
	if st.Head == "" {
		label += " · no commits yet"
	}
	g.branch.SetText(label)
	status := branch
	if n := len(st.Files); n > 0 {
		status += fmt.Sprintf(" · %d changed", n)
	}
	g.s.statusBranch.SetText(status)

	staged := 0
	for _, f := range st.Files {
		if f.Staged() {
			staged++
		}
	}
	g.commit.SetText("Commit")
	if staged > 0 {
		g.commit.SetText(fmt.Sprintf("Commit %s", countOf(staged, "file")))
		g.commit.Enable()
	} else {
		g.commit.Disable()
	}
	g.history.RemoveAll()
	for _, l := range log {
		t := widget.NewLabel(l)
		t.Truncation = fyne.TextTruncateEllipsis // long subjects must not widen the column
		t.SizeName = fynetheme.SizeNameCaptionText
		t.TextStyle.Monospace = true
		g.history.Add(t)
	}
	if len(st.Files) == 0 {
		g.rows = append(g.rows, gitRow{heading: "No changes"})
		g.list.Refresh()
	}
	g.setBody(g.view)
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

func (g *gitPanel) setBody(o fyne.CanvasObject) {
	if len(g.body.Objects) == 1 && g.body.Objects[0] == o {
		return
	}
	g.body.Objects = []fyne.CanvasObject{o}
	g.body.Refresh()
}

// apply stages or unstages paths (all changes with all), then refreshes.
func (g *gitPanel) apply(op string, all bool, paths []string) {
	repo := g.repo
	if repo == nil {
		return
	}
	if all {
		paths = []string{"."}
		if op == "unstage" {
			paths = nil
			for _, f := range g.status.Files {
				if f.Staged() {
					paths = append(paths, f.Path)
				}
			}
			if len(paths) == 0 {
				return
			}
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if op == "stage" {
			err = repo.Stage(ctx, paths...)
		} else {
			err = repo.Unstage(ctx, paths...)
		}
		g.s.dispatch(func() {
			if err != nil {
				g.s.Notify(notifications.Error, "Git "+op+" failed", err.Error())
			}
			g.refresh()
		})
	}()
}

// doCommit commits the staged files with the message in the box.
func (g *gitPanel) doCommit() {
	repo, msg := g.repo, g.message.Text
	if repo == nil {
		return
	}
	if strings.TrimSpace(msg) == "" {
		g.s.Notify(notifications.Info, "Write a commit message", "The message says what the commit changes and why.")
		g.s.win.Canvas().Focus(g.message)
		return
	}
	g.commit.Disable()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		hash, err := repo.Commit(ctx, msg)
		g.s.dispatch(func() {
			if err != nil {
				g.s.Notify(notifications.Error, "Commit failed", err.Error())
			} else {
				g.message.SetText("")
				g.s.Notify(notifications.Success, "Committed "+hash, firstLine(msg))
			}
			g.refresh()
		})
	}()
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// diff opens the file in the editor beside its committed text.
func (g *gitPanel) diff(f git.File) {
	repo := g.repo
	if repo == nil {
		return
	}
	abs := filepath.Join(repo.Root, filepath.FromSlash(f.Path))
	if _, err := os.Stat(abs); err != nil {
		g.s.Notify(notifications.Info, "Deleted file", f.Path+" is no longer on disk; Stage records the deletion.")
		return
	}
	if !g.s.startEditor() {
		g.s.OpenFile(abs)
		return
	}
	g.s.activate(regionEditor)
	sess, base := g.s.ed.sess, f.Path
	if f.OrigPath != "" {
		base = f.OrigPath
	}
	g.s.ed.do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		text, err := repo.Show(ctx, "HEAD", base)
		if err == nil {
			err = sess.Diff(abs, "HEAD:"+base, text)
		}
		if err != nil {
			g.s.dispatch(func() { g.s.Notify(notifications.Error, "Cannot show the diff", err.Error()) })
		}
	})
	if c := g.s.win.Canvas(); c != nil {
		c.Focus(g.s.ed.view)
	}
}
