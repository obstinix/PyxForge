package inspect

import (
	"debug/elf"
	"fmt"
	"io"
	"slices"
	"strings"
)

// ELF is the part of an ELF file a kernel developer checks: where it loads and starts, its
// sections, and its symbols.
type ELF struct {
	Class    string // "ELF32" or "ELF64"
	Machine  string // "EM_386", "EM_X86_64", "EM_ARM"…
	Type     string // "ET_EXEC", "ET_REL"…
	Entry    uint64
	Mode     Mode // the x86 mode its code is in, 0 for other machines
	Sections []Section
	Segments []Segment
	Symbols  []Symbol // functions and objects with addresses, by address
}

// Section is one section header.
type Section struct {
	Name  string
	Addr  uint64
	Size  uint64
	Flags string // "AX" allocated and executable, "WA" writable…
}

// Segment is one program header that loads.
type Segment struct {
	VAddr, PAddr, FileSize, MemSize uint64
	Flags                           string // "R-X"…
}

// Symbol is a named address.
type Symbol struct {
	Name string
	Addr uint64
	Size uint64
	Kind string // "func" or "object"
}

// ReadELF summarises an ELF file. It reports an error for anything that is not ELF.
func ReadELF(r io.ReaderAt) (*ELF, error) {
	f, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	e := &ELF{Class: classOf(f.Class), Machine: f.Machine.String(), Type: f.Type.String(), Entry: f.Entry}
	switch f.Machine {
	case elf.EM_386:
		e.Mode = 32
	case elf.EM_X86_64:
		e.Mode = 64
	}
	for _, s := range f.Sections {
		if s.Name == "" {
			continue
		}
		e.Sections = append(e.Sections, Section{Name: s.Name, Addr: s.Addr, Size: s.Size, Flags: sectionFlags(s.Flags)})
	}
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD {
			continue
		}
		e.Segments = append(e.Segments, Segment{VAddr: p.Vaddr, PAddr: p.Paddr, FileSize: p.Filesz, MemSize: p.Memsz,
			Flags: segmentFlags(p.Flags)})
	}
	syms, _ := f.Symbols() // a stripped file has none; that is not an error
	for _, s := range syms {
		kind := ""
		switch elf.ST_TYPE(s.Info) {
		case elf.STT_FUNC:
			kind = "func"
		case elf.STT_OBJECT:
			kind = "object"
		case elf.STT_NOTYPE:
			if s.Section != elf.SHN_UNDEF && s.Name != "" && !strings.HasPrefix(s.Name, ".") {
				kind = "label" // assembly labels carry no type
			}
		}
		if kind == "" || s.Section == elf.SHN_UNDEF {
			continue
		}
		e.Symbols = append(e.Symbols, Symbol{Name: s.Name, Addr: s.Value, Size: s.Size, Kind: kind})
	}
	slices.SortStableFunc(e.Symbols, func(a, b Symbol) int {
		if a.Addr != b.Addr {
			if a.Addr < b.Addr {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return e, nil
}

// CodeAt returns the bytes of the section that holds addr, and the section's start, so a caller
// can disassemble around an address.
func CodeAt(r io.ReaderAt, addr uint64) ([]byte, uint64, error) {
	f, err := elf.NewFile(r)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	for _, s := range f.Sections {
		if s.Type == elf.SHT_PROGBITS && s.Flags&elf.SHF_ALLOC != 0 && addr >= s.Addr && addr < s.Addr+s.Size {
			data, err := s.Data()
			return data, s.Addr, err
		}
	}
	return nil, 0, fmt.Errorf("no section holds 0x%x", addr)
}

func classOf(c elf.Class) string {
	if c == elf.ELFCLASS64 {
		return "ELF64"
	}
	return "ELF32"
}

func sectionFlags(f elf.SectionFlag) string {
	var b strings.Builder
	if f&elf.SHF_WRITE != 0 {
		b.WriteByte('W')
	}
	if f&elf.SHF_ALLOC != 0 {
		b.WriteByte('A')
	}
	if f&elf.SHF_EXECINSTR != 0 {
		b.WriteByte('X')
	}
	return b.String()
}

func segmentFlags(f elf.ProgFlag) string {
	out := []byte("---")
	if f&elf.PF_R != 0 {
		out[0] = 'R'
	}
	if f&elf.PF_W != 0 {
		out[1] = 'W'
	}
	if f&elf.PF_X != 0 {
		out[2] = 'X'
	}
	return string(out)
}
