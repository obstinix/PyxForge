package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// trustShell is a shell on a project with one build profile, whose trust store is a file of the
// test's own. The profile runs this test binary, which exits at once.
func trustShell(t *testing.T) (*Shell, string, queue, *workspace.TrustStore) {
	t.Helper()
	a := test.NewTempApp(t)
	root := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	toml := fmt.Sprintf("[project]\nname = \"trust\"\n[profiles.build]\ntool = %q\nargs = [\"-test.run=^$\", \"with space\"]\n"+
		"[qemu]\nboot_image = \"build/boot.bin\"\n", filepath.ToSlash(exe))
	if err := os.WriteFile(filepath.Join(root, "pyxforge.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	store := &workspace.TrustStore{Path: filepath.Join(t.TempDir(), "trusted-folders.json")}
	q := make(queue, 4096)
	s := NewWithOptions(a, root, Options{Dispatch: q.post, Trust: store})
	noToastExpiry(s)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Log:\n%s", strings.Join(s.logLines, "\n"))
		}
	})
	return s, root, q, store
}

func logCount(s *Shell, prefix string) int {
	n := 0
	for _, l := range s.logLines {
		if strings.Contains(l, prefix) {
			n++
		}
	}
	return n
}

// TestBuildAsksForTrust: the first Build in a folder the user hasn't trusted shows the commands
// it would run and runs nothing until the folder is trusted; Cancel runs nothing and asks again
// next time; trusting runs the build and is remembered; revoking makes it ask again (#31).
func TestBuildAsksForTrust(t *testing.T) {
	s, root, q, store := trustShell(t)
	b := s.buildp

	b.start()
	p := s.trustPrompt
	if p == nil {
		t.Fatal("an untrusted folder's build ran without asking")
	}
	if p.action != "Build" || workspace.Canonical(p.dir) != workspace.Canonical(root) {
		t.Errorf("prompt for %q in %q", p.action, p.dir)
	}
	if listed := dialogText(s); !strings.Contains(listed, `-test.run=^$ "with space"`) {
		t.Errorf("the dialog doesn't list the command it would run:\n%s", listed)
	}
	p.cancel.OnTapped()
	if b.running || logCount(s, "Build started") != 0 || store.Trusted(root) {
		t.Fatal("Cancel ran the build, or trusted the folder")
	}
	b.start()
	if s.trustPrompt == nil {
		t.Fatal("after Cancel, the next Build did not ask again")
	}
	s.trustPrompt.trust.OnTapped()
	if !store.Trusted(root) {
		t.Fatal("trusting did not remember the folder")
	}
	q.pumpUntil(t, "the build", func() bool { return logCount(s, "Build finished") == 1 })

	b.start() // trusted now: no question
	if s.trustPrompt != nil {
		t.Fatal("a trusted folder's build asked again")
	}
	q.pumpUntil(t, "the second build", func() bool { return logCount(s, "Build finished") == 2 })

	s.revokeTrust()
	if store.Trusted(root) {
		t.Fatal("Revoke Trust left the folder trusted")
	}
	b.start()
	if s.trustPrompt == nil {
		t.Fatal("after revoking, Build did not ask")
	}
	s.trustPrompt.cancel.OnTapped()
}

// TestMachineAndRunCurrentFileAskForTrust: Run in QEMU lists the build and the QEMU command it
// would run, and Run Current File the program, before anything runs in an untrusted folder.
func TestMachineAndRunCurrentFileAskForTrust(t *testing.T) {
	s, root, q, _ := trustShell(t)
	s.mach.start(true)
	p := s.trustPrompt
	if p == nil || p.action != "Debug in QEMU" {
		t.Fatalf("Debug in QEMU in an untrusted folder: prompt %+v", p)
	}
	text := dialogText(s)
	for _, want := range []string{"-test.run=^$", "qemu-system-x86_64", "boot.bin", "gdb --interpreter=mi"} {
		if !strings.Contains(text, want) {
			t.Errorf("the dialog lacks %q:\n%s", want, text)
		}
	}
	p.cancel.OnTapped()
	if s.mach.starting || s.mach.inst != nil {
		t.Fatal("declining started the machine")
	}

	script := filepath.Join(root, "hello.py")
	if err := os.WriteFile(script, []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.runFile(script, false)
	if p := s.trustPrompt; p == nil {
		t.Skip("no Python here, so Run Current File stopped before asking") // it says so instead
	} else if p.action != "Run hello.py" || !strings.Contains(dialogText(s), "hello.py") {
		t.Errorf("Run Current File prompt %+v:\n%s", p, dialogText(s))
	} else {
		p.cancel.OnTapped()
	}
	for end := time.Now().Add(300 * time.Millisecond); time.Now().Before(end); {
		select {
		case f := <-q:
			f()
		case <-time.After(20 * time.Millisecond):
		}
	}
	if slices.ContainsFunc(s.logLines, func(l string) bool { return strings.Contains(l, "Terminal: Run hello.py") }) {
		t.Error("declining ran the file")
	}
}

func TestCommandLine(t *testing.T) {
	if got := commandLine([]string{"qemu", "-drive", `file=C:\my os\boot.bin`, "", `say "hi"`}); got != `qemu -drive "file=C:\my os\boot.bin" "" "say \"hi\""` {
		t.Errorf("commandLine = %s", got)
	}
}

// dialogText is the text of the trust dialog showing, from its labels.
func dialogText(s *Shell) string {
	if s.trustPrompt == nil {
		return ""
	}
	return strings.Join(texts(s.trustPrompt.content), "\n")
}

// TestCommitAsksToTrustTheRepositoryHooks: in an untrusted folder, Commit in a repository whose
// own configuration points core.hooksPath at a project folder asks first, listing the hooks;
// Cancel commits nothing and runs no hook; trusting commits, running them (#30).
func TestCommitAsksToTrustTheRepositoryHooks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig")) // never the user's own
	a := test.NewTempApp(t)
	root := t.TempDir()
	gitInit(t, root)
	for name, body := range map[string]string{"hooks/pre-commit": "#!/bin/sh\necho ran > hook-ran\n", "boot.asm": "org 0x7c00\n"} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"config", "core.hooksPath", "hooks"}, {"add", "boot.asm"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	store := &workspace.TrustStore{Path: filepath.Join(t.TempDir(), "trusted-folders.json")}
	q := make(queue, 1024)
	s := NewWithOptions(a, root, Options{Dispatch: q.post, Trust: store})
	noToastExpiry(s)
	g := s.gitp
	g.refresh()
	q.pumpUntil(t, "the repository", func() bool { return g.loaded && !g.running && g.repo != nil })

	commits := func() int {
		out, _ := exec.Command("git", "-C", root, "rev-list", "--all", "--count").Output()
		n := 0
		fmt.Sscan(string(out), &n)
		return n
	}
	g.message.SetText("Boot sector")
	g.doCommit()
	q.pumpUntil(t, "the trust question", func() bool { return s.trustPrompt != nil })
	if p := s.trustPrompt; p.action != "Commit" || !strings.Contains(dialogText(s), "hooks/pre-commit") || !strings.Contains(dialogText(s), "git commit") {
		t.Errorf("prompt %q:\n%s", p.action, dialogText(s))
	}
	s.trustPrompt.cancel.OnTapped()
	if _, err := os.Stat(filepath.Join(root, "hook-ran")); err == nil || commits() != 0 {
		t.Fatal("Cancel committed, or ran the project's hook")
	}
	g.doCommit()
	q.pumpUntil(t, "the trust question again", func() bool { return s.trustPrompt != nil })
	s.trustPrompt.trust.OnTapped()
	q.pumpUntil(t, "the commit", func() bool { return commits() == 1 && !g.running })
	if _, err := os.Stat(filepath.Join(root, "hook-ran")); err != nil {
		t.Error("the trusted repository's hook did not run")
	}
	if !store.Trusted(root) {
		t.Error("the folder was not remembered as trusted")
	}
}
