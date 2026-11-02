package shell

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/listing"
)

// asmListing is a NASM source of the project and the listing its build profile writes.
type asmListing struct {
	source  string // absolute
	listing string // absolute
}

// nasmListings finds, in the build profiles, each NASM run that writes a listing (-l file):
// the source it assembles and the listing. Paths are relative to the profile's folder.
func nasmListings(root string, c *config.Config) []asmListing {
	var out []asmListing
	for _, p := range c.Profiles {
		tool := strings.TrimSuffix(strings.ToLower(filepath.Base(p.Tool)), ".exe")
		if tool != "nasm" {
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(p.SourceDir))
		var lst string
		var sources []string
		for i := 0; i < len(p.Args); i++ {
			a := p.Args[i]
			switch {
			case a == "-l" && i+1 < len(p.Args):
				i++
				lst = p.Args[i]
			case strings.HasPrefix(a, "-l") && len(a) > 2:
				lst = a[2:]
			case len(a) == 2 && a[0] == '-' && strings.ContainsRune("ofIiDPpUEWMLw", rune(a[1])) && i+1 < len(p.Args):
				i++ // an option's value, such as the output after -o
			case !strings.HasPrefix(a, "-"):
				switch strings.ToLower(filepath.Ext(a)) {
				case ".asm", ".nasm", ".s":
					sources = append(sources, a)
				}
			}
		}
		if lst == "" || len(sources) != 1 { // one listing names the lines of one source
			continue
		}
		abs := func(p string) string {
			if filepath.IsAbs(p) {
				return filepath.Clean(p)
			}
			return filepath.Join(dir, filepath.FromSlash(p))
		}
		out = append(out, asmListing{source: abs(sources[0]), listing: abs(lst)})
	}
	return out
}

// loadListing reads l's listing as it is now (the source may have been built again) and the
// address its code runs at: its org, or for a boot image without one, where the BIOS loads it.
func loadListing(l asmListing, c *config.Config) (*listing.Map, uint64, bool) {
	f, err := os.Open(l.listing)
	if err != nil {
		return nil, 0, false
	}
	defer f.Close()
	m, err := listing.ParseNASM(f)
	if err != nil {
		return nil, 0, false
	}
	switch {
	case m.HasOrg:
		return m, m.Org, true
	case c.Qemu != nil && c.Qemu.BootImage != "":
		return m, 0x7c00, true
	}
	return nil, 0, false
}

// listingAddr is where a line of a NASM source of the project runs, from its build listing, for
// a flat binary GDB has no lines for (#61).
func (m *machine) listingAddr(file string, line int) (uint64, bool) {
	c, err := config.Load(m.s.root)
	if err != nil {
		return 0, false
	}
	for _, l := range nasmListings(m.s.root, c) {
		if !sameFile(resolved(l.source), resolved(file)) {
			continue
		}
		if lm, base, ok := loadListing(l, c); ok {
			return lm.Addr(line, base)
		}
	}
	return 0, false
}

// listingLine is the NASM source line assembled at addr, from the project's listings: where a
// flat binary stopped.
func (m *machine) listingLine(addr uint64) (string, int, bool) {
	c, err := config.Load(m.s.root)
	if err != nil {
		return "", 0, false
	}
	for _, l := range nasmListings(m.s.root, c) {
		if lm, base, ok := loadListing(l, c); ok {
			if line, ok := lm.Line(addr, base); ok {
				return l.source, line, true
			}
		}
	}
	return "", 0, false
}
