// Package listing reads NASM listing files (nasm -l), which say at what offset each source line
// was assembled. A flat binary, such as a boot sector, has no debug information, so this is how
// a line of its source becomes an address GDB can stop at, and an address a line (#61).
package listing

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Map is what one listing says about its main source file.
type Map struct {
	Org    uint64 // the origin set by an org directive; 0 when there is none
	HasOrg bool
	lines  map[int]uint64 // source line → offset of its first byte
	addrs  map[uint64]int // offset → source line
}

// listing lines: a line number, an offset and the bytes assembled, then the source text, which
// starts with a level (<1>) for a line of an included file or a macro expansion; a line that
// assembles nothing has no offset or bytes.
var (
	entry  = regexp.MustCompile(`^\s*(\d+)\s+([0-9A-Fa-f]{8})\s+([0-9A-Fa-f\[\]()]+-?)\s*(.*)$`)
	text   = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
	level  = regexp.MustCompile(`^<\d+>`)
	orgDir = regexp.MustCompile(`(?i)^\s*\[?\s*org\s+([0-9A-Fa-fxXhH$]+)\s*\]?\s*(;.*)?$`)
)

// ParseNASM reads a NASM listing. Lines from included files and macro expansions (marked <n>)
// are left out: their numbers are not lines of the main file.
func ParseNASM(r io.Reader) (*Map, error) {
	m := &Map{lines: map[int]uint64{}, addrs: map[uint64]int{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		l := sc.Text()
		if e := entry.FindStringSubmatch(l); e != nil {
			if level.MatchString(e[4]) {
				continue
			}
			line, _ := strconv.Atoi(e[1])
			off, err := strconv.ParseUint(e[2], 16, 64)
			if err != nil {
				continue
			}
			if _, seen := m.lines[line]; !seen {
				m.lines[line] = off
			}
			if _, seen := m.addrs[off]; !seen {
				m.addrs[off] = line
			}
			continue
		}
		if t := text.FindStringSubmatch(l); t != nil && !level.MatchString(t[2]) {
			if o := orgDir.FindStringSubmatch(t[2]); o != nil && !m.HasOrg {
				if v, ok := number(o[1]); ok {
					m.Org, m.HasOrg = v, true
				}
			}
		}
	}
	return m, sc.Err()
}

// number reads a NASM number: 0x7c00, 7c00h, $7c00 or 31744.
func number(s string) (uint64, bool) {
	s = strings.ToLower(s)
	base := 10
	switch {
	case strings.HasPrefix(s, "0x"):
		s, base = s[2:], 16
	case strings.HasPrefix(s, "$"):
		s, base = s[1:], 16
	case strings.HasSuffix(s, "h"):
		s, base = s[:len(s)-1], 16
	}
	v, err := strconv.ParseUint(s, base, 64)
	return v, err == nil
}

// Addr is the address of line when the binary is loaded at base (the org, for a boot
// sector): base plus the line's offset. ok is false for a line that assembles nothing.
func (m *Map) Addr(line int, base uint64) (addr uint64, ok bool) {
	off, ok := m.lines[line]
	return base + off, ok
}

// Line is the source line assembled at addr, for a binary loaded at base; ok is false for an
// address that starts no line.
func (m *Map) Line(addr, base uint64) (line int, ok bool) {
	if addr < base {
		return 0, false
	}
	line, ok = m.addrs[addr-base]
	return line, ok
}
