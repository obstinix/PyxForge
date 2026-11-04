package git

import (
	"strings"
	"testing"
)

// FuzzParseStatus feeds parseStatus arbitrary `git status --porcelain=v2 -z` output; file
// names are anything a repository holds (#43; #18 was a panic found this way).
func FuzzParseStatus(f *testing.F) {
	f.Add(strings.Join([]string{
		"# branch.oid 1a2b3c", "# branch.head main", "# branch.upstream origin/main", "# branch.ab +2 -1",
		"1 M. N... 100644 100644 100644 aaa bbb boot/boot.asm",
		"1 .D N... 100644 100644 000000 aaa aaa old file.c",
		"2 R. N... 100644 100644 100644 aaa aaa R100 kernel/main.c", "kernel/kmain.c",
		"u UU N... 100644 100644 100644 100644 a b c link.ld",
		"? notes.md", "! build/",
		"",
	}, "\x00"))
	f.Add("# branch.oid (initial)\x00# branch.head (detached)\x00")
	f.Add("2 R. N... 100644 100644 100644 aaa aaa R100 last")
	f.Add("1 0       0\x002 M       x y\x00# branch.ab +x -y")
	f.Add("2 R. N... 100644 100644 100644 aaa aaa R100 new.c\x00old.c\x00u UU N... 1 2 3 4 a b c x\x00? n\x00")
	f.Fuzz(func(t *testing.T, out string) {
		st := parseStatus(out)
		for _, file := range st.Files {
			if file.Index == 0 || file.Work == 0 {
				t.Fatalf("a file without a state: %+v", file)
			}
		}
	})
}
