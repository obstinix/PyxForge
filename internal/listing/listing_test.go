package listing

import (
	"os"
	"strings"
	"testing"
)

// TestBootSectorListing: the example boot sector's listing (NASM 2.16) gives each instruction's
// address under its org, and each address back its line.
func TestBootSectorListing(t *testing.T) {
	f, err := os.Open("testdata/boot.lst")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := ParseNASM(f)
	if err != nil {
		t.Fatal(err)
	}
	if !m.HasOrg || m.Org != 0x7c00 {
		t.Fatalf("org = %#x (%v)", m.Org, m.HasOrg)
	}
	for line, want := range map[int]uint64{8: 0x7c00, 9: 0x7c01, 18: 0x7c10, 31: 0x7c22, 47: 0x7dfe} {
		if got, ok := m.Addr(line, m.Org); !ok || got != want {
			t.Errorf("line %d at %#x (%v), want %#x", line, got, ok, want)
		}
	}
	for _, line := range []int{1, 5, 7, 17, 25} { // comments, directives, labels, blank lines
		if addr, ok := m.Addr(line, m.Org); ok {
			t.Errorf("line %d, which assembles nothing, has address %#x", line, addr)
		}
	}
	if line, ok := m.Line(0x7c22, m.Org); !ok || line != 31 {
		t.Errorf("0x7c22 is line %d (%v), want 31", line, ok)
	}
	if _, ok := m.Line(0x7c21, m.Org); ok {
		t.Error("an address inside an instruction was given a line")
	}
	if _, ok := m.Line(0x100, m.Org); ok {
		t.Error("an address below the org was given a line")
	}
	// A data line continued over several listing lines keeps its first offset.
	if got, _ := m.Addr(44, m.Org); got != 0x7c31 {
		t.Errorf("the message is at %#x", got)
	}
}

// TestIncludedAndMacroLines: lines marked <n> belong to an included file or a macro's
// expansion (numbered by the macro's own lines), not to the main file. The listing is NASM's
// for an %include and a two-line macro.
func TestIncludedAndMacroLines(t *testing.T) {
	f, err := os.Open("testdata/include.lst")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := ParseNASM(f)
	if err != nil {
		t.Fatal(err)
	}
	if m.Org != 0x7c00 {
		t.Errorf("org = %#x", m.Org)
	}
	for _, line := range []int{2, 5, 6, 8} { // the include, the macro's body, its call
		if got, ok := m.Addr(line, m.Org); ok {
			t.Errorf("line %d of the main file got %#x from an included or expanded line", line, got)
		}
	}
	for line, want := range map[int]uint64{3: 0x7c02, 9: 0x7c06} {
		if got, ok := m.Addr(line, m.Org); !ok || got != want {
			t.Errorf("line %d at %#x (%v), want %#x", line, got, ok, want)
		}
	}
	for in, want := range map[string]uint64{"0x7C00": 0x7c00, "7c00h": 0x7c00, "$7c00": 0x7c00, "31744": 31744} {
		if v, ok := number(in); !ok || v != want {
			t.Errorf("number(%q) = %#x, %v", in, v, ok)
		}
	}
	if m, _ := ParseNASM(strings.NewReader("     1 00000000 90 nop\n")); m.HasOrg {
		t.Error("a listing without org has one")
	}
	if m, _ := ParseNASM(strings.NewReader("     1                                  [org 7c00h] ; boot\n")); m.Org != 0x7c00 {
		t.Errorf("[org 7c00h] read as %#x", m.Org)
	}
}
