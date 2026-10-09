package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The first sixteen tests are the golden cases: each ports one test from the 2.x core
// (legacy/core/src/config.rs, `mod tests`), with the same input and the same assertions.

func mustParse(t *testing.T, s string) *Config {
	t.Helper()
	c, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return c
}

func wantErr(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Fatalf("error = %v, want one containing %q", err, substr)
	}
}

// 1. test_parse_minimal_config
func TestParseMinimalConfig(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[profiles.bootloader]
tool = "nasm"
args = ["-f", "bin", "boot.asm", "-o", "boot.bin"]
`)
	if c.Project.Name != "test-os" || len(c.Profiles) != 1 {
		t.Fatalf("project %q, %d profiles", c.Project.Name, len(c.Profiles))
	}
	b := c.Profiles["bootloader"]
	if b.Tool != "nasm" || !reflect.DeepEqual(b.Args, []string{"-f", "bin", "boot.asm", "-o", "boot.bin"}) ||
		b.SourceDir != "." || b.OutputDir != "build" || len(b.DependsOn) != 0 {
		t.Errorf("bootloader = %+v", b)
	}
}

// 2. test_parse_full_config
func TestParseFullConfig(t *testing.T) {
	c := mustParse(t, `
[project]
name = "my-os"
description = "A test operating system"

[profiles.bootloader]
tool = "nasm"
description = "Assemble the bootloader"
source_dir = "boot"
output_dir = "build"
args = ["-f", "bin", "boot.asm", "-o", "boot.bin"]

[profiles.kernel]
tool = "make"
description = "Build the kernel"
source_dir = "."
output_dir = "build"
args = ["all"]
depends_on = ["bootloader"]

[profiles.kernel.env]
CC = "x86_64-elf-gcc"
`)
	if c.Project.Name != "my-os" || len(c.Profiles) != 2 {
		t.Fatalf("project %q, %d profiles", c.Project.Name, len(c.Profiles))
	}
	k := c.Profiles["kernel"]
	if k.Tool != "make" || !reflect.DeepEqual(k.DependsOn, []string{"bootloader"}) || k.Env["CC"] != "x86_64-elf-gcc" {
		t.Errorf("kernel = %+v", k)
	}
}

// 3. test_validate_missing_dependency
func TestValidateMissingDependency(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[profiles.kernel]
tool = "make"
depends_on = ["bootloader"]
`)
	wantErr(t, c.Validate(), "does not exist")
}

// 4. test_validate_empty_project_name
func TestValidateEmptyProjectName(t *testing.T) {
	c := mustParse(t, `
[project]
name = ""

[profiles.bootloader]
tool = "nasm"
`)
	wantErr(t, c.Validate(), "must not be empty")
}

// 5. test_get_profile_found
func TestGetProfileFound(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[profiles.bootloader]
tool = "nasm"
`)
	p, err := c.Profile("bootloader")
	if err != nil || p.Tool != "nasm" {
		t.Errorf("Profile(bootloader) = %+v, %v", p, err)
	}
}

// 6. test_get_profile_not_found
func TestGetProfileNotFound(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[profiles.bootloader]
tool = "nasm"
`)
	_, err := c.Profile("nonexistent")
	wantErr(t, err, "Unknown profile")
}

// 7. test_parse_qemu_config_defaults
func TestParseQemuConfigDefaults(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[qemu]
boot_image = "build/boot.bin"
`)
	q := c.Qemu
	if q == nil {
		t.Fatal("no [qemu]")
	}
	if q.Executable != "qemu-system-x86_64" || q.Machine != "pc" || q.Memory != "128M" ||
		q.BootImage != "build/boot.bin" || q.Kernel != "" || len(q.ExtraArgs) != 0 ||
		!q.Debug.Enabled || q.Debug.GdbPort != 1234 {
		t.Errorf("qemu = %+v", q)
	}
}

// 8. test_parse_qemu_config_custom
func TestParseQemuConfigCustom(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[qemu]
executable = "qemu-system-i386"
machine = "q35"
memory = "256M"
boot_image = "build/custom_boot.bin"
extra_args = ["-nographic", "-serial", "mon:stdio"]

[qemu.debug]
enabled = false
gdb_port = 5678
`)
	q := c.Qemu
	if q == nil {
		t.Fatal("no [qemu]")
	}
	if q.Executable != "qemu-system-i386" || q.Machine != "q35" || q.Memory != "256M" ||
		q.BootImage != "build/custom_boot.bin" || q.Kernel != "" ||
		!reflect.DeepEqual(q.ExtraArgs, []string{"-nographic", "-serial", "mon:stdio"}) ||
		q.Debug.Enabled || q.Debug.GdbPort != 5678 {
		t.Errorf("qemu = %+v", q)
	}
}

// 9. test_validate_missing_qemu_boot_image
func TestValidateMissingQemuBootImage(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[qemu]
boot_image = ""
`)
	wantErr(t, c.Validate(), "qemu.boot_image must not be empty")
}

// 10. test_validate_missing_both_boot_image_and_kernel
func TestValidateMissingBothBootImageAndKernel(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[qemu]
executable = "qemu-system-x86_64"
`)
	wantErr(t, c.Validate(), "Either qemu.boot_image or qemu.kernel must be specified")
}

// 11. test_parse_gdb_config_defaults
func TestParseGdbConfigDefaults(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[gdb]
`)
	if c.Gdb == nil || c.Gdb.Executable != "gdb" || c.Gdb.Architecture != "i8086" {
		t.Errorf("gdb = %+v", c.Gdb)
	}
}

// 12. test_parse_gdb_config_custom
func TestParseGdbConfigCustom(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[gdb]
executable = "x86_64-elf-gdb"
architecture = "i386"
`)
	if c.Gdb == nil || c.Gdb.Executable != "x86_64-elf-gdb" || c.Gdb.Architecture != "i386" {
		t.Errorf("gdb = %+v", c.Gdb)
	}
}

// 13. test_validate_invalid_gdb_architecture
func TestValidateInvalidGdbArchitecture(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[gdb]
architecture = "arm64"
`)
	wantErr(t, c.Validate(), "gdb.architecture 'arm64' is invalid")
}

// 14. test_no_gdb_section_is_valid
func TestNoGdbSectionIsValid(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"
`)
	if c.Gdb != nil {
		t.Error("an absent [gdb] was filled in")
	}
	if err := c.Validate(); err != nil {
		t.Error(err)
	}
}

// 15. test_profile_gdb_overrides
func TestProfileGdbOverrides(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[profiles.kernel]
tool = "make"
[profiles.kernel.gdb]
executable = "my-gdb"
architecture = "i386:x86-64"
`)
	g := c.Profiles["kernel"].Gdb
	if g == nil || g.Executable != "my-gdb" || g.Architecture != "i386:x86-64" {
		t.Errorf("kernel gdb = %+v", g)
	}
	if err := c.Validate(); err != nil {
		t.Error(err)
	}
}

// 16. test_profile_invalid_gdb_architecture
func TestProfileInvalidGdbArchitecture(t *testing.T) {
	c := mustParse(t, `
[project]
name = "test-os"

[profiles.kernel]
tool = "make"
[profiles.kernel.gdb]
architecture = "arm32"
`)
	wantErr(t, c.Validate(), "profile 'kernel' gdb.architecture 'arm32' is invalid")
}

// Beyond 2.x.

// TestArmArchitecture is parity row 2: the 2.x Embedded preset wrote architecture = "arm",
// which validation rejected, so every command failed for those projects.
func TestArmArchitecture(t *testing.T) {
	c := mustParse(t, `
[project]
name = "embedded"

[profiles.firmware]
tool = "arm-none-eabi-gcc"

[qemu]
executable = "qemu-system-arm"
machine = "virt"
kernel = "build/firmware.elf"

[gdb]
executable = "gdb-multiarch"
architecture = "arm"
`)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if g := c.GdbFor("firmware"); g.Executable != "gdb-multiarch" || g.Architecture != "arm" {
		t.Errorf("GdbFor = %+v", g)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir); !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "No pyxforge.toml found in") {
		t.Errorf("missing file: %v", err)
	}
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, FileName), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("[project\nname = 1")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "Failed to parse") || !strings.Contains(err.Error(), "line") {
		t.Errorf("malformed file: %v", err)
	}
	write("[profiles.boot]\ntool = \"nasm\"\n")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "missing field `project`") {
		t.Errorf("no [project]: %v", err)
	}
	write("[project]\nname = \"x\"\n[profiles.boot]\nargs = []\n")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "missing field `tool` in [profiles.boot]") {
		t.Errorf("profile without tool: %v", err)
	}
	write("[project]\nname = \"x\"\n[profiles.kernel]\ntool = \"make\"\ndepends_on = [\"boot\"]\n")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Load did not validate: %v", err)
	}
	write("[project]\nname = \"x\"\nauthor = \"me\"\n[profiles.c]\ntool = \"gcc\"\n[profiles.a]\ntool = \"nasm\"\n[profiles.b]\ntool = \"ld\"\n")
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ProfileNames(); !reflect.DeepEqual(got, []string{"c", "a", "b"}) {
		t.Errorf("profiles out of file order: %v", got)
	}
	if len(c.Warnings) != 1 || !strings.Contains(c.Warnings[0], "project.author") {
		t.Errorf("unknown keys: %v", c.Warnings)
	}
	_, err = c.Profile("z")
	wantErr(t, err, "Available profiles: [a, b, c]")
}

func TestGdbForFallsBack(t *testing.T) {
	c := mustParse(t, "[project]\nname = \"x\"\n[profiles.k]\ntool = \"make\"\n[profiles.k.gdb]\narchitecture = \"i386\"\n")
	if g := c.GdbFor("k"); g.Executable != "gdb" || g.Architecture != "i386" {
		t.Errorf("profile override without [gdb]: %+v", g)
	}
	if g := c.GdbFor("missing"); g.Executable != "gdb" || g.Architecture != "i8086" {
		t.Errorf("defaults: %+v", g)
	}
}
