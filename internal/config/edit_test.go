package config

import (
	"errors"
	"strings"
	"testing"
)

const machineToml = `# PyxForge project
[project]
name = "boot" # the name

[qemu]
executable = "qemu-system-i386"
memory = "64M"   # enough for a boot sector
boot_image = "build/boot.bin"

[qemu.debug]
enabled = true
gdb_port = 1234

[[build.profiles]]
name = "boot"
`

// TestSetValue: a key's line changes and keeps its comment; a new key joins its table; a new
// table goes at the end; every other line is unchanged (#69).
func TestSetValue(t *testing.T) {
	text, err := SetValue(machineToml, "qemu", "memory", `"256M"`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, `memory = "256M"   # enough for a boot sector`) {
		t.Errorf("the comment was lost:\n%s", text)
	}
	if text, err = SetValue(text, "qemu", "machine", `"q35"`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "boot_image = \"build/boot.bin\"\nmachine = \"q35\"\n\n[qemu.debug]") {
		t.Errorf("the new key is not at the end of [qemu]:\n%s", text)
	}
	if text, err = SetValue(text, "qemu.debug", "gdb_port", "4321"); err != nil {
		t.Fatal(err)
	}
	if text, err = SetValue(text, "gdb", "architecture", `"i8086"`); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(text, "name = \"boot\"\n\n[gdb]\narchitecture = \"i8086\"\n") {
		t.Errorf("the new table is not at the end:\n%s", text)
	}
	text = RemoveKey(text, "qemu", "boot_image")
	c, err := Parse(text)
	if err != nil {
		t.Fatalf("the edited file does not parse: %v\n%s", err, text)
	}
	if c.Qemu.Memory != "256M" || c.Qemu.Machine != "q35" || c.Qemu.Debug.GdbPort != 4321 || c.Qemu.BootImage != "" || c.Gdb.Architecture != "i8086" || c.Project.Name != "boot" {
		t.Errorf("parsed back: %+v %+v", c.Qemu, c.Gdb)
	}
	for _, keep := range []string{"# PyxForge project", `name = "boot" # the name`, "[[build.profiles]]"} {
		if !strings.Contains(text, keep) {
			t.Errorf("%q was lost", keep)
		}
	}

	// A comment after a string holding '#' is told from the string.
	if got, _ := SetValue("[qemu]\nkernel = \"a#b.elf\" # note\n", "qemu", "kernel", `"k.elf"`); got != "[qemu]\nkernel = \"k.elf\" # note\n" {
		t.Errorf("got %q", got)
	}
	// Windows line endings stay.
	if got, _ := SetValue("[qemu]\r\nmemory = \"1M\"\r\n", "qemu", "memory", `"2M"`); got != "[qemu]\r\nmemory = \"2M\"\r\n" {
		t.Errorf("got %q", got)
	}
	// Inline tables and dotted keys are left to the user.
	if _, err := SetValue("[qemu]\ndebug = { enabled = true, gdb_port = 1 }\n", "qemu", "debug", "x"); !errors.Is(err, ErrNotEditable) {
		t.Errorf("an inline table: %v", err)
	}
	if _, err := SetValue("[qemu]\ndebug = { enabled = true, gdb_port = 1 }\n", "qemu.debug", "gdb_port", "2"); !errors.Is(err, ErrNotEditable) {
		t.Errorf("a key inside an inline table: %v", err)
	}
	if _, err := SetValue("qemu.memory = \"1M\"\n", "qemu", "memory", `"2M"`); !errors.Is(err, ErrNotEditable) {
		t.Errorf("a dotted key: %v", err)
	}
	if s, err := TOMLString(`C:\qemu\"x"`); err != nil || s != `"C:\\qemu\\\"x\""` {
		t.Errorf("TOMLString = %s, %v", s, err)
	}
}

// TestEditsSkipMultiLineValues: lines inside a multi-line string or array are text, not keys or
// headers, so an edit never lands inside one (found by FuzzSetValue, #43).
func TestEditsSkipMultiLineValues(t *testing.T) {
	// A "[qemu]" line inside a multi-line string is not the [qemu] table.
	text := "[project]\nname = \"x\"\nnotes = \"\"\"\n[qemu]\nmachine = \"old\"\n\"\"\"\n"
	got, err := SetValue(text, "qemu", "machine", `"pc"`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, text) || !strings.HasSuffix(got, "\n[qemu]\nmachine = \"pc\"\n") {
		t.Errorf("the string was edited instead of a new table added:\n%s", got)
	}
	// A new key goes after a multi-line array, not into it.
	text = "[qemu]\nextra_args = [\n  \"-s\",\n  \"-S\",\n]\n"
	if got, err = SetValue(text, "qemu", "machine", `"pc"`); err != nil || got != text+"machine = \"pc\"\n" {
		t.Errorf("got %q, %v", got, err)
	}
	// A value that is itself a multi-line string is not rewritten line by line.
	if _, err := SetValue("[qemu]\nmachine = \"\"\"\npc\n\"\"\"\n", "qemu", "machine", `"q35"`); !errors.Is(err, ErrNotEditable) {
		t.Errorf("a multi-line string value: %v", err)
	}
	// Removing a key removes all of its value.
	text = "[qemu]\nkernel = '''\nk.elf\n'''\nmemory = \"1M\"\n"
	if got := RemoveKey(text, "qemu", "kernel"); got != "[qemu]\nmemory = \"1M\"\n" {
		t.Errorf("RemoveKey left %q", got)
	}
	// A file that isn't TOML is not edited.
	if _, err := SetValue("[qemu\nmachine = \"pc\"\n", "qemu", "machine", `"q35"`); !errors.Is(err, ErrNotEditable) {
		t.Errorf("a broken file: %v", err)
	}
	if bad := "[qemu]\nmachine = \"pc\"\nmachine = \"q35\"\n"; RemoveKey(bad, "qemu", "machine") != bad {
		t.Error("RemoveKey changed a file that is not TOML")
	}
}
