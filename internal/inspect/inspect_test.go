package inspect

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// The first three cases port legacy/core/src/hex.rs's tests.
func TestDumpAndBootSector(t *testing.T) {
	if d, b := Dump(nil, 0), BootSector(nil); d != nil || b.Size != 0 || b.IsSector || b.Signature {
		t.Errorf("empty: %v %+v", d, b)
	}

	data := make([]byte, 512)
	data[0], data[1] = 0xeb, 0x3c // jmp short
	data[510], data[511] = 0x55, 0xaa
	lines := Dump(data, 0)
	b := BootSector(data)
	if !b.IsSector || !b.Signature || len(lines) != 32 || lines[0].Hex[0] != "eb" || lines[0].Hex[1] != "3c" ||
		lines[31].Offset != 496 || lines[31].Hex[14] != "55" || lines[31].Hex[15] != "aa" {
		t.Errorf("valid sector: %+v, %d lines", b, len(lines))
	}
	if b.Used != 2 || b.Free != 508 {
		t.Errorf("footprint: used %d, free %d", b.Used, b.Free)
	}

	data[510] = 0
	if b := BootSector(data); !b.IsSector || b.Signature {
		t.Errorf("bad signature: %+v", b)
	}

	// A disk image longer than a sector still has its first sector checked.
	img := make([]byte, 4096)
	copy(img, "\xfa\x31\xc0")
	img[510], img[511] = 0x55, 0xaa
	if b := BootSector(img); b.IsSector || !b.Signature || b.Used != 3 {
		t.Errorf("disk image: %+v", b)
	}

	got := Dump([]byte("PyxForge OK\r\n\x00\x01"), 0x7c00)[0].String()
	if want := "00007c00  50 79 78 46 6f 72 67 65  20 4f 4b 0d 0a 00 01     |PyxForge OK....|"; got != want {
		t.Errorf("line:\n got %q\nwant %q", got, want)
	}
}

func TestDisassemble(t *testing.T) {
	code := []byte{0xBA, 0xF8, 0x03, 0xB0, 'O', 0xEE, 0xFA, 0xF4, 0xEB, 0xFD, 0x0F}
	ins := Disassemble(code, 0x7c00, ModeOf("i8086"), true)
	var texts []string
	for _, in := range ins {
		texts = append(texts, in.Text)
	}
	want := []string{"mov dx, 0x3f8", "mov al, 0x4f", "out dx, al", "cli", "hlt", "jmp 0x7c07", "(bad)"}
	if !slices.Equal(texts, want) {
		t.Errorf("real mode, Intel syntax: %q", texts)
	}
	if ins[1].Addr != 0x7c03 || len(ins[0].Bytes) != 3 {
		t.Errorf("addresses: %+v", ins[:2])
	}
	if s := ins[0].String(); !strings.HasPrefix(s, "    7c00:  ba f8 03") || !strings.HasSuffix(s, "mov dx, 0x3f8") {
		t.Errorf("listing line %q", s)
	}

	// The same bytes in protected mode are a different instruction: modes matter.
	if pm := Disassemble(code[:5], 0, ModeOf("i386"), false); pm[0].Text != "mov $0x4fb003f8,%edx" {
		t.Errorf("protected mode, AT&T syntax: %q", pm[0].Text)
	}
	if ModeOf("auto") != 16 || ModeOf("i386:x86-64") != 64 {
		t.Error("ModeOf")
	}
}

func TestReadELF(t *testing.T) {
	f, err := os.Open("testdata/kernel.elf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	e, err := ReadELF(f)
	if err != nil {
		t.Fatal(err)
	}
	if e.Class != "ELF32" || e.Machine != "EM_386" || e.Type != "ET_EXEC" || e.Entry != 0x100000 || e.Mode != 32 {
		t.Errorf("header: %+v", e)
	}
	var names []string
	for _, s := range e.Sections {
		names = append(names, s.Name)
		if s.Name == ".text" && (s.Addr != 0x100000 || s.Flags != "AX") {
			t.Errorf(".text: %+v", s)
		}
		if s.Name == ".bss" && (s.Size != 4096 || s.Flags != "WA") {
			t.Errorf(".bss: %+v", s)
		}
	}
	if !slices.Contains(names, ".text") || !slices.Contains(names, ".bss") {
		t.Errorf("sections %v", names)
	}
	if !slices.ContainsFunc(e.Segments, func(s Segment) bool { return s.Flags == "R-X" && s.VAddr == 0x100000 }) {
		t.Errorf("segments %+v", e.Segments)
	}
	var syms []string
	for _, s := range e.Symbols {
		syms = append(syms, s.Name)
	}
	if len(syms) < 3 || syms[0] != "kmain" || !slices.Contains(syms, "clear") || !slices.Contains(syms, "stack_top") {
		t.Errorf("symbols %v", syms)
	}

	code, start, err := CodeAt(f, e.Entry)
	if err != nil || start != 0x100000 {
		t.Fatal(err)
	}
	if ins := Disassemble(code, start, e.Mode, true); !strings.HasPrefix(ins[0].Text, "mov esp, 0x") || !strings.HasPrefix(ins[1].Text, "call ") {
		t.Errorf("entry: %+v", ins[:2])
	}
	if _, err := ReadELF(strings.NewReader("MZ not an ELF")); err == nil {
		t.Error("a non-ELF file was accepted")
	}
}
