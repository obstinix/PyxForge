package listing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzParseNASM reads arbitrary NASM listings: they come from the user's own build (#43).
func FuzzParseNASM(f *testing.F) {
	for _, name := range []string{"boot.lst", "include.lst"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(data))
	}
	f.Add("     1 00000000 90 nop\n")
	f.Add("     1                                  org 0x7c00\n     2 00000000 EBFE <1> jmp $\n")
	f.Fuzz(func(t *testing.T, text string) {
		m, err := ParseNASM(strings.NewReader(text))
		if err != nil || m == nil {
			return
		}
		// Looking up any line or address is safe, and the address found for a line maps back
		// to a line.
		for line := -1; line < 50; line++ {
			if addr, ok := m.Addr(line, 0x7c00); ok {
				if _, back := m.Line(addr, 0x7c00); !back {
					t.Fatalf("line %d is at %#x, but %#x has no line", line, addr, addr)
				}
			}
		}
		_, _ = m.Line(0, 0)
		_, _ = m.Line(^uint64(0), 0x7c00)
	})
}
