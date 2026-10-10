// Package inspect reads build output the way an OS developer looks at it: a hex and ASCII dump,
// the facts that decide whether a BIOS will boot a sector, x86 disassembly in real, protected
// or long mode, and an ELF file's headers, sections and symbols.
package inspect

import (
	"fmt"
	"strings"

	"golang.org/x/arch/x86/x86asm"
)

// Line is one row of a hex dump: up to 16 bytes.
type Line struct {
	Offset uint64   // address of the first byte
	Hex    []string // two hex digits per byte
	ASCII  string   // printable bytes, '.' for the rest
}

// String formats the line like `hexdump -C`.
func (l Line) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%08x  ", l.Offset)
	for i := range 16 {
		switch {
		case i < len(l.Hex):
			b.WriteString(l.Hex[i])
			b.WriteByte(' ')
		default:
			b.WriteString("   ")
		}
		if i == 7 {
			b.WriteByte(' ')
		}
	}
	b.WriteString(" |" + l.ASCII + "|")
	return b.String()
}

// Dump splits data into hex dump lines starting at base (2.x parity: hex.rs).
func Dump(data []byte, base uint64) []Line {
	var out []Line
	for off := 0; off < len(data); off += 16 {
		chunk := data[off:min(off+16, len(data))]
		l := Line{Offset: base + uint64(off)}
		var ascii strings.Builder
		for _, c := range chunk {
			l.Hex = append(l.Hex, fmt.Sprintf("%02x", c))
			if c >= 0x20 && c <= 0x7e {
				ascii.WriteByte(c)
			} else {
				ascii.WriteByte('.')
			}
		}
		l.ASCII = ascii.String()
		out = append(out, l)
	}
	return out
}

// Boot is what decides whether a BIOS boots an image's first sector.
type Boot struct {
	Size        int  // the image's size in bytes
	IsSector    bool // exactly 512 bytes: a lone boot sector (2.x is_boot_sector)
	Signature   bool // the first sector ends with 0x55 0xAA (2.x has_boot_signature for 512-byte files)
	Used        int  // bytes of the first sector before the zero padding that precedes the signature
	Free        int  // bytes left for code and data in the first sector (510 - Used)
	SignatureAt int  // where the signature belongs: 510
}

// BootSector checks the first sector of an image.
func BootSector(data []byte) Boot {
	b := Boot{Size: len(data), IsSector: len(data) == 512, SignatureAt: 510}
	if len(data) >= 512 {
		b.Signature = data[510] == 0x55 && data[511] == 0xaa
		used := 510
		for used > 0 && data[used-1] == 0 {
			used--
		}
		b.Used, b.Free = used, 510-used
	} else {
		b.Used, b.Free = len(data), max(510-len(data), 0)
	}
	return b
}

// Mode is an x86 processor mode in bits: 16 (real mode), 32 (protected), 64 (long).
type Mode int

// ModeOf maps a GDB architecture name to the mode its code runs in.
func ModeOf(arch string) Mode {
	switch arch {
	case "i386":
		return 32
	case "i386:x86-64":
		return 64
	}
	return 16 // i8086, and "auto", which 2.x resolves to i8086
}

// Instruction is one decoded instruction.
type Instruction struct {
	Addr  uint64
	Bytes []byte
	Text  string
}

// Disassemble decodes x86 code at addr. Undecodable bytes become one-byte "(bad)" entries, so
// the listing stays aligned with memory. With intel the syntax is NASM-like; otherwise AT&T.
func Disassemble(code []byte, addr uint64, mode Mode, intel bool) []Instruction {
	var out []Instruction
	for off := 0; off < len(code); {
		pc := addr + uint64(off)
		inst, err := x86asm.Decode(code[off:], int(mode))
		if err != nil || inst.Len == 0 || inst.Op == 0 { // Op 0: only prefixes, no instruction
			out = append(out, Instruction{Addr: pc, Bytes: code[off : off+1], Text: "(bad)"})
			off++
			continue
		}
		text := x86asm.GNUSyntax(inst, pc, nil)
		if intel {
			text = strings.ToLower(x86asm.IntelSyntax(inst, pc, nil))
		}
		out = append(out, Instruction{Addr: pc, Bytes: code[off : off+inst.Len], Text: text})
		off += inst.Len
	}
	return out
}

// String formats an instruction like objdump: address, bytes, text.
func (in Instruction) String() string {
	hex := make([]string, len(in.Bytes))
	for i, b := range in.Bytes {
		hex[i] = fmt.Sprintf("%02x", b)
	}
	return fmt.Sprintf("%8x:  %-21s %s", in.Addr, strings.Join(hex, " "), in.Text)
}

// Format names what kind of file data is, from its first bytes: "elf", "pe" (Windows
// executables), "mach-o", or "raw" for anything else, such as a boot image.
func Format(data []byte) string {
	switch {
	case len(data) >= 4 && string(data[:4]) == "\x7fELF":
		return "elf"
	case len(data) >= 2 && string(data[:2]) == "MZ":
		return "pe"
	case len(data) >= 4 && (string(data[:4]) == "\xfe\xed\xfa\xce" || string(data[:4]) == "\xfe\xed\xfa\xcf" ||
		string(data[:4]) == "\xce\xfa\xed\xfe" || string(data[:4]) == "\xcf\xfa\xed\xfe"):
		return "mach-o"
	}
	return "raw"
}

// Multiboot reports which Multiboot header an image carries: 1 (magic 0x1BADB002, 4-byte
// aligned in the first 8 KiB), 2 (0xE85250D6, 8-byte aligned in the first 32 KiB), or 0. A
// Multiboot loader starts the kernel in 32-bit protected mode, whatever its ELF class says.
func Multiboot(data []byte) int {
	le := func(i int) uint32 {
		return uint32(data[i]) | uint32(data[i+1])<<8 | uint32(data[i+2])<<16 | uint32(data[i+3])<<24
	}
	for i := 0; i+4 <= min(len(data), 8192); i += 4 {
		if le(i) == 0x1BADB002 {
			return 1
		}
	}
	for i := 0; i+4 <= min(len(data), 32768); i += 8 {
		if le(i) == 0xE85250D6 {
			return 2
		}
	}
	return 0
}
