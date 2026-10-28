package shell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	if s.workspaceStatus == workspaceLoading {
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
	s.confirmWorkspaceChange(func() { s.replaceWorkspace("").showWelcome() })
}

// A workspace owns its editor, watchers and running tools. Retire that shell and create
// fresh services in the SAME native window using the existing panel constructors. Late
// callbacks stay bound to a closed shell, rather than to a different project.
func (s *Shell) replaceWorkspace(root string) *Shell {
	size := s.win.Canvas().Size()
	ew, dh, iw := s.bench.explorerW, s.bench.dockH, s.bench.inspectorW
	explorerOn, dockOn, inspectorOn := s.bench.explorerOn, s.bench.dockOn, s.bench.inspectorOn
	s.Shutdown()
	for _, sc := range s.shortcuts {
		s.win.Canvas().RemoveShortcut(sc)
	}
	s.win.Canvas().Unfocus()
	opts := s.opts
	opts.window = s.win
	next := NewWithOptions(s.app, root, opts)
	s.replacement = next
	next.bench.explorerW, next.bench.dockH, next.bench.inspectorW = ew, dh, iw
	next.bench.explorerOn, next.bench.dockOn, next.bench.inspectorOn = explorerOn, dockOn, inspectorOn
	next.win.Resize(size)
	next.relayout()
	if root != "" {
		next.gitp.refresh()
	}
	return next
}

func (s *Shell) confirmWorkspaceChange(change func()) {
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
	s.Notify(notifications.Info, "No Folder Open", "Open a folder first.")
	return false
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
