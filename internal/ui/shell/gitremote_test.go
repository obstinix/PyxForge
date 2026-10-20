package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// logged reports whether a log line (notifications are logged too) contains text.
func logged(s *Shell, text string) bool {
	return slices.ContainsFunc(s.logLines, func(l string) bool { return strings.Contains(l, text) })
}

// tapInDialog taps the button labelled text in the dialog in front.
func tapInDialog(t *testing.T, s *Shell, text string) {
	t.Helper()
	top := s.win.Canvas().Overlays().Top()
	b := findButton(top, text)
	if b == nil {
		t.Fatalf("no %q button in the dialog in front", text)
	}
	test.Tap(b)
}

// entries lists the text fields in o, in order.
func entries(o fyne.CanvasObject) []*dialogEntry {
	var out []*dialogEntry
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch w := o.(type) {
		case *dialogEntry:
			out = append(out, w)
		case *fyne.Container:
			for _, c := range w.Objects {
				walk(c)
			}
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(w).Objects() {
				walk(c)
			}
		}
	}
	walk(o)
	return out
}

// TestGitInitializeAsksFirst: a folder that is not a repository offers Initialize Repository,
// which asks before running git init; then the tab shows the new repository.
func TestGitInitializeAsksFirst(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	a := test.NewTempApp(t)
	root := t.TempDir()
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	g := s.gitp
	g.refresh()
	q.pumpUntil(t, "the first status", func() bool { return !g.running })
	if !s.available("git.init") || s.available("git.fetch") {
		t.Errorf("outside a repository: init %v, fetch %v", s.available("git.init"), s.available("git.fetch"))
	}
	b := findButton(g.body, "Initialize Repository")
	if b == nil {
		t.Fatal("no Initialize Repository button outside a repository")
	}
	test.Tap(b)
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		t.Fatal("git init ran before the user agreed")
	}
	tapInDialog(t, s, "Initialize")
	q.pumpUntil(t, "the new repository", func() bool { return g.loaded && !g.running })
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf("no .git after Initialize: %v", err)
	}
	if s.available("git.init") || !logged(s, "Initialized a Git repository") {
		t.Error("after Initialize: still offered, or not reported")
	}
}

// TestGitRemoteWorkflow drives Add Remote, Publish, Fetch, Pull and Push from the Git tab
// against a remote on this machine, and a push the remote rejects.
func TestGitRemoteWorkflow(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	gitInit(t, root)
	boot := filepath.Join(root, "boot.asm")
	if err := os.WriteFile(boot, []byte("org 0x7c00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "--quiet", "-m", "first")
	bare := filepath.Join(t.TempDir(), "remote.git")
	gitRun(t, filepath.Dir(bare), "init", "--quiet", "--bare", bare)
	gitRun(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")

	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	g := s.gitp
	idle := func() bool { return !g.running && g.netBusy == "" }
	g.refresh()
	q.pumpUntil(t, "the status", func() bool { return g.loaded && idle() })
	if g.pushBtn.Text != "Publish" {
		t.Errorf("push button %q on a branch without an upstream", g.pushBtn.Text)
	}

	// No remote yet: publishing offers to add one; the form adds it and then publishes.
	g.push()
	tapInDialog(t, s, "Add Remote…")
	fields := entries(s.win.Canvas().Overlays().Top())
	if len(fields) != 2 || fields[0].Text != "origin" {
		t.Fatalf("Add Remote fields: %d, name %q", len(fields), fields[0].Text)
	}
	fields[1].SetText(bare)
	tapInDialog(t, s, "Add")
	q.pumpUntil(t, "the remote", func() bool { return idle() && len(g.remotes) == 1 })
	tapInDialog(t, s, "Publish to origin")
	q.pumpUntil(t, "the published branch", func() bool { return idle() && g.status.Upstream == "origin/main" })
	if !logged(s, "Published main It is on origin now") || gitRun(t, bare, "log", "--format=%s") != "first" {
		t.Errorf("publish: log %v, remote has %q", s.logLines, gitRun(t, bare, "log", "--format=%s"))
	}

	// Another clone pushes a commit: Fetch shows it, Pull brings it.
	other := filepath.Join(t.TempDir(), "other")
	gitRun(t, filepath.Dir(other), "clone", "--quiet", bare, other)
	gitRun(t, other, "config", "user.name", "Other")
	gitRun(t, other, "config", "user.email", "other@example.invalid")
	if err := os.WriteFile(filepath.Join(other, "boot.asm"), []byte("org 0x7c00\nbits 16\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, other, "commit", "--quiet", "-am", "16-bit")
	gitRun(t, other, "push", "--quiet")
	test.Tap(g.fetchBtn)
	if !g.fetchBtn.Disabled() || !g.stopNet.Visible() {
		t.Error("Fetch running: the buttons are not disabled, or there is no Stop")
	}
	q.pumpUntil(t, "the fetch", func() bool { return idle() && g.status.Behind == 1 })
	if g.pullBtn.Text != "Pull ↓1" || !logged(s, "Fetched main is behind origin/main by 1 commit.") {
		t.Errorf("after Fetch: pull button %q", g.pullBtn.Text)
	}
	test.Tap(g.pullBtn)
	q.pumpUntil(t, "the pull", func() bool { return idle() && g.status.Behind == 0 && g.pullBtn.Text == "Pull" })
	if data, _ := os.ReadFile(boot); string(data) != "org 0x7c00\nbits 16\n" || !logged(s, "Pulled 1 commit from origin/main.") {
		t.Errorf("after Pull: %q", data)
	}

	// A commit here: Push sends it.
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("# notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "--quiet", "-m", "notes")
	g.refresh()
	q.pumpUntil(t, "one ahead", func() bool { return idle() && g.pushBtn.Text == "Push ↑1" })
	test.Tap(g.pushBtn)
	q.pumpUntil(t, "the push", func() bool { return idle() && g.status.Ahead == 0 })
	if !logged(s, "Pushed 1 commit to origin/main.") || !strings.HasPrefix(gitRun(t, bare, "log", "--format=%s"), "notes") {
		t.Error("Push did not reach the remote")
	}

	// Both sides commit: the push is rejected and explained, and nothing is forced.
	gitRun(t, other, "pull", "--quiet")
	if err := os.WriteFile(filepath.Join(other, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, other, "add", ".")
	gitRun(t, other, "commit", "--quiet", "-m", "from other")
	gitRun(t, other, "push", "--quiet")
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "--quiet", "-m", "from here")
	g.push()
	q.pumpUntil(t, "the rejection", func() bool { return idle() && logged(s, "The remote has commits this branch does not") })
	if gitRun(t, bare, "log", "-1", "--format=%s") != "from other" {
		t.Error("the rejected push changed the remote")
	}
	test.Tap(g.pullBtn)
	q.pumpUntil(t, "the refused pull", func() bool { return idle() && logged(s, "Cannot fast-forward") })
}

// TestGitConflictInProgress: a merge that stopped on a conflict shows above the files with
// Abort, fills the commit box with Git's message and lists the file under Conflicts; staging it
// with its markers asks first; Abort, after asking, puts everything back.
func TestGitConflictInProgress(t *testing.T) {
	a := test.NewTempApp(t)
	root := t.TempDir()
	gitInit(t, root)
	boot := filepath.Join(root, "boot.asm")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(boot, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("org 0x7c00\n")
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "--quiet", "-m", "first")
	gitRun(t, root, "switch", "--quiet", "-c", "other")
	write("org 0x8000\n")
	gitRun(t, root, "commit", "--quiet", "-am", "other origin")
	gitRun(t, root, "switch", "--quiet", "main")
	write("org 0x7e00\n")
	gitRun(t, root, "commit", "--quiet", "-am", "main origin")
	if out, err := exec.Command("git", "-C", root, "merge", "other").CombinedOutput(); err == nil {
		t.Fatalf("the merge did not conflict:\n%s", out)
	}

	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	g := s.gitp
	g.refresh()
	q.pumpUntil(t, "the merge", func() bool { return g.loaded && !g.running && g.op == "merge" })
	if findButton(g.banner, "Abort") == nil || !strings.HasPrefix(g.message.Text, "Merge branch 'other'") {
		t.Errorf("banner %d objects, message %q", len(g.banner.Objects), g.message.Text)
	}
	if got := headings(g); !slices.Equal(got, []string{"Conflicts (1)"}) || !s.available("git.abort") {
		t.Errorf("groups %v, abort available %v", got, s.available("git.abort"))
	}
	if !g.commit.Disabled() || g.commit.Text != "Resolve 1 conflict first" {
		t.Errorf("commit button %q, disabled %v, with a conflict", g.commit.Text, g.commit.Disabled())
	}
	g.list.Select(1) // the conflicted file opens as it is, markers and all, not as a diff
	if !slices.ContainsFunc(s.editors.Items, func(t *container.TabItem) bool { return t.Text == "boot.asm" }) {
		t.Error("selecting the conflicted file did not open it")
	}

	s.lastDialog = nil
	g.resolve(g.rows[1].file)
	if s.lastDialog == nil {
		t.Fatal("staging a file with conflict markers did not ask")
	}
	s.lastDialog.Hide()
	s.lastDialog = nil
	g.apply("stage", true, nil)
	if s.lastDialog == nil {
		t.Fatal("Stage All with conflict markers did not ask")
	}
	s.lastDialog.Hide()

	g.abortOperation()
	tapInDialog(t, s, "Abort")
	q.pumpUntil(t, "the abort", func() bool { return !g.running && g.op == "" && len(g.banner.Objects) == 0 })
	if data, _ := os.ReadFile(boot); string(data) != "org 0x7e00\n" || g.message.Text != "" {
		t.Errorf("after Abort: file %q, message %q", data, g.message.Text)
	}

	// Merge again and resolve it: staging asks nothing, and the commit uses Git's message.
	_ = exec.Command("git", "-C", root, "merge", "other").Run()
	g.refresh()
	q.pumpUntil(t, "the second merge", func() bool { return !g.running && g.op == "merge" })
	write("org 0x7e00 ; and 0x8000 elsewhere\n")
	s.lastDialog = nil
	g.resolve(g.rows[1].file)
	if s.lastDialog != nil {
		t.Error("staging a resolved file asked")
	}
	q.pumpUntil(t, "the resolved file staged", func() bool { return !g.running && slices.Equal(headings(g), []string{"Staged (1)"}) })
	g.doCommit()
	q.pumpUntil(t, "the merge commit", func() bool { return !g.running && g.op == "" && len(g.history.Objects) > 0 })
	if top := g.history.Objects[0].(*widget.Label).Text; !strings.Contains(top, "Merge branch 'other'") {
		t.Errorf("history starts with %q", top)
	}
}

// TestGitPanelWorkflowWithLFS: in a Git LFS repository, a change is listed, staged and
// committed from the Git tab, stored as an LFS pointer, and shows in the history.
func TestGitPanelWorkflowWithLFS(t *testing.T) {
	if err := exec.Command("git", "lfs", "version").Run(); err != nil {
		t.Skip("git-lfs is not installed")
	}
	a := test.NewTempApp(t)
	root := t.TempDir()
	gitInit(t, root)
	gitRun(t, root, "lfs", "install", "--local")
	gitRun(t, root, "lfs", "track", "*.img")
	disk := filepath.Join(root, "disk.img")
	if err := os.WriteFile(disk, []byte("\x00\x01 first image\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "--quiet", "-m", "first image")

	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post})
	g := s.gitp
	if err := os.WriteFile(disk, []byte("\x00\x02 second image\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g.refresh()
	q.pumpUntil(t, "the change", func() bool { return g.loaded && !g.running && slices.Equal(headings(g), []string{"Changes (1)"}) })
	g.apply("stage", false, []string{"disk.img"})
	q.pumpUntil(t, "the staged change", func() bool { return !g.running && slices.Equal(headings(g), []string{"Staged (1)"}) })
	g.message.SetText("Second image")
	g.doCommit()
	q.pumpUntil(t, "the commit", func() bool { return !g.running && slices.Equal(headings(g), []string{"No changes"}) })
	if top := g.history.Objects[0].(*widget.Label).Text; !strings.HasSuffix(top, "Second image") {
		t.Errorf("history starts with %q", top)
	}
	if blob := gitRun(t, root, "cat-file", "blob", "HEAD:disk.img"); !strings.HasPrefix(blob, "version https://git-lfs.github.com/spec/v1") {
		t.Errorf("disk.img was committed as %q, not as an LFS pointer", blob)
	}
}

// TestWithoutHints: git's advice lines leave the notification; what it reports stays.
func TestWithoutHints(t *testing.T) {
	in := "git pull: hint: Diverging branches can't be fast-forwarded, you need to either:\r\nhint:\r\nhint:   git merge --no-ff\r\nfatal: Not possible to fast-forward, aborting.\r\n"
	if got := withoutHints(in); got != "fatal: Not possible to fast-forward, aborting." {
		t.Errorf("withoutHints = %q", got)
	}
	if got := withoutHints("git push: error: failed to push some refs"); got != "git push: error: failed to push some refs" {
		t.Errorf("withoutHints dropped a report: %q", got)
	}
}

// TestGitBarFitsANarrowPanel: with a long branch and upstream, the Git tab's bar still fits a
// panel at the editor's narrowest: the upstream text is cut short and the bar scrolls, so no
// button is cut off (PXF-DEF-006).
func TestGitBarFitsANarrowPanel(t *testing.T) {
	a := test.NewTempApp(t)
	s := NewWithOptions(a, t.TempDir(), Options{})
	g := s.gitp
	g.branchBtn.SetText("feature/vga-driver ▾")
	g.branch.SetText("origin/feature/vga-driver ↑12 ↓3")
	g.pullBtn.SetText("Pull ↓3")
	g.pushBtn.SetText("Push ↑12")
	g.stopNet.Show()
	if w := g.bar.MinSize().Width; w > minEditorW {
		t.Errorf("the Git bar needs %.0f, more than the panel's %d", w, minEditorW)
	}
	win := test.NewWindow(g.bar)
	defer win.Close()
	win.Resize(fyne.NewSize(1200, 60))
	if g.branch.Size().Width < g.branch.MinSize().Width || g.branch.Size().Width < 100 {
		t.Errorf("in a wide panel the upstream text gets %.0f", g.branch.Size().Width)
	}
}
