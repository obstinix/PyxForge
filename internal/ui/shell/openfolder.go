package shell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/workspace"
)

type workspaceStatus uint8

const (
	workspaceEmpty workspaceStatus = iota
	workspaceLoading
	workspaceOpen
	workspaceFailed
)

func workspaceTitle(root string) string {
	if root == "" {
		return "PyxForge"
	}
	return "PyxForge · " + filepath.Base(root)
}

func workspaceName(root string) string {
	if root == "" {
		return "No Folder Open"
	}
	return filepath.Base(root)
}

func (s *Shell) openFolder() { s.pickFolder("Open Folder", s.openFolderAt) }

// Validate before disturbing the current workspace. Failed selections never enter history.
func (s *Shell) openFolderAt(dir string) {
	if s.workspaceStatus == workspaceLoading || s.switching {
		return
	}
	s.workspaceStatus, s.workspaceError = workspaceLoading, nil
	go func() {
		w, err := workspace.Discover(dir)
		if err == nil {
			var f *os.File
			f, err = os.Open(w.Dir)
			if err == nil {
				_, err = f.ReadDir(1)
				_ = f.Close()
				if err == io.EOF {
					err = nil
				}
			}
		}
		s.dispatch(func() {
			if err != nil {
				s.workspaceStatus, s.workspaceError = workspaceFailed, err
				s.Notify(notifications.Error, "Could not open the folder", err.Error())
				return
			}
			s.workspaceStatus = workspaceEmpty
			if s.root != "" {
				s.workspaceStatus = workspaceOpen
			}
			if sameFile(s.root, w.Dir) {
				s.addRecent(prefRecentFolders, w.Dir)
				return
			}
			s.confirmWorkspaceChange(func() { s.replaceWorkspace(w.Dir) })
		})
	}()
}

func (s *Shell) closeWorkspace() {
	if s.workspaceStatus == workspaceLoading {
		return
	}
	if s.root == "" {
		s.showWelcome()
		return
	}
	s.confirmWorkspaceChange(func() { s.replaceWorkspace("") })
}

// A workspace owns its editor, watchers and running tools. Retire it off the UI thread, then
// create fresh services in the SAME native window. Late callbacks stay bound to the closed shell.
func (s *Shell) replaceWorkspace(root string) {
	if s.switching || s.closed.Load() {
		return
	}
	s.switching = true
	size := s.win.Canvas().Size()
	ew, dh, iw := s.bench.explorerW, s.bench.dockH, s.bench.inspectorW
	explorerOn, dockOn, inspectorOn := s.bench.explorerOn, s.bench.dockOn, s.bench.inspectorOn
	s.closed.Store(true)
	if s.cloneDialog != nil {
		s.cloneDialog.Hide()
	}
	s.saveState()
	for _, sc := range s.shortcuts {
		s.win.Canvas().RemoveShortcut(sc)
	}
	s.app.Lifecycle().SetOnEnteredForeground(func() {})
	s.win.SetContent(container.NewCenter(widget.NewLabel("Stopping workspace activity…")))
	s.win.Canvas().Unfocus()
	opts := s.opts
	opts.window = s.win
	opts.group = s.group
	go func() {
		s.stopAll()
		s.post(func() {
			next := NewWithOptions(s.app, root, opts)
			next.bench.explorerW, next.bench.dockH, next.bench.inspectorW = ew, dh, iw
			next.bench.explorerOn, next.bench.dockOn, next.bench.inspectorOn = explorerOn, dockOn, inspectorOn
			next.win.Resize(size)
			next.relayout()
			if root != "" {
				next.gitp.refresh()
			} else {
				next.showWelcome()
			}
		})
	}()
}

func (s *Shell) confirmWorkspaceChange(change func()) {
	if s.switching {
		return
	}
	work := s.workspaceWork()
	if len(work) > 0 {
		s.choose("Stop work and change workspace?",
			"Changing the workspace stops "+strings.Join(work, ", ")+". This work cannot continue without its current workspace.",
			choice{label: "Stop and Continue", primary: true, run: func() { s.confirmUnsavedWorkspaceChange(change) }})
		return
	}
	s.confirmUnsavedWorkspaceChange(change)
}

func (s *Shell) workspaceWork() []string {
	var work []string
	if s.buildp != nil && s.buildp.running {
		work = append(work, "the running build")
	}
	if s.mach != nil && (s.mach.starting || s.mach.inst != nil || s.mach.dbg != nil) {
		work = append(work, "QEMU/GDB")
	}
	if s.term != nil {
		for _, session := range s.term.sessions {
			if !session.done {
				work = append(work, "terminal sessions")
				break
			}
		}
	}
	if s.gitp != nil && s.gitp.netBusy != "" {
		work = append(work, "the Git network operation")
	}
	if s.cloneStop != nil {
		work = append(work, "the repository clone")
	}
	return work
}

func (s *Shell) confirmUnsavedWorkspaceChange(change func()) {
	if s.ed == nil || s.ed.sess == nil {
		change()
		return
	}
	names, err := s.ed.sess.ModifiedFiles()
	if err != nil {
		s.Notify(notifications.Error, "Could not check unsaved files", err.Error())
		return
	}
	if len(names) == 0 {
		change()
		return
	}
	s.confirmUnsaved(fmt.Sprintf("Save changes to %s?", strings.Join(names, ", ")), func() {
		left, err := s.ed.sess.SaveAll()
		if err != nil {
			s.Notify(notifications.Error, "Not saved; workspace stays open", err.Error())
			return
		}
		if len(left) == 0 {
			change()
		}
	}, change)
}

func (s *Shell) requireWorkspace() bool {
	if s.root != "" {
		return true
	}
	s.Notify(notifications.Info, "No Folder Open", "Open a project folder first. A file opened on its own can be built after you open its discovered project.")
	return false
}

func (s *Shell) offerProjectForFile(path string) {
	if s.root != "" || s.switching {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	go func() {
		w, err := workspace.Discover(filepath.Dir(abs))
		if err != nil || w.ProjectFile == "" && w.GitRoot == "" {
			return
		}
		root, kind := w.Root, "project"
		if w.ProjectFile == "" {
			root, kind = w.GitRoot, "Git repository"
		}
		s.dispatch(func() {
			if s.root != "" || s.switching || s.projectOffers[root] {
				return
			}
			s.projectOffers[root] = true
			s.choose("Open the file's project?",
				fmt.Sprintf("%s is in the %s %s. Open that folder as the workspace to enable project tools?",
					filepath.Base(abs), filepath.Base(root), kind),
				choice{label: "Open Project", primary: true, run: func() { s.openFolderAt(root) }},
				choice{label: "Keep File Only", run: func() {}})
		})
	}()
}

// Used only for explicit file dialogs and process launches, never assigned to workspace root.
func (s *Shell) processFolder() string {
	if s.root != "" {
		return s.root
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return os.TempDir()
}
