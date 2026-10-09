package shell

import (
	"fmt"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// problem is one diagnostic in the Problems panel.
type problem struct {
	path string
	d    neovim.Diagnostic
}

// buildProblems makes the Problems tab: every diagnostic Neovim knows about (from LSP or any
// other source), errors first; selecting one opens the file at that position.
func (s *Shell) buildProblems() *container.TabItem {
	s.problemList = widget.NewList(
		func() int { return len(s.problems) },
		func() fyne.CanvasObject {
			icon := kit.NewIcon(icons.CircleX, kit.Error)
			where := kit.NewText("", kit.Mono, kit.Secondary)
			where.TextSize = theme.TextCaption + 1
			msg := kit.NewText("", kit.Body, kit.Primary)
			return container.NewHBox(icon, where, msg)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			p := s.problems[i]
			row := o.(*fyne.Container).Objects
			icon := row[0].(*kit.Icon)
			icon.Name, icon.Role = severityIcon(p.d.Severity)
			icon.Refresh()
			rel, err := filepath.Rel(s.root, p.path)
			if err != nil {
				rel = p.path
			}
			row[1].(*kit.Text).SetText(fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), p.d.Line+1, p.d.Col+1))
			msg := p.d.Message
			if p.d.Source != "" {
				msg += "  (" + p.d.Source + ")"
			}
			row[2].(*kit.Text).SetText(msg)
		},
	)
	s.problemList.OnSelected = func(i widget.ListItemID) {
		s.problemList.UnselectAll()
		if i < len(s.problems) && s.ed != nil && s.ed.sess != nil {
			p, sess := s.problems[i], s.ed.sess
			go func() { _ = sess.GoTo(p.path, p.d.Line, p.d.Col) }()
			s.FocusEditor()
		}
	}
	s.problemsEmpty = placeholder(icons.TriangleAlert, "No problems",
		"Diagnostics from Neovim appear here as files are checked; build errors join them when builds run.")
	s.problemList.Hide()
	s.problemsTab = container.NewTabItem("Problems", container.NewStack(s.problemsEmpty, s.problemList))
	return s.problemsTab
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

// refreshProblems rebuilds the list from the editor's diagnostics.
func (s *Shell) refreshProblems() {
	s.problems = s.problems[:0]
	if s.ed != nil {
		for _, bd := range s.ed.diags {
			for _, d := range bd.items {
				s.problems = append(s.problems, problem{path: bd.path, d: d})
			}
		}
	}
	sort.Slice(s.problems, func(i, j int) bool {
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
	title := "Problems"
	if n := len(s.problems); n > 0 {
		title = fmt.Sprintf("Problems (%d)", n)
		s.problemsEmpty.Hide()
		s.problemList.Show()
	} else {
		s.problemList.Hide()
		s.problemsEmpty.Show()
	}
	if s.problemsTab.Text != title {
		s.problemsTab.Text = title
		s.dock.Refresh()
	}
	s.problemList.Refresh()
}
