package shell

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// captureState records the layout and open files for the next session.
func (s *Shell) captureState() workspace.State {
	st := workspace.State{Root: s.root, Expanded: s.explorer.Expanded(), PanelTab: s.dock.Selected().Text,
		Build: s.buildp.pick.Selected}
	st.Explorer, st.Panel, st.Inspector = &s.bench.explorerOn, &s.bench.dockOn, &s.bench.inspectorOn
	if strings.HasPrefix(st.PanelTab, "Problems") {
		st.PanelTab = "Problems" // the title carries a count
	}
	if size := s.win.Canvas().Size(); size.Width > 0 {
		st.Width, st.Height = size.Width, size.Height
	}
	selected := s.editors.Selected()
	for _, it := range s.editors.Items {
		p := s.tabFile(it)
		if p == "" {
			continue
		}
		st.Files = append(st.Files, p)
		if it == selected {
			st.Active = p
		}
	}
	return st
}

// tabFile is the file an editor tab shows, or "" for other tabs (Settings).
func (s *Shell) tabFile(it *container.TabItem) string {
	if s.ed != nil {
		if bt := s.ed.byTab[it]; bt != nil {
			return bt.path
		}
	}
	for p, t := range s.open {
		if t == it {
			return p
		}
	}
	return ""
}

// saveState writes the workspace state, when this shell keeps one.
func (s *Shell) saveState() {
	if s.states == nil {
		return
	}
	st := s.captureState()
	if err := s.states.Save(st); err != nil {
		s.logf("Workspace layout not saved: %v", err)
		return
	}
	s.lastSaved, _ = json.Marshal(st)
}

// autosave writes the workspace state when it differs from what was last written, so a crash
// or a forced exit loses at most a few seconds of layout changes.
func (s *Shell) autosave() {
	if s.states == nil || s.closed.Load() {
		return
	}
	if data, err := json.Marshal(s.captureState()); err == nil && !bytes.Equal(data, s.lastSaved) {
		s.saveState()
	}
}

// startAutosave runs autosave on the UI thread every interval until Shutdown.
func (s *Shell) startAutosave(every time.Duration) {
	t := time.NewTicker(every)
	go func() {
		defer t.Stop()
		for range t.C {
			if s.closed.Load() {
				return
			}
			s.dispatch(s.autosave)
		}
	}()
}

// restoreState brings back the previous session's layout and files.
func (s *Shell) restoreState() {
	if s.states == nil {
		return
	}
	st := s.states.Load(s.root)
	if st.Explorer != nil {
		s.bench.explorerOn = *st.Explorer
	}
	if st.Panel != nil {
		s.bench.dockOn = *st.Panel
	}
	if st.Inspector != nil {
		s.bench.inspectorOn = *st.Inspector
		s.inspectorAuto = false
	}
	s.relayout()
	for i, it := range s.dock.Items {
		if strings.HasPrefix(it.Text, st.PanelTab) && st.PanelTab != "" && it != s.term.tab {
			s.dock.SelectIndex(i) // the terminal is not started on its own at launch
		}
	}
	if st.Build != "" {
		s.app.Preferences().SetString(prefBuildPick, st.Build)
	}
	if st.Width >= 640 && st.Height >= 400 {
		s.win.Resize(fyne.NewSize(st.Width, st.Height))
	}
	s.explorer.Expand(st.Expanded)
	if len(st.Files) > 0 {
		s.openFiles(st.Files, st.Active)
	}
}

// openFiles opens several files in order, then selects active. The editor opens them one
// after another, so tabs keep their order.
func (s *Shell) openFiles(files []string, active string) {
	if !s.startEditor() {
		for _, f := range files {
			s.OpenFile(f)
		}
		if active != "" {
			s.OpenFile(active)
		}
		return
	}
	s.activate(regionEditor)
	sess := s.ed.sess
	order := append(slices.Clone(files), active) // never append into the caller's slice
	s.ed.do(func() {
		for _, f := range order {
			if f == "" {
				continue
			}
			if err := sess.Open(f); err != nil {
				s.dispatch(func() { s.logf("Not reopened: %s (%v)", filepath.Base(f), err) })
			}
		}
	})
}

// newEntry asks for a path relative to the workspace and creates a file or a folder there.
// The entry starts in the active file's folder.
func (s *Shell) newEntry(dir bool) {
	start := ""
	if p := s.activeFile(); p != "" {
		if rel, err := filepath.Rel(s.root, filepath.Dir(p)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			start = filepath.ToSlash(rel) + "/"
		}
	}
	what, verb := "File", "Create File"
	if dir {
		what, verb = "Folder", "Create Folder"
	}
	entry := widget.NewEntry()
	entry.SetText(start)
	entry.SetPlaceHolder("path relative to " + filepath.Base(s.root))
	d := dialog.NewForm("New "+what, verb, "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Name", entry)}, func(ok bool) {
			if !ok {
				return
			}
			p, err := s.explorer.Create(entry.Text, dir)
			if err != nil {
				s.Notify(notifications.Error, "Cannot create "+strings.ToLower(what), err.Error())
				return
			}
			s.explorer.Reveal(p)
			if !dir {
				s.OpenFile(p)
			}
		}, s.win)
	d.Resize(fyne.NewSize(420, d.MinSize().Height))
	d.Show()
	s.win.Canvas().Focus(entry)
	s.lastDialog = d
}

// activeFile is the file in the selected editor tab, or "".
func (s *Shell) activeFile() string {
	if it := s.editors.Selected(); it != nil {
		return s.tabFile(it)
	}
	return ""
}

// revealActive shows the active file in the explorer.
func (s *Shell) revealActive() {
	p := s.activeFile()
	if p == "" {
		s.Notify(notifications.Info, "No file to reveal", "Open a file in the editor first.")
		return
	}
	if !s.bench.explorerOn {
		s.ToggleExplorer()
	}
	s.explorer.Reveal(p)
	s.FocusExplorer()
}
