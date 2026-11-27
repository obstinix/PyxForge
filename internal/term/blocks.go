package term

import (
	"strings"
	"time"

	"github.com/obstinix/PyxForge/internal/vt"
)

// Block is one command a shell with PyxForge's integration ran (D24): from its prompt to the
// end of its output, as the shell marked them (OSC 133).
type Block struct {
	Command    string    // what was typed, as the screen shows it
	Cwd        string    // the folder it ran in (OSC 7), "" when the shell never said
	Start, End time.Time // when its output started and when it ended (zero while running)
	Exit       int       // its exit status; -1 while it runs or when the shell gave none
	Running    bool
	PromptLine int    // the absolute line its prompt starts on
	Output     [2]int // its output's absolute lines, [first, end)
	Seq        uint64 // the sequence number of the block's prompt mark: stable while it is kept
}

// Duration is how long the command ran, or has run so far.
func (b Block) Duration() time.Duration {
	if b.Start.IsZero() {
		return 0
	}
	if b.Running {
		return time.Since(b.Start)
	}
	return b.End.Sub(b.Start)
}

// Failed reports a command that ended with a non-zero status.
func (b Block) Failed() bool { return !b.Running && b.Exit > 0 }

// Lines is the block's absolute lines, [prompt, end of output).
func (b Block) Lines() [2]int { return [2]int{b.PromptLine, b.Output[1]} }

// Blocks are the commands on a screen's kept lines, oldest first. A prompt nothing was run at
// (an empty Enter, or the prompt waiting now) is not a block.
func Blocks(sc *vt.Screen) []Block {
	if sc == nil {
		return nil
	}
	marks := sc.Marks()
	end := sc.EndLine()
	var out []Block
	var cur *Block
	var cmdAt vt.Pos
	haveCmd := false
	finish := func() {
		if cur != nil && !cur.Start.IsZero() {
			out = append(out, *cur)
		}
		cur, haveCmd = nil, false
	}
	for _, m := range marks {
		switch m.Kind {
		case vt.MarkPrompt:
			if cur != nil && cur.Running && !cur.Start.IsZero() {
				// A new prompt without an end mark: the command ended unreported.
				cur.Running, cur.Output[1] = false, max(m.At.Line, cur.Output[0])
				cur.End = m.Time
			}
			finish()
			cur = &Block{PromptLine: m.At.Line, Cwd: m.Cwd, Exit: -1, Running: true, Seq: m.Seq}
		case vt.MarkCommand:
			if cur != nil {
				cmdAt, haveCmd = m.At, true
			}
		case vt.MarkOutput:
			if cur == nil {
				continue
			}
			if haveCmd {
				cur.Command = strings.TrimSpace(sc.Text(cmdAt, m.At))
			}
			if cur.Command == "" {
				continue // Enter at an empty prompt
			}
			cur.Start, cur.Cwd = m.Time, m.Cwd
			cur.Output = [2]int{m.At.Line, end}
			if m.At.Col > 0 { // the output starts on the command's own line
				cur.Output[0] = m.At.Line
			}
		case vt.MarkEnd:
			if cur == nil || cur.Start.IsZero() {
				cur = nil
				continue
			}
			cur.Running, cur.Exit, cur.End = false, m.Exit, m.Time
			last := m.At.Line
			if m.At.Col > 0 { // output without a final newline ends on the end mark's line
				last++
			}
			cur.Output[1] = max(last, cur.Output[0])
			finish()
		}
	}
	if cur != nil && cur.Running && !cur.Start.IsZero() {
		out = append(out, *cur)
	}
	return out
}

// OutputText is a block's output as text.
func OutputText(sc *vt.Screen, b Block) string {
	if sc == nil || b.Output[1] <= b.Output[0] {
		return ""
	}
	return strings.TrimRight(sc.Text(vt.Pos{Line: b.Output[0]}, vt.Pos{Line: b.Output[1]}), "\n")
}
