package shell

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/obstinix/PyxForge/internal/git"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Worktrees (#68): the repository checked out in other folders, each on its own branch. The
// palette lists them; one is opened in this window, or removed.

// worktrees lists the repository's worktrees in the palette, after Create Worktree….
func (g *gitPanel) worktrees() {
	repo := g.repo
	if repo == nil {
		g.s.Notify(notifications.Info, "Not a Git repository", "Git > Initialize Repository… starts one in this folder.")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		list, err := repo.Worktrees(ctx)
		g.s.dispatch(func() {
			if err != nil {
				g.s.Notify(notifications.Error, "Cannot list worktrees", err.Error())
				return
			}
			items := []commandpalette.Item{{Title: "Create Worktree…", Detail: "check a branch out in another folder", Icon: icons.Plus, Run: g.createWorktree}}
			for _, w := range list {
				name, detail := w.Branch, w.Path
				if name == "" {
					name = "detached at " + shortHash(w.Head)
				}
				switch {
				case sameFile(w.Path, repo.Root):
					detail += " · this window"
				case w.Prunable:
					detail += " · folder gone"
				}
				items = append(items, commandpalette.Item{Title: name, Detail: detail, Icon: icons.GitBranch, Run: func() { g.worktreeChosen(w) }})
			}
			g.s.palette.Show("Worktrees", items)
		})
	}()
}

// worktreeChosen offers what can be done with a worktree: open it here, or remove it.
func (g *gitPanel) worktreeChosen(w git.Worktree) {
	name := filepath.Base(w.Path)
	switch {
	case w.Prunable:
		g.s.choose("Forget "+name+"?", "Its folder, "+w.Path+", no longer exists.",
			choice{label: "Forget", primary: true, run: func() {
				g.gitOp("prune worktrees", func(ctx context.Context, r *git.Repo) error { return r.PruneWorktrees(ctx) }, func(err error) {
					if err != nil {
						g.s.Notify(notifications.Error, "Not forgotten", err.Error())
					}
				})
			}})
		return
	case sameFile(w.Path, g.repo.Root):
		g.s.Notify(notifications.Info, "This window shows "+name, "Choose another worktree to open or remove it.")
		return
	}
	choices := []choice{{label: "Open Here", primary: true, run: func() { g.s.openFolderAt(w.Path) }}}
	if !w.Main {
		choices = append(choices, choice{label: "Remove…", run: func() { g.removeWorktree(w) }})
	}
	g.s.choose(name, w.Path+"\nOn "+w.Branch+".", choices...)
}

// removeWorktree deletes a worktree's folder after asking; Git refuses one with changes not
// committed, and PyxForge does not force it.
func (g *gitPanel) removeWorktree(w git.Worktree) {
	g.s.choose("Remove the worktree "+filepath.Base(w.Path)+"?",
		"Deletes the folder "+w.Path+". The branch "+w.Branch+" and its commits stay. Git refuses if the folder has changes not committed.",
		choice{label: "Remove", primary: true, run: func() {
			g.gitOp("remove worktree", func(ctx context.Context, r *git.Repo) error { return r.RemoveWorktree(ctx, w.Path) }, func(err error) {
				if err != nil {
					g.s.Notify(notifications.Error, "Worktree not removed", withoutHints(err.Error()))
					return
				}
				g.s.Notify(notifications.Success, "Removed the worktree "+filepath.Base(w.Path), "")
			})
		}})
}

// createWorktree asks for a branch and a folder, beside the repository's by default, and
// checks the branch out there.
func (g *gitPanel) createWorktree() {
	repo := g.repo
	if repo == nil {
		return
	}
	branch, folder := g.s.newDialogEntry(), g.s.newDialogEntry()
	branch.SetPlaceHolder("feature/vga-driver")
	fresh := widget.NewCheck("A new branch, from the current commit", nil)
	fresh.SetChecked(true)
	edited := false
	folder.OnChanged = func(string) { edited = true }
	branch.OnChanged = func(b string) {
		if !edited || folder.Text == "" {
			folder.SetText(filepath.Join(filepath.Dir(repo.Root), filepath.Base(repo.Root)+"-"+strings.NewReplacer("/", "-", `\`, "-").Replace(strings.TrimSpace(b))))
			edited = false
		}
	}
	browse := widget.NewButton("Browse…", func() {
		g.s.pickFolder("Folder for the worktree", func(dir string) { folder.SetText(filepath.Join(dir, filepath.Base(folder.Text))) })
	})
	d := dialog.NewForm("Create Worktree", "Create", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Branch", branch), widget.NewFormItem("", fresh),
		widget.NewFormItem("Folder", container.NewBorder(nil, nil, nil, browse, folder)),
	}, func(ok bool) {
		if ok {
			g.addWorktree(strings.TrimSpace(folder.Text), strings.TrimSpace(branch.Text), fresh.Checked)
		}
	}, g.s.win)
	d.Resize(fyne.NewSize(560, d.MinSize().Height))
	g.s.submitForm(d, branch, folder)
	g.s.win.Canvas().Focus(branch)
	g.s.lastDialog = d
}

func (g *gitPanel) addWorktree(path, branch string, create bool) {
	g.gitOp("add worktree", func(ctx context.Context, r *git.Repo) error { return r.AddWorktree(ctx, path, branch, create) }, func(err error) {
		if err != nil {
			g.s.Notify(notifications.Error, "Worktree not created", withoutHints(err.Error()))
			return
		}
		g.s.logf("Git: worktree %s at %s", branch, path)
		g.s.choose("Worktree created", branch+" is checked out in "+path+".",
			choice{label: "Open Here", primary: true, run: func() { g.s.openFolderAt(path) }})
	})
}
