package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// buildSource prefixes the source of build diagnostics in the editor, so Problems, which lists
// the build's own copy, does not show them twice.
const buildSource = "build: "

// problem is one entry in the Problems panel. A build message about no particular file (a
// linker that cannot find a library) has no path; one about an object file's section has a
// path but no line.
type problem struct {
	path    string
	d       neovim.Diagnostic // Line and Col 0-based
	noLine  bool
	tool    string // who reported a problem without a path
	profile string // the build profile, for build problems
}

// located reports whether the problem can be opened in the editor.
func (p problem) located() bool { return p.path != "" }

// buildProblems makes the Problems tab: the diagnostics Neovim knows about (language servers
// and the last build), errors first, with a summary and next/previous navigation. Selecting
// one opens the file at its position.
func (s *Shell) buildProblems() *container.TabItem {
	s.problemList = widget.NewList(
		func() int { return len(s.problems) },
		func() fyne.CanvasObject {
			icon := kit.NewIcon(icons.CircleX, kit.Error)
			where := kit.NewText("", kit.Mono, kit.Secondary)
			where.TextSize = theme.TextCaption + 1
			msg := widget.NewLabel("")
			msg.Truncation = fyne.TextTruncateEllipsis
			msg.SizeName = fynetheme.SizeNameText
			row := container.NewBorder(nil, nil, container.NewHBox(icon, where), nil, msg)
			return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space2), row)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			p := s.problems[i]
			row := o.(*fyne.Container).Objects[0].(*fyne.Container)
			lead := row.Objects[1].(*fyne.Container)
			icon, where := lead.Objects[0].(*kit.Icon), lead.Objects[1].(*kit.Text)
			icon.Name, icon.Role = severityIcon(p.d.Severity)
			icon.Refresh()
			where.SetText(s.problemPlace(p))
			msg := p.d.Message
			switch {
			case p.d.Source != "":
				msg += "  (" + p.d.Source + ")"
			case p.profile != "":
				msg += "  (" + p.profile + ")"
			}
			row.Objects[0].(*widget.Label).SetText(msg)
			row.Refresh()
		},
	)
	s.problemList.OnSelected = func(i widget.ListItemID) {
		s.problemList.UnselectAll()
		s.goToProblem(i)
	}
	s.problemsEmpty = placeholder(icons.TriangleAlert, "No problems",
		"Diagnostics from language servers appear here as files are checked; build errors and warnings join them when builds run.")
	s.problemSummary = kit.NewText("", kit.Body, kit.Secondary)
	s.problemSummary.TextSize = theme.TextCaption + 1
	prev := kit.NewIconButton(icons.ChevronUp, "Previous Problem (Ctrl+Shift+F7)", func() { s.stepProblem(-1) })
	next := kit.NewIconButton(icons.ChevronDown, "Next Problem (Ctrl+Shift+F8)", func() { s.stepProblem(1) })
	bar := kit.Row(theme.TabBarHeight, container.NewHBox(s.problemSummary, layout.NewSpacer(), prev, next))
	top := container.NewVBox(container.New(layout.NewCustomPaddedLayout(0, 0, theme.Space2, theme.Space1), bar), kit.NewRule(false))
	s.problemView = container.NewBorder(top, nil, nil, nil, s.problemList)
	s.problemsTab = container.NewTabItem("Problems", container.NewStack(s.problemsEmpty, s.problemView))
	s.problemView.Hide()
	s.problemAt = -1
	return s.problemsTab
}

// problemPlace is where a problem is: path:line:column relative to the workspace, the file
// alone, or the tool that reported it.
func (s *Shell) problemPlace(p problem) string {
	if !p.located() {
		if p.tool != "" {
			return p.tool
		}
		return p.profile
	}
	rel, err := filepath.Rel(s.root, p.path)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = p.path
	}
	rel = filepath.ToSlash(rel)
	if p.noLine {
		return rel
	}
	return fmt.Sprintf("%s:%d:%d", rel, p.d.Line+1, p.d.Col+1)
}

func severityIcon(sev int) (icons.Name, kit.Role) {
	switch sev {
	case 1:
		return icons.CircleX, kit.Error
	case 2:
		return icons.TriangleAlert, kit.Warning
	}
	return icons.Info, kit.Secondary
}

// goToProblem opens problem i: its file at its position, or the Build tab for a problem with
// no file. A file that no longer exists is reported instead.
func (s *Shell) goToProblem(i int) {
	if i < 0 || i >= len(s.problems) {
		return
	}
	s.problemAt = i
	s.problemList.ScrollTo(i)
	p := s.problems[i]
	if !p.located() {
		s.showDockTab(slices.Index(s.dock.Items, s.buildp.tab))
		return
	}
	if _, err := os.Stat(p.path); err != nil {
		s.Notify(notifications.Info, "File not found", s.problemPlace(p)+" no longer exists; build again to refresh the list.")
		return
	}
	if !s.startEditor() {
		s.OpenFile(p.path)
		return
	}
	sess := s.ed.sess
	s.ed.do(func() { _ = sess.GoTo(p.path, p.d.Line, p.d.Col) })
	s.FocusEditor()
}

// stepProblem moves to the next (1) or previous (-1) problem that has a file, wrapping.
func (s *Shell) stepProblem(step int) {
	n := len(s.problems)
	if n == 0 {
		s.Notify(notifications.Info, "No problems", "Nothing to go to.")
		return
	}
	i := s.problemAt
	for range n {
		i = ((i+step)%n + n) % n
		if s.problems[i].located() {
			s.goToProblem(i)
			return
		}
	}
	s.goToProblem(i) // only problems without files: show the build output
}

// refreshProblems rebuilds the list from the last build and the editor's diagnostics.
func (s *Shell) refreshProblems() {
	s.problems = append(s.problems[:0], s.buildp.diags...)
	if s.ed != nil {
		for _, bd := range s.ed.diags {
			for _, d := range bd.items {
				if strings.HasPrefix(d.Source, buildSource) {
					continue // the build's own entry is already listed
				}
				s.problems = append(s.problems, problem{path: bd.path, d: d})
			}
		}
	}
	sort.SliceStable(s.problems, func(i, j int) bool {
		a, b := s.problems[i], s.problems[j]
		if a.d.Severity != b.d.Severity {
			return a.d.Severity < b.d.Severity
		}
		if a.path != b.path {
			return a.path < b.path
		}
		if a.d.Line != b.d.Line {
			return a.d.Line < b.d.Line
		}
		return a.d.Col < b.d.Col
	})
	var counts [5]int
	for _, p := range s.problems {
		counts[min(max(p.d.Severity, 1), 4)]++
	}
	title := "Problems"
	if n := len(s.problems); n > 0 {
		title = fmt.Sprintf("Problems (%d)", n)
		s.problemsEmpty.Hide()
		s.problemView.Show()
	} else {
		s.problemView.Hide()
		s.problemsEmpty.Show()
	}
	s.problemSummary.SetText(fmt.Sprintf("%s · %s · %s", countOf(counts[1], "error"), countOf(counts[2], "warning"),
		countOf(counts[3]+counts[4], "message")))
	if s.problemsTab.Text != title {
		s.problemsTab.Text = title
		s.dock.Refresh()
	}
	if s.problemAt >= len(s.problems) {
		s.problemAt = -1
	}
	s.problemList.Refresh()
}
