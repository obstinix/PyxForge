package shell

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/obstinix/PyxForge/internal/pty"
	"github.com/obstinix/PyxForge/internal/shellint"
	"github.com/obstinix/PyxForge/internal/term"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
	"github.com/obstinix/PyxForge/internal/vt"
)

// TestTerminalCommandBlocks: a shell the Terminal starts loads the integration, so a command
// typed in it becomes a block the view draws; with Command blocks off, it is a plain shell.
func TestTerminalCommandBlocks(t *testing.T) {
	s, _, q := newEditorShell(t)
	tm := s.term
	if !shellint.Supported(s.terminalShell()) {
		t.Skipf("the default shell here (%s) cannot be integrated", s.terminalShell().Name)
	}
	s.showDockItem(tm.tab)
	ts := tm.current
	if !ts.blocks || !tm.view.BlocksOn {
		t.Fatalf("the session has no command blocks: %+v", ts)
	}
	q.pumpUntil(t, "the first prompt", func() bool { return ts.Screen().MarkSeq() > 0 })
	for _, r := range "echo blocks-here" {
		tm.view.TypedRune(r)
	}
	tm.view.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	q.pumpUntil(t, "echo's block", func() bool {
		return slices.ContainsFunc(tm.view.Blocks(), func(b term.Block) bool { return b.Command == "echo blocks-here" && !b.Running })
	})

	s.app.Preferences().SetBool(prefTerminalBlocks, false)
	tm.newSession()
	if tm.current.blocks || tm.view.BlocksOn {
		t.Error("Command blocks off still integrated the new shell")
	}
}

// TestCmdHasNoBlocks: with cmd.exe (or another shell that cannot be integrated) the pane says
// once that command blocks need PowerShell, bash, zsh or fish, with the way to the setting.
func TestCmdHasNoBlocks(t *testing.T) {
	s, _ := newTestShell(t)
	tm := s.term
	ts := &termSession{name: "Shell 1", kind: kindShell, screen: newScreen(), plainShell: true}
	tm.sessions = append(tm.sessions, ts)
	tm.showSession(ts)
	box := tm.pane().box
	if len(box.Objects) != 2 || !slices.Contains(texts(box), "Command blocks need PowerShell, bash, zsh or fish.") {
		t.Fatalf("no note: %v", texts(box))
	}
	test.Tap(findButton(box, "Terminal settings"))
	if s.settingsUI == nil || s.settingsUI.current != "terminal" {
		t.Error("the note's button did not open Settings > Terminal")
	}
	s.closeSettings()
	s.noBlocksNoted = true
	tm.sync()
	if len(tm.pane().box.Objects) != 1 {
		t.Error("the dismissed note came back")
	}
}

// TestSessionRail: the rail lists every session with its kind and state, marks the ones on
// view, collapses to icons in a narrow panel, and its rows select sessions.
func TestSessionRail(t *testing.T) {
	s, _ := newTestShell(t)
	tm := s.term
	a := &termSession{name: "Shell 1", kind: kindShell, screen: newScreen()}
	b := &termSession{name: "Run hello.c", kind: kindRun, screen: newScreen(), done: true, status: 2}
	tm.sessions = append(tm.sessions, a, b)
	tm.showSession(a)
	if len(tm.rail.rows) != 2 || tm.rail.rows[1].ts.kind.icon() == tm.rail.rows[0].ts.kind.icon() {
		t.Fatalf("rows %d", len(tm.rail.rows))
	}
	if !strings.HasPrefix(sessionState(b), "exit 2") || sessionState(a) != "running" {
		t.Errorf("states %q %q", sessionState(a), sessionState(b))
	}
	w := test.NewWindow(tm.frame)
	defer w.Close()
	w.Resize(fyne.NewSize(1000, 300))
	if tm.rail.narrow || tm.rail.object().Size().Width != railWide {
		t.Errorf("wide panel: rail %v wide, narrow %v", tm.rail.object().Size().Width, tm.rail.narrow)
	}
	w.Resize(fyne.NewSize(600, 300))
	if !tm.rail.narrow || tm.rail.object().Size().Width != railNarrow {
		t.Errorf("narrow panel: rail %v wide, narrow %v", tm.rail.object().Size().Width, tm.rail.narrow)
	}
	tm.rail.rows[1].Tapped(&fyne.PointEvent{})
	if tm.current != b {
		t.Error("tapping a row did not show its session")
	}
}

// TestTerminalSplit: Split Right shows a second session beside the first, at most two; closing
// the second pane's session closes the pane, and the first keeps its own.
func TestTerminalSplit(t *testing.T) {
	s, _ := newTestShell(t)
	tm := s.term
	a := &termSession{name: "A", screen: newScreen()}
	b := &termSession{name: "B", screen: newScreen()}
	tm.sessions = append(tm.sessions, a, b)
	tm.showSession(a)
	tm.splitRight(nil)
	if len(tm.panes) != 2 || tm.panes[0].session != a || tm.panes[1].session != b || tm.current != b {
		t.Fatalf("split: %d panes", len(tm.panes))
	}
	if _, ok := tm.body.Objects[0].(*fyne.Container); !ok || len(tm.body.Objects[0].(*fyne.Container).Objects) != 3 {
		t.Error("the two panes are not side by side with a sash")
	}
	tm.splitRight(nil) // a third is refused
	if len(tm.panes) != 2 {
		t.Error("a third pane opened")
	}
	tm.showSession(a) // the other pane's session: the panes swap
	if len(tm.panes) != 2 || tm.panes[1].session != a || tm.panes[0].session != b {
		t.Errorf("showing A in the second pane: %v, %v", tm.panes[0].session.name, tm.panes[1].session.name)
	}
	tm.closeSession(b)
	if len(tm.panes) != 1 || tm.current != a {
		t.Errorf("after closing B: %d panes, current %v", len(tm.panes), tm.current.name)
	}
}

// TestSerialSession: the machine's serial port is a session of its own, read only until Send
// input is on; then what is typed reaches the port.
func TestSerialSession(t *testing.T) {
	s, _ := newTestShell(t)
	tm := s.term
	var sent []string
	ts := tm.console(kindSerial, "QEMU serial", func(b []byte) { sent = append(sent, string(b)) })
	tm.consoleLine(ts, "Hello from the boot sector")
	if !strings.Contains(ts.Screen().String(), "Hello from the boot sector") {
		t.Fatalf("screen %q", ts.Screen().String())
	}
	tm.showSession(ts)
	tm.view.TypedRune('x')
	if len(sent) != 0 {
		t.Error("a read-only session sent input")
	}
	var sw *kit.Switch
	for _, o := range test.LaidOutObjects(tm.pane().box) {
		if x, ok := o.(*kit.Switch); ok && x.Label == "Send input" {
			sw = x
		}
	}
	if sw == nil {
		t.Fatal("no Send input switch")
	}
	sw.TypedRune(' ')
	tm.view.TypedRune('x')
	if !ts.sendInput || len(sent) != 1 || sent[0] != "x" {
		t.Errorf("Send input on: sent %q", sent)
	}
	if again := tm.console(kindSerial, "QEMU serial", nil); again != ts {
		t.Error("a new QEMU made a second serial session")
	}
	tm.consoleEnded(ts, 0)
	if !ts.done || statusDot(themeTokens(), ts) != themeTokens().Text.Tertiary {
		t.Error("an ended console is not shown as detached")
	}
}

// TestTerminalLinkResolution: paths in output resolve against the command's folder, and only
// to files that exist inside the project.
func TestTerminalLinkResolution(t *testing.T) {
	s, root := newTestShell(t)
	sub := filepath.Join(root, "src")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "main.c"), []byte("int main;\n"), 0o644)
	outside := filepath.Join(t.TempDir(), "secret.c")
	os.WriteFile(outside, []byte("x"), 0o644)
	cases := []struct {
		cwd, path string
		ok        bool
	}{
		{root, "src/main.c", true},
		{sub, "main.c", true},
		{"", "src/main.c", true}, // no folder reported: the project root
		{sub, "../src/main.c", true},
		{sub, "../../" + filepath.Base(filepath.Dir(outside)) + "/secret.c", false},
		{root, outside, false},
		{root, "src/missing.c", false},
		{root, "src", false},
	}
	for _, c := range cases {
		got, ok := s.resolveTerminalLink(c.cwd, c.path)
		if ok != c.ok {
			t.Errorf("%q in %q: %q %v, want %v", c.path, c.cwd, got, ok, c.ok)
		}
	}
}

// TestSendBlockToProblems: a block's compiler output becomes problems with the source
// "Terminal: <command>", paths read against the block's folder; sending again replaces them.
func TestSendBlockToProblems(t *testing.T) {
	s, root := newTestShell(t)
	b := term.Block{Command: "gcc -c main.c", Cwd: root, Exit: 1}
	out := "main.c:3:5: error: expected ';' before 'return'\nmain.c:1:1: warning: unused include\n"
	s.sendBlockToProblems(b, out)
	var mine []problem
	for _, p := range s.problems {
		if p.d.Source == "Terminal: gcc -c main.c" {
			mine = append(mine, p)
		}
	}
	if len(mine) != 2 || mine[0].path != filepath.Join(root, "main.c") || mine[0].d.Line != 2 || mine[0].d.Col != 4 || mine[0].d.Severity != 1 {
		t.Fatalf("problems %+v", mine)
	}
	s.sendBlockToProblems(b, "main.c:9:1: error: again\n")
	n := 0
	for _, p := range s.problems {
		if p.d.Source == "Terminal: gcc -c main.c" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("sending again left %d problems from it", n)
	}
	s.sendBlockToProblems(term.Block{Command: "ls"}, "README.md\n")
	if _, ok := s.termProblems["ls"]; !ok || len(s.termProblems["ls"]) != 0 {
		t.Error("output with no messages was not recorded as none")
	}
}

// TestTerminalPromptChords: the chords for the previous and next command and for finding in
// the output are Ctrl+Shift ones, which K2 leaves PyxForge, and the terminal hands them over.
func TestTerminalPromptChords(t *testing.T) {
	s, _ := newTestShell(t)
	for id, keys := range map[string]string{"terminal.previousPrompt": "Ctrl+Shift+Up", "terminal.nextPrompt": "Ctrl+Shift+Down", "terminal.find": "Ctrl+Shift+F"} {
		c, ok := s.Commands().Get(id)
		if !ok || c.Keys != keys {
			t.Errorf("%s: %q", id, c.Keys)
		}
	}
	_ = pty.Shells
}

func newScreen() *vt.Screen { return vt.New(80, 10) }

func themeTokens() theme.Tokens { return theme.Current() }
