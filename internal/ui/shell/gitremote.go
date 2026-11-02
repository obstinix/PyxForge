package shell

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/git"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// The Git tab's work with other repositories and with Git's own state (PXF-DEF-030):
// initializing a repository, when asked; fetch, pull and push, one at a time and stoppable, with
// failures explained; publishing a branch and adding a remote; and an operation Git stopped in
// the middle of, a merge or rebase that met conflicts, shown above the files with a way out.

// initRepo makes the workspace folder a Git repository, after asking.
func (g *gitPanel) initRepo() {
	if !g.s.requireWorkspace() {
		return
	}
	root := g.s.root
	g.s.choose("Initialize a Git repository?",
		fmt.Sprintf("git init makes %s a Git repository: it adds a .git folder there. Nothing is staged or committed.", root),
		choice{label: "Initialize", primary: true, run: func() {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				repo, err := git.Init(ctx, root)
				g.s.dispatch(func() {
					if err != nil {
						g.s.Notify(notifications.Error, "Repository not initialized", err.Error())
						return
					}
					g.repo = repo
					g.s.Notify(notifications.Success, "Initialized a Git repository", repo.Root)
					g.refresh()
				})
			}()
		}})
}

// netOp runs a fetch, pull or push in the background: one at a time, with the bar saying what
// runs and offering Stop. run returns what to say when it worked: a title and a detail.
func (g *gitPanel) netOp(doing string, run func(ctx context.Context, r *git.Repo) (string, string, error), done func()) {
	repo := g.repo
	switch {
	case repo == nil:
		g.s.Notify(notifications.Info, "Not a Git repository", "Git > Initialize Repository… starts one in this folder.")
		return
	case g.netBusy != "":
		g.s.Notify(notifications.Info, g.netBusy, "Wait for it to finish, or stop it.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), git.NetworkTimeout)
	g.netBusy, g.netStop = doing+"…", cancel
	g.showRemoteState()
	go func() {
		defer cancel()
		title, detail, err := run(ctx, repo)
		g.s.dispatch(func() {
			g.netBusy, g.netStop = "", nil
			if err != nil {
				g.remoteFailed(err)
			} else {
				g.s.Notify(notifications.Success, title, detail)
				if done != nil {
					done()
				}
			}
			g.refresh()
		})
	}()
}

// stopNetwork stops the fetch, pull or push running.
func (g *gitPanel) stopNetwork() {
	if g.netStop != nil {
		g.netStop()
	}
}

// fetch downloads the remote's new commits; the bar then shows how far behind the branch is.
func (g *gitPanel) fetch() {
	g.netOp("Fetching", func(ctx context.Context, r *git.Repo) (string, string, error) {
		if err := r.Fetch(ctx); err != nil {
			return "", "", err
		}
		st, err := r.Status(ctx)
		switch {
		case err != nil || st.Upstream == "":
			return "Fetched", "", nil
		case st.Behind == 0:
			return "Fetched", st.Branch + " is up to date with " + st.Upstream + ".", nil
		}
		return "Fetched", fmt.Sprintf("%s is behind %s by %s.", st.Branch, st.Upstream, countOf(st.Behind, "commit")), nil
	}, nil)
}

// pull brings the branch up to its upstream when that only moves it forward.
func (g *gitPanel) pull() {
	if g.loaded && g.status.Upstream == "" {
		g.noUpstream()
		return
	}
	g.netOp("Pulling", func(ctx context.Context, r *git.Repo) (string, string, error) {
		before, _ := r.Status(ctx)
		if err := r.Pull(ctx); err != nil {
			return "", "", err
		}
		after, _ := r.Status(ctx)
		if before.Head == after.Head {
			return "Already up to date", after.Branch + " has every commit of " + after.Upstream + ".", nil
		}
		return "Pulled", fmt.Sprintf("%s from %s.", countOf(before.Behind, "commit"), after.Upstream), nil
	}, g.afterCheckout)
}

// push sends the branch to its upstream, or offers to publish a branch that has none.
func (g *gitPanel) push() {
	if g.loaded && g.status.Upstream == "" {
		g.publish()
		return
	}
	g.netOp("Pushing", func(ctx context.Context, r *git.Repo) (string, string, error) {
		before, _ := r.Status(ctx)
		if err := r.Push(ctx, ""); err != nil {
			return "", "", err
		}
		if before.Ahead == 0 {
			return "Nothing to push", before.Upstream + " has every commit of " + before.Branch + ".", nil
		}
		return "Pushed", fmt.Sprintf("%s to %s.", countOf(before.Ahead, "commit"), before.Upstream), nil
	}, nil)
}

// publish pushes a branch without an upstream to a remote and makes that its upstream. It asks
// which remote, and offers to add one when there is none.
func (g *gitPanel) publish() {
	branch := g.status.Branch
	if branch == "" {
		g.s.Notify(notifications.Info, "No branch to publish", "HEAD is detached: create a branch for these commits first (Git > Create Branch…).")
		return
	}
	if len(g.remotes) == 0 {
		g.s.choose("No remote", "This repository has no remote to push "+branch+" to. Add one, such as an empty repository made on your Git server, then publish.",
			choice{label: "Add Remote…", primary: true, run: func() { g.addRemote(g.publish) }})
		return
	}
	var choices []choice
	for i, r := range g.remotes {
		choices = append(choices, choice{label: "Publish to " + r.Name, primary: i == 0, run: func() {
			g.netOp("Publishing "+branch, func(ctx context.Context, repo *git.Repo) (string, string, error) {
				if err := repo.Push(ctx, r.Name); err != nil {
					return "", "", err
				}
				return "Published " + branch, "It is on " + r.Name + " now, and " + r.Name + "/" + branch + " is its upstream.", nil
			}, nil)
		}})
	}
	g.s.choose("Publish "+branch+"?", fmt.Sprintf("%s has no upstream yet. Publishing pushes it to the remote under the same name and makes that its upstream, so Pull and Push use it.", branch), choices...)
}

// noUpstream explains a Pull on a branch without an upstream.
func (g *gitPanel) noUpstream() {
	branch := g.status.Branch
	g.s.choose("No upstream for "+branch, branch+" tracks no remote branch, so there is nothing to pull. Publishing pushes it and sets one; for an existing remote branch, run git branch --set-upstream-to=origin/<name> in the Terminal.",
		choice{label: "Publish…", primary: true, run: g.publish})
}

// addRemote asks for a remote's name and URL and adds it; then runs next, if any.
func (g *gitPanel) addRemote(next func()) {
	if g.repo == nil {
		g.s.Notify(notifications.Info, "Not a Git repository", "Git > Initialize Repository… starts one in this folder.")
		return
	}
	name, url := g.s.newDialogEntry(), g.s.newDialogEntry()
	name.SetText("origin")
	if len(g.remotes) > 0 {
		name.SetText("")
	}
	url.SetPlaceHolder("https://github.com/you/project.git or git@github.com:you/project.git")
	d := dialog.NewForm("Add Remote", "Add", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Name", name), widget.NewFormItem("URL", url)}, func(ok bool) {
			if !ok {
				return
			}
			n, u := strings.TrimSpace(name.Text), strings.TrimSpace(url.Text)
			g.gitOp("add remote", func(ctx context.Context, r *git.Repo) error { return r.AddRemote(ctx, n, u) }, func(err error) {
				if err != nil {
					g.s.Notify(notifications.Error, "Remote not added", err.Error())
					return
				}
				g.remotes = append(g.remotes, git.Remote{Name: n, URL: u})
				g.s.Notify(notifications.Success, "Added remote "+n, u)
				if next != nil {
					next()
				}
			})
		}, g.s.win)
	d.Resize(fyne.NewSize(520, d.MinSize().Height))
	g.s.submitForm(d, name, url)
	g.s.win.Canvas().Focus(url)
	g.s.lastDialog = d
}

// remoteFailed says what went wrong in terms of what to do next; git's own message follows, and
// the Log has it whole.
func (g *gitPanel) remoteFailed(err error) {
	g.s.logf("Git: %v", err)
	var pe *git.ProjectProgramError
	if errors.As(err, &pe) {
		g.s.Notify(notifications.Error, "Refused: a program named by this repository", err.Error())
		return
	}
	var re *git.RemoteError
	if !errors.As(err, &re) {
		g.s.Notify(notifications.Error, "Git failed", err.Error())
		return
	}
	op := strings.ToUpper(re.Op[:1]) + re.Op[1:]
	title, hint := op+" failed", ""
	switch re.Problem {
	case git.ProblemCancelled:
		g.s.Notify(notifications.Info, op+" stopped", "")
		return
	case git.ProblemTimedOut:
		title, hint = op+" took too long", fmt.Sprintf("It was stopped after %v. A slow connection or a large repository may need the Terminal.", git.NetworkTimeout)
	case git.ProblemAuth:
		title, hint = op+" failed: not signed in", "The remote refused the credentials, or Git could not ask for them. Sign in through a credential helper (Git for Windows comes with Git Credential Manager) or load your SSH key into ssh-agent, then try again."
	case git.ProblemHostKey:
		title, hint = "The server's SSH key is not trusted", "Connect once from the Terminal (git fetch) to check the server's key and accept it."
	case git.ProblemNetwork:
		title, hint = "Cannot reach the remote", "Check the network connection and the remote's address (git remote -v)."
	case git.ProblemNotFound:
		title, hint = "Remote repository not found", "Check its URL (git remote -v). A private repository also answers this when the account cannot see it."
	case git.ProblemNoRemote:
		g.s.choose("No remote", "This repository has no remote yet.", choice{label: "Add Remote…", primary: true, run: func() { g.addRemote(nil) }})
		return
	case git.ProblemNoUpstream:
		g.noUpstream()
		return
	case git.ProblemRejected:
		title, hint = "The remote has commits this branch does not", "Pull them first, then push again. PyxForge never force-pushes."
	case git.ProblemDiverged:
		title, hint = "Cannot fast-forward", "This branch and its upstream both have new commits. Combine them in the Terminal (git merge or git rebase); any conflicts then show here."
	case git.ProblemLocalChanges:
		title, hint = "Uncommitted changes in the way", "The pull would overwrite changes not committed yet. Commit or stash them, then pull again."
	}
	body := withoutHints(err.Error())
	if hint != "" {
		body = hint + "\n" + body
	}
	g.s.Notify(notifications.Error, title, body)
}

// withoutHints drops the "hint:" lines from git's message: the notification says what to do,
// and the Log keeps the message whole.
func withoutHints(msg string) string {
	var keep []string
	for _, l := range strings.Split(msg, "\n") {
		bare := strings.TrimSpace(l)
		if _, rest, ok := strings.Cut(bare, ": "); ok && strings.HasPrefix(bare, "git ") {
			bare = rest // "git pull: hint: ..."
		}
		if bare != "" && !strings.HasPrefix(bare, "hint:") {
			keep = append(keep, strings.TrimRight(l, "\r"))
		}
	}
	return strings.Join(keep, "\n")
}

// showRemoteState labels Fetch, Pull and Push with what they would do, or with the operation
// running.
func (g *gitPanel) showRemoteState() {
	busy := g.netBusy != ""
	st := g.status
	pull, push := "Pull", "Push"
	if st.Behind > 0 {
		pull = fmt.Sprintf("Pull ↓%d", st.Behind)
	}
	if st.Ahead > 0 {
		push = fmt.Sprintf("Push ↑%d", st.Ahead)
	}
	if g.loaded && st.Upstream == "" && st.Branch != "" {
		push = "Publish"
	}
	g.pullBtn.SetText(pull)
	g.pushBtn.SetText(push)
	for _, b := range []*widget.Button{g.fetchBtn, g.pullBtn, g.pushBtn} {
		if busy || g.repo == nil {
			b.Disable()
		} else {
			b.Enable()
		}
	}
	if busy {
		g.branch.SetText(g.netBusy)
		g.stopNet.Show()
	} else {
		g.stopNet.Hide()
	}
	g.barRow.Refresh()
}

// operationText says what Git is waiting for in an operation it stopped in the middle of.
var operationText = map[string][2]string{ // what it is; how to finish
	"merge":       {"Merging", "Resolve each conflicted file, stage it, then commit."},
	"rebase":      {"Rebasing", "Resolve each conflicted file and stage it, then run git rebase --continue in the Terminal."},
	"am":          {"Applying patches", "Resolve each conflicted file and stage it, then run git am --continue in the Terminal."},
	"cherry-pick": {"Cherry-picking", "Resolve each conflicted file, stage it, then commit."},
	"revert":      {"Reverting", "Resolve each conflicted file, stage it, then commit."},
}

// showOperation shows the operation in progress above the files, with Abort; the commit box
// gets the message Git prepared for it.
func (g *gitPanel) showOperation(op, message string) {
	g.op = op
	g.banner.RemoveAll()
	text, ok := operationText[op]
	if !ok {
		g.banner.Refresh()
		return
	}
	label := widget.NewLabel(text[0] + ": " + text[1])
	label.Wrapping = fyne.TextWrapWord
	abort := widget.NewButton("Abort", g.abortOperation)
	g.banner.Add(container.NewPadded(container.NewBorder(nil, nil, nil, abort, label)))
	g.banner.Add(kit.NewRule(false))
	g.banner.Refresh()
	if message != "" && strings.TrimSpace(g.message.Text) == "" {
		g.message.SetText(message)
	}
}

// abortOperation gives up the operation in progress, after asking.
func (g *gitPanel) abortOperation() {
	op := g.op
	text, ok := operationText[op]
	if !ok {
		g.s.Notify(notifications.Info, "Nothing to abort", "Git has no merge, rebase, cherry-pick or revert in progress.")
		return
	}
	g.s.choose("Abort "+strings.ToLower(text[0])+"?",
		"Puts the branch, the staged changes and the files back as they were before it started. Conflicts resolved so far are lost.",
		choice{label: "Abort", primary: true, run: func() {
			g.gitOp("abort "+op, func(ctx context.Context, r *git.Repo) error { return r.Abort(ctx, op) }, func(err error) {
				if err != nil {
					g.s.Notify(notifications.Error, "Not aborted", err.Error())
					return
				}
				g.message.SetText("")
				g.s.Notify(notifications.Success, "Aborted: back to before "+strings.ToLower(text[0]), "")
				g.afterCheckout()
			})
		}})
}

// resolve stages a conflicted file, which marks it resolved, asking first while it still has
// conflict markers.
func (g *gitPanel) resolve(f git.File) {
	abs := filepath.Join(g.repo.Root, filepath.FromSlash(f.Path))
	if has, _ := git.HasConflictMarkers(abs); has {
		g.s.choose("Mark "+filepath.Base(abs)+" resolved?",
			f.Path+" still has conflict markers (<<<<<<< and >>>>>>>). Staging it records the file with them.",
			choice{label: "Stage Anyway", run: func() { g.apply("stage", false, []string{f.Path}) }})
		return
	}
	g.apply("stage", false, []string{f.Path})
}

// unresolved lists the conflicted files that still have conflict markers.
func (g *gitPanel) unresolved() []string {
	var out []string
	for _, f := range g.status.Files {
		if f.Conflicted() {
			if has, _ := git.HasConflictMarkers(filepath.Join(g.repo.Root, filepath.FromSlash(f.Path))); has {
				out = append(out, f.Path)
			}
		}
	}
	return out
}
