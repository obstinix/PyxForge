package term

import (
	"fmt"
	"testing"

	"github.com/obstinix/PyxForge/internal/vt"
)

// run writes one integrated command; exit < 0 leaves it running.
func run(s *vt.Screen, cwd, cmd, out string, exit int) {
	if cwd != "" {
		fmt.Fprintf(s, "\x1b]7;file://h%s\x07", cwd)
	}
	fmt.Fprintf(s, "\x1b]133;A\x07$ \x1b]133;B\x07%s\r\n\x1b]133;C\x07%s", cmd, out)
	if exit >= 0 {
		fmt.Fprintf(s, "\x1b]133;D;%d\x07", exit)
	}
}

// TestBlocks: commands become blocks with their command, folder, exit status, prompt line and
// output lines; an empty Enter and the waiting prompt are not blocks; a command still running
// is the last block.
func TestBlocks(t *testing.T) {
	s := vt.New(60, 20)
	run(s, "/src", "make", "cc main.c\r\nok\r\n", 0)
	run(s, "", "", "", 0) // Enter at an empty prompt
	run(s, "/src/sub", "false", "", 1)
	run(s, "", "sleep 9", "zz", -1)
	bs := Blocks(s)
	if len(bs) != 3 {
		t.Fatalf("%d blocks: %+v", len(bs), bs)
	}
	b := bs[0]
	if b.Command != "make" || b.Exit != 0 || b.Running || b.Failed() || b.PromptLine != 0 || b.Output != [2]int{1, 3} {
		t.Errorf("make: %+v", b)
	}
	if got := OutputText(s, b); got != "cc main.c\nok" {
		t.Errorf("make's output %q", got)
	}
	if bs[1].Command != "false" || bs[1].Exit != 1 || !bs[1].Failed() || bs[1].Cwd == "" || bs[1].Output[1] != bs[1].Output[0] {
		t.Errorf("false: %+v", bs[1])
	}
	if !bs[2].Running || bs[2].Exit != -1 || bs[2].Output[1] != s.EndLine() || OutputText(s, bs[2]) != "zz" {
		t.Errorf("running: %+v, output %q", bs[2], OutputText(s, bs[2]))
	}
	if bs[0].Duration() < 0 || bs[0].End.Before(bs[0].Start) {
		t.Errorf("duration %v", bs[0].Duration())
	}
}

// TestBlocksWithoutEnd: a prompt after a command with no end mark ends the command there.
func TestBlocksWithoutEnd(t *testing.T) {
	s := vt.New(60, 10)
	run(s, "", "ssh box", "bye\r\n", -1)
	run(s, "", "ls", "a\r\n", 0)
	bs := Blocks(s)
	if len(bs) != 2 || bs[0].Running || bs[0].Exit != -1 || bs[0].Output != [2]int{1, 2} {
		t.Fatalf("blocks %+v", bs)
	}
}

// TestBlocksAfterTrim: blocks whose prompt was trimmed from the history are gone; later ones
// keep their lines.
func TestBlocksAfterTrim(t *testing.T) {
	s := vt.New(40, 3)
	s.SetMaxHistory(4)
	run(s, "", "old", "1\r\n2\r\n3\r\n4\r\n5\r\n6\r\n", 0)
	run(s, "", "new", "x\r\n", 0)
	bs := Blocks(s)
	if len(bs) != 1 || bs[0].Command != "new" || bs[0].PromptLine < s.FirstLine() {
		t.Fatalf("blocks %+v (first line %d)", bs, s.FirstLine())
	}
}
