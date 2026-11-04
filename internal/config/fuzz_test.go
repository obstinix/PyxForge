package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// FuzzParse feeds pyxforge.toml parsing arbitrary text: a project's file is whatever its
// author wrote (#43).
func FuzzParse(f *testing.F) {
	seeds, _ := filepath.Glob(filepath.Join("..", "..", "examples", "*", "pyxforge.toml"))
	for _, s := range seeds {
		if data, err := os.ReadFile(s); err == nil {
			f.Add(string(data))
		}
	}
	f.Add("[project]\nname = \"x\"\n[profiles.debug]\ntool = \"nasm\"\nargs = [\"-f\", \"bin\"]\n")
	f.Add("[project]\nname = \"x\"\n[qemu]\nmachine = \"pc\"\nextra_args = [\"-s\"]\n[qemu.debug]\nport = 1234\n")
	f.Add("[project]\n")
	f.Fuzz(func(t *testing.T, s string) {
		c, err := Parse(s)
		if err != nil {
			return
		}
		_ = c.Validate()
		for _, name := range c.ProfileNames() {
			if _, err := c.Profile(name); err != nil {
				t.Fatalf("profile %q is listed but %v", name, err)
			}
			_ = c.GdbFor(name)
		}
	})
}

var (
	fuzzTable = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)?$`)
	fuzzKey   = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// lookup returns table.key from a decoded document; table may be dotted ("qemu.debug").
func lookup(doc map[string]any, table, key string) any {
	m := doc
	for _, part := range strings.Split(table, ".") {
		next, ok := m[part].(map[string]any)
		if !ok {
			return nil
		}
		m = next
	}
	return m[key]
}

// FuzzSetValue checks the in-place editor Set Up Machine uses (#69): whatever it writes is
// TOML that holds the new value, or it refuses.
func FuzzSetValue(f *testing.F) {
	f.Add("[project]\nname = \"x\"\n\n[qemu]\nmachine = \"pc\" # the default\nmemory = 32\n", "qemu", "machine", "q35")
	f.Add("[project]\nname = \"x\"\n", "qemu.debug", "port", "1234")
	f.Add("[project]\r\nname = \"x\"\r\n[gdb]\r\n", "gdb", "path", `C:\tools\gdb.exe`)
	f.Add("[qemu]\nextra_args = [\n  \"-s\",\n]\nmachine = 'pc'\n", "qemu", "machine", "isapc")
	f.Add("qemu.machine = \"pc\"\n", "qemu", "machine", "pc")
	f.Add("x = \"\"\"\n[qemu]\nmachine = 1\n\"\"\"\n", "qemu", "machine", "pc")
	f.Fuzz(func(t *testing.T, text, table, key, value string) {
		var before map[string]any
		if _, err := toml.Decode(text, &before); err != nil {
			return // only files that are TOML to start with
		}
		if !fuzzTable.MatchString(table) || !fuzzKey.MatchString(key) {
			return
		}
		quoted, err := TOMLString(value)
		if err != nil {
			return
		}
		out, err := SetValue(text, table, key, quoted)
		if err != nil {
			return
		}
		var after map[string]any
		if _, err := toml.Decode(out, &after); err != nil {
			t.Fatalf("SetValue(%q, %q, %q, %q) wrote a file that is not TOML (%v):\n%s", text, table, key, value, err, out)
		}
		if got := lookup(after, table, key); got != value {
			t.Fatalf("SetValue(%q, %q, %q, %q) left %s.%s = %#v:\n%s", text, table, key, value, table, key, got, out)
		}
	})
}
