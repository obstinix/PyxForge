package neovim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindAndReplace: literal search (regex characters are text), case and whole-word
// options, counts, wrapping, Replace at the cursor, Replace All as one undo step.
func TestFindAndReplace(t *testing.T) {
	s := startReal(t, nil)
	file := filepath.Join(t.TempDir(), "find.txt")
	body := "mov ax, [bx+si]\nMOV bx, 1\nremove ax\nmov cx, [bx+si]\nкот Кот\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Open(file); err != nil {
		t.Fatal(err)
	}
	cursor := func() [2]int {
		var c [2]int
		_ = s.ExecLua(`local c = vim.api.nvim_win_get_cursor(0) return { c[1], c[2] }`, &c)
		return c
	}
	text := func() string {
		var lines []string
		_ = s.ExecLua(`return vim.api.nvim_buf_get_lines(0, 0, -1, false)`, &lines)
		return strings.Join(lines, "\n")
	}

	// "[bx+si]" is text, not a character class; both lines match.
	r, err := s.Find("[bx+si]", FindOptions{}, false, true)
	if err != nil || !r.Found || r.Total != 2 || r.Current != 1 || cursor() != [2]int{1, 8} {
		t.Fatalf("Find [bx+si] = %+v, %v; cursor %v", r, err, cursor())
	}
	if r, _ = s.Find("[bx+si]", FindOptions{}, false, false); r.Current != 2 || cursor()[0] != 4 {
		t.Errorf("Next = %+v at %v", r, cursor())
	}
	if r, _ = s.Find("[bx+si]", FindOptions{}, false, false); r.Current != 1 || cursor()[0] != 1 {
		t.Errorf("Next wraps = %+v at %v", r, cursor())
	}
	if r, _ = s.Find("[bx+si]", FindOptions{}, true, false); r.Current != 2 {
		t.Errorf("Previous wraps = %+v", r)
	}

	// Case and whole words.
	if r, _ = s.Find("mov", FindOptions{}, false, false); r.Total != 4 { // mov, MOV, (re)mov(e), mov
		t.Errorf("mov ignoring case: %+v", r)
	}
	if r, _ = s.Find("mov", FindOptions{MatchCase: true}, false, false); r.Total != 3 {
		t.Errorf("mov matching case: %+v", r)
	}
	if r, _ = s.Find("mov", FindOptions{WholeWord: true}, false, false); r.Total != 3 {
		t.Errorf("mov as a word: %+v", r)
	}
	if r, _ = s.Find("кот", FindOptions{}, false, false); r.Total != 2 {
		t.Errorf("Cyrillic ignoring case: %+v", r)
	}
	if r, _ = s.Find("nothing here", FindOptions{}, false, false); r.Found || r.Total != 0 {
		t.Errorf("no match: %+v", r)
	}

	// Replace changes the match at the cursor only, then moves on.
	if _, err := s.Find("ax", FindOptions{WholeWord: true}, false, true); err != nil {
		t.Fatal(err)
	}
	if r, err = s.Replace("ax", "eax", FindOptions{WholeWord: true}); err != nil || r.Total != 1 {
		t.Fatalf("Replace = %+v, %v", r, err)
	}
	if !strings.HasPrefix(text(), "mov eax, [bx+si]\nMOV bx, 1\nremove ax\n") {
		t.Errorf("after Replace:\n%s", text())
	}

	// Replace All, with characters :s would treat specially, in one undo step.
	if n, err := s.ReplaceAll("[bx+si]", `[b/x&~\1]`, FindOptions{}); err != nil || n != 2 {
		t.Fatalf("ReplaceAll = %d, %v", n, err)
	}
	if got := text(); strings.Count(got, `[b/x&~\1]`) != 2 || strings.Contains(got, "[bx+si]") {
		t.Errorf("after Replace All:\n%s", got)
	}
	if err := s.Command("undo"); err != nil {
		t.Fatal(err)
	}
	if got := text(); strings.Count(got, "[bx+si]") != 2 {
		t.Errorf("one undo did not take back Replace All:\n%s", got)
	}
}
