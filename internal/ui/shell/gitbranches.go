package shell

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/git"
	"github.com/obstinix/PyxForge/internal/ui/commandpalette"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// choice is one button of a decision dialog.
type choice struct {
	label   string
	primary bool
	run     func()
}

// choose asks a question with a button per choice, and Cancel.
func (s *Shell) choose(title, message string, choices ...choice) {
	var d dialog.Dialog
	buttons := container.NewHBox()
	cancel := widget.NewButton("Cancel", func() { d.Hide() })
	buttons.Add(cancel)
	for _, c := range choices {
		b := widget.NewButton(c.label, func() { d.Hide(); c.run() })
		if c.primary {
			b.Importance = widget.HighImportance
		}
		buttons.Add(b)
	}
	text := widget.NewLabel(message)
	text.Wrapping = fyne.TextWrapWord
	content := container.NewBorder(nil, buttons, nil, nil, text)
	d = dialog.NewCustomWithoutButtons(title, content, s.win)
	d.Resize(fyne.NewSize(460, content.MinSize().Height+80))
	d.Show()
	s.lastDialog = d
}

// gitOp runs a Git operation off the UI thread, then reports and refreshes on it.
func (g *gitPanel) gitOp(what string, op func(ctx context.Context, r *git.Repo) error, done func(err error)) {
	repo := g.repo
	if repo == nil {
		g.s.Notify(notifications.Info, "Not a Git repository", "Run git init in the Terminal to start one.")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		err := op(ctx, repo)
		g.s.dispatch(func() {
			if err != nil {
				g.s.logf("Git %s failed: %v", what, err)
			}
			if done != nil {
				done(err)
			}
			g.refresh()
		})
	}()
}

// chooseBranch lists the branches in the palette; choosing one switches to it.
func (g *gitPanel) chooseBranch() {
	repo := g.repo
	if repo == nil {
		g.refresh()
		g.s.Notify(notifications.Info, "Not a Git repository", "Run git init in the Terminal to start one.")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		list, err := repo.Branches(ctx)
		g.s.dispatch(func() {
			if err != nil {
				g.s.Notify(notifications.Error, "Cannot list branches", err.Error())
				return
			}
			items := []commandpalette.Item{{Title: "Create Branch…", Icon: icons.Plus, Run: g.createBranch}}
			for _, b := range list {
				detail := b.Subject
				if b.Current {
					detail = "current"
				}
				items = append(items, commandpalette.Item{Title: b.Name, Detail: detail, Icon: icons.GitBranch,
					Run: func() {
						if !b.Current {
							g.switchBranch(b.Name)
						}
					}})
			}
			g.s.palette.Show("Switch to a branch", items)
		})
	}()
}

// switchBranch checks out a branch. With uncommitted changes it asks first: Git carries
// changes that do not conflict to the other branch and refuses the switch otherwise.
func (g *gitPanel) switchBranch(name string) {
	if n := len(g.status.Files); n > 0 {
		g.s.choose("Switch to "+name+"?",
			fmt.Sprintf("%s uncommitted changes. Git brings them along to %s when they do not conflict with it, and refuses to switch when they do. Stashing them first leaves %s clean; Pop Stash brings them back.",
				countOf(n, "file has"), name, name),
			choice{label: "Switch", run: func() { g.doSwitch(name, false) }},
			choice{label: "Stash and Switch", primary: true, run: func() { g.doSwitch(name, true) }})
		return
	}
	g.doSwitch(name, false)
}

func (g *gitPanel) doSwitch(name string, stashFirst bool) {
	g.gitOp("switch", func(ctx context.Context, r *git.Repo) error {
		if stashFirst {
			if err := r.StashPush(ctx, "before switching to "+name, true); err != nil && !errors.Is(err, git.ErrNothingToStash) {
				return err
			}
		}
		return r.Switch(ctx, name)
	}, func(err error) {
		if err != nil {
			g.s.Notify(notifications.Error, "Not switched to "+name, err.Error())
			return
		}
		g.s.Notify(notifications.Success, "Switched to "+name, "")
		g.afterCheckout()
	})
}

// afterCheckout shows the files the checkout changed: the explorer lists them again and
// Neovim reloads buffers whose files changed on disk.
func (g *gitPanel) afterCheckout() {
	g.s.explorer.Reload()
	if g.s.ed != nil && g.s.ed.sess != nil {
		sess := g.s.ed.sess
		go func() { _ = sess.Command("silent! checktime") }()
	}
}

// createBranch asks for a name and creates the branch at HEAD, switching to it if asked.
func (g *gitPanel) createBranch() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("feature/vga-driver")
	switchTo := widget.NewCheck("Switch to it", nil)
	switchTo.SetChecked(true)
	d := dialog.NewForm("Create Branch", "Create", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Name", entry), widget.NewFormItem("", switchTo)}, func(ok bool) {
			if ok {
				g.createBranchNamed(entry.Text, switchTo.Checked)
			}
		}, g.s.win)
	d.Resize(fyne.NewSize(420, d.MinSize().Height))
	d.Show()
	g.s.win.Canvas().Focus(entry)
	g.s.lastDialog = d
}

func (g *gitPanel) createBranchNamed(name string, switchTo bool) {
	g.gitOp("create branch", func(ctx context.Context, r *git.Repo) error { return r.CreateBranch(ctx, name, switchTo) },
		func(err error) {
			if err != nil {
				g.s.Notify(notifications.Error, "Branch not created", err.Error())
				return
			}
			g.s.Notify(notifications.Success, "Created branch "+name, "")
			if switchTo {
				g.afterCheckout()
			}
		})
}

// deleteBranch lists the other branches; the one chosen is deleted after a confirmation, and
// a second one if its commits exist on no other branch.
func (g *gitPanel) deleteBranch() {
	repo := g.repo
	if repo == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		list, err := repo.Branches(ctx)
		g.s.dispatch(func() {
			if err != nil {
				g.s.Notify(notifications.Error, "Cannot list branches", err.Error())
				return
			}
			var items []commandpalette.Item
			for _, b := range list {
				if b.Current {
					continue
				}
				items = append(items, commandpalette.Item{Title: b.Name, Detail: b.Subject, Icon: icons.GitBranch,
					Run: func() {
						g.s.choose("Delete branch "+b.Name+"?", "The branch name is removed. Its commits stay on any other branch that has them.",
							choice{label: "Delete", primary: true, run: func() { g.deleteBranchNamed(b.Name, false) }})
					}})
			}
			if len(items) == 0 {
				g.s.Notify(notifications.Info, "No other branches", "The current branch cannot be deleted.")
				return
			}
			g.s.palette.Show("Delete a branch", items)
		})
	}()
}

func (g *gitPanel) deleteBranchNamed(name string, force bool) {
	g.gitOp("delete branch", func(ctx context.Context, r *git.Repo) error { return r.DeleteBranch(ctx, name, force) },
		func(err error) {
			switch {
			case errors.Is(err, git.ErrNotMerged):
				g.s.choose(name+" is not merged",
					name+" has commits that no other branch has. Deleting it leaves them reachable only through Git's reflog, for a limited time.",
					choice{label: "Delete Anyway", run: func() { g.deleteBranchNamed(name, true) }})
			case err != nil:
				g.s.Notify(notifications.Error, "Branch not deleted", err.Error())
			default:
				g.s.Notify(notifications.Success, "Deleted branch "+name, "")
			}
		})
}

// stashChanges asks for a message and saves the uncommitted changes.
func (g *gitPanel) stashChanges() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("What these changes are")
	untracked := widget.NewCheck("Include untracked files", nil)
	untracked.SetChecked(true)
	d := dialog.NewForm("Stash Changes", "Stash", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Message", entry), widget.NewFormItem("", untracked)}, func(ok bool) {
			if ok {
				g.stashNamed(entry.Text, untracked.Checked)
			}
		}, g.s.win)
	d.Resize(fyne.NewSize(420, d.MinSize().Height))
	d.Show()
	g.s.win.Canvas().Focus(entry)
	g.s.lastDialog = d
}

func (g *gitPanel) stashNamed(message string, untracked bool) {
	g.gitOp("stash", func(ctx context.Context, r *git.Repo) error { return r.StashPush(ctx, message, untracked) },
		func(err error) {
			switch {
			case errors.Is(err, git.ErrNothingToStash):
				g.s.Notify(notifications.Info, "Nothing to stash", "The working tree has no changes.")
			case err != nil:
				g.s.Notify(notifications.Error, "Not stashed", err.Error())
			default:
				g.s.Notify(notifications.Success, "Changes stashed", message)
				g.afterCheckout()
			}
		})
}

// applyStash restores a stash entry; pop also removes it when it applies cleanly.
func (g *gitPanel) applyStash(ref string, pop bool) {
	what := "apply"
	if pop {
		what = "pop"
	}
	g.gitOp("stash "+what, func(ctx context.Context, r *git.Repo) error { return r.StashApply(ctx, ref, pop) },
		func(err error) {
			if err != nil {
				msg := err.Error()
				if pop {
					msg += "\nThe stash entry was kept."
				}
				g.s.Notify(notifications.Error, "Stash not applied", msg)
			} else {
				g.s.Notify(notifications.Success, "Stash applied", ref)
			}
			g.afterCheckout()
		})
}

// dropStash deletes a stash entry after a confirmation.
func (g *gitPanel) dropStash(st git.Stash) {
	g.s.choose("Drop "+st.Ref+"?", st.Message+"\n\nIts changes are deleted. Apply it first to keep them.",
		choice{label: "Drop", primary: true, run: func() {
			g.gitOp("stash drop", func(ctx context.Context, r *git.Repo) error { return r.StashDrop(ctx, st.Ref) },
				func(err error) {
					if err != nil {
						g.s.Notify(notifications.Error, "Stash not dropped", err.Error())
					}
				})
		}})
}

// chooseStash lists the stash entries in the palette for an action.
func (g *gitPanel) chooseStash(title string, run func(git.Stash)) {
	if len(g.stashes) == 0 {
		g.s.Notify(notifications.Info, "No stashes", "Stash Changes saves uncommitted work.")
		return
	}
	var items []commandpalette.Item
	for _, st := range g.stashes {
		items = append(items, commandpalette.Item{Title: st.Message, Detail: st.Ref, Icon: icons.List, Run: func() { run(st) }})
	}
	g.s.palette.Show(title, items)
}

// stashRows lays out the stash list beside the commit box.
func (g *gitPanel) showStashes() {
	g.stashBox.RemoveAll()
	if len(g.stashes) == 0 {
		return
	}
	g.stashBox.Add(container.NewPadded(kit.Title(fmt.Sprintf("Stashes (%d)", len(g.stashes)))))
	for _, st := range g.stashes {
		label := widget.NewLabel(st.Message)
		label.Truncation = fyne.TextTruncateEllipsis
		label.SizeName = fynetheme.SizeNameCaptionText
		apply := widget.NewButton("Apply", func() { g.applyStash(st.Ref, false) })
		pop := widget.NewButton("Pop", func() { g.applyStash(st.Ref, true) })
		drop := widget.NewButton("Drop", func() { g.dropStash(st) })
		for _, b := range []*widget.Button{apply, pop, drop} {
			b.Importance = widget.LowImportance
		}
		g.stashBox.Add(container.NewBorder(nil, nil, nil, container.NewHBox(apply, pop, drop), label))
	}
}
