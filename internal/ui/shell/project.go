package shell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/git"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// A minimal project is an explicitly named empty folder; existing machine/build setup
// remains responsible for pyxforge.toml. Never merge into or overwrite an existing folder.
func projectDestination(parent, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:<>"|?*`) || strings.HasSuffix(name, ".") {
		return "", errors.New("enter a project name without path separators or reserved filename characters")
	}
	if !filepath.IsAbs(parent) {
		return "", errors.New("select a destination directory")
	}
	info, err := os.Stat(parent)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("the destination must be a directory")
	}
	return filepath.Join(parent, name), nil
}

func createProject(parent, name string) (string, error) {
	path, err := projectDestination(parent, name)
	if err != nil {
		return "", err
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Shell) newProject() {
	s.projectDialog(false)
}

func (s *Shell) cloneRepository() {
	if s.cloneStop != nil {
		s.Notify(notifications.Info, "Clone in progress", "Wait for the current clone to finish.")
		return
	}
	s.projectDialog(true)
}

func (s *Shell) projectDialog(clone bool) {
	name, parent, remote := s.newDialogEntry(), s.newDialogEntry(), s.newDialogEntry()
	name.SetPlaceHolder("Project name")
	parent.SetPlaceHolder("Select a destination directory")
	remote.SetPlaceHolder("Repository URL or local repository path")
	browse := widget.NewButton("Browse…", func() { s.pickFolder("Select Destination", parent.SetText) })
	items := []*widget.FormItem{widget.NewFormItem("Name", name), widget.NewFormItem("Destination", container.NewBorder(nil, nil, nil, browse, parent))}
	title, verb := "New Project", "Create"
	if clone {
		title, verb = "Clone Git Repository", "Clone"
		items = append([]*widget.FormItem{widget.NewFormItem("Repository", remote)}, items...)
	}
	name.Validator = func(v string) error { _, err := projectDestination(parent.Text, v); return err }
	parent.OnChanged = func(string) { _ = name.Validate() }
	if clone {
		remote.Validator = func(v string) error {
			if strings.TrimSpace(v) == "" {
				return errors.New("enter a repository")
			}
			return nil
		}
	}
	d := dialog.NewForm(title, verb, "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		if clone {
			dest, err := projectDestination(parent.Text, name.Text)
			if err != nil {
				s.Notify(notifications.Error, title, err.Error())
				return
			}
			s.cloneAt(remote.Text, dest)
			return
		}
		path, err := createProject(parent.Text, name.Text)
		if err != nil {
			s.Notify(notifications.Error, "Could not create project", err.Error())
			return
		}
		s.openFolderAt(path)
	}, s.win)
	d.Resize(fyne.NewSize(560, d.MinSize().Height))
	s.submitForm(d, name, parent, remote)
	s.lastDialog = d
}

func (s *Shell) cloneAt(remote, dest string) {
	if s.cloneStop != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), git.NetworkTimeout)
	s.cloneStop = cancel
	s.Notify(notifications.Info, "Cloning repository", dest)
	go func() {
		defer cancel()
		_, err := git.Clone(ctx, strings.TrimSpace(remote), dest)
		s.dispatch(func() {
			s.cloneStop = nil
			if err != nil {
				s.Notify(notifications.Error, "Clone failed", err.Error())
				return
			}
			s.choose("Repository cloned", dest, choice{label: "Open Folder", primary: true, run: func() { s.openFolderAt(dest) }})
		})
	}()
}
