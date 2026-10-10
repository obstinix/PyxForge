// Package config reads and validates pyxforge.toml, a project's build profiles, QEMU and GDB
// settings. The schema and every validation message match the 2.x Rust core (legacy/core,
// config.rs), whose tests are ported here as golden cases, so existing projects keep working.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the configuration file at a project's root.
const FileName = "pyxforge.toml"

// ValidGdbArchitectures are the values gdb.architecture accepts. "arm" is new in 3.0: the 2.x
// Embedded preset wrote it but validation rejected it (parity matrix row 2).
var ValidGdbArchitectures = []string{"i8086", "i386", "i386:x86-64", "auto", "arm"}

// Config is a parsed and validated pyxforge.toml.
type Config struct {
	Project  Project
	Profiles map[string]*Profile
	Qemu     *Qemu // nil when the file has no [qemu] table
	Gdb      *Gdb  // nil when the file has no [gdb] table

	order    []string // profile names in file order
	Warnings []string // keys PyxForge does not know; ignored, as 2.x did
}

// Project is the [project] table.
type Project struct {
	Name        string
	Description string
}

// Profile is one [profiles.<name>] table: a build step.
type Profile struct {
	Name        string
	Tool        string // program to run, e.g. "nasm", "make", "gcc"
	Description string
	SourceDir   string // working folder relative to the project root; default "."
	OutputDir   string // output folder relative to the project root; default "build"
	Args        []string
	Env         map[string]string
	DependsOn   []string
	Gdb         *ProfileGdb // per-profile overrides, nil when absent
}

// Qemu is the [qemu] table.
type Qemu struct {
	Executable string   `json:"executable"`          // default "qemu-system-x86_64"
	Machine    string   `json:"machine"`             // default "pc"
	Memory     string   `json:"memory"`              // default "128M"
	BootImage  string   `json:"bootImage,omitempty"` // raw disk image to boot, relative to the project root
	Kernel     string   `json:"kernel,omitempty"`    // kernel image for -kernel, relative to the project root
	ExtraArgs  []string `json:"extraArgs,omitempty"`
	// Snapshots boots the image through a qcow2 overlay so QEMU can save and restore machine
	// state (3.0; 2.x ignores the key).
	Snapshots bool      `json:"snapshots,omitempty"`
	Debug     QemuDebug `json:"debug"`

	hasBootImage, hasKernel bool // the keys were present, even if empty
}

// QemuDebug is [qemu.debug].
type QemuDebug struct {
	Enabled bool   `json:"enabled"` // default true: start paused with a GDB stub
	GdbPort uint16 `json:"gdbPort"` // default 1234
}

// Gdb is the [gdb] table.
type Gdb struct {
	Executable   string `json:"executable"`   // default "gdb"
	Architecture string `json:"architecture"` // default "i8086"
}

// ProfileGdb is [profiles.<name>.gdb]; empty fields fall back to [gdb].
type ProfileGdb struct {
	Executable   string
	Architecture string
}

// The file's shape: pointers tell an absent key from an empty one, so defaults apply only to
// keys that are missing, exactly as serde's #[serde(default)] does.
type fileConfig struct {
	Project  *fileProject           `toml:"project"`
	Profiles map[string]fileProfile `toml:"profiles"`
	Qemu     *fileQemu              `toml:"qemu"`
	Gdb      *fileGdb               `toml:"gdb"`
}

type fileProject struct {
	Name        *string `toml:"name"`
	Description string  `toml:"description"`
}

type fileProfile struct {
	Tool        *string           `toml:"tool"`
	Description string            `toml:"description"`
	SourceDir   *string           `toml:"source_dir"`
	OutputDir   *string           `toml:"output_dir"`
	Args        []string          `toml:"args"`
	Env         map[string]string `toml:"env"`
	DependsOn   []string          `toml:"depends_on"`
	Gdb         *fileProfileGdb   `toml:"gdb"`
}

type fileProfileGdb struct {
	Executable   *string `toml:"executable"`
	Architecture *string `toml:"architecture"`
}

type fileQemu struct {
	Executable *string        `toml:"executable"`
	Machine    *string        `toml:"machine"`
	Memory     *string        `toml:"memory"`
	BootImage  *string        `toml:"boot_image"`
	Kernel     *string        `toml:"kernel"`
	ExtraArgs  []string       `toml:"extra_args"`
	Snapshots  bool           `toml:"snapshots"`
	Debug      *fileQemuDebug `toml:"debug"`
}

type fileQemuDebug struct {
	Enabled *bool   `toml:"enabled"`
	GdbPort *uint16 `toml:"gdb_port"`
}

type fileGdb struct {
	Executable   *string `toml:"executable"`
	Architecture *string `toml:"architecture"`
}

// ErrNotFound is returned by Load when the project has no pyxforge.toml.
var ErrNotFound = errors.New("no " + FileName)

// Load reads, parses and validates root/pyxforge.toml.
func Load(root string) (*Config, error) {
	path := filepath.Join(root, FileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		//lint:ignore ST1005 the message matches the 2.x core word for word (parity)
		return nil, fmt.Errorf("No %s found in %s: %w", FileName, root, ErrNotFound)
	}
	if err != nil {
		//lint:ignore ST1005 the message matches the 2.x core word for word (parity)
		return nil, fmt.Errorf("Failed to read %s: %w", path, err)
	}
	c, err := Parse(string(data))
	if err != nil {
		//lint:ignore ST1005 the message matches the 2.x core word for word (parity)
		return nil, fmt.Errorf("Failed to parse %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Parse decodes TOML into a Config and applies defaults. It does not validate; Load does.
func Parse(s string) (*Config, error) {
	var f fileConfig
	md, err := toml.Decode(s, &f)
	if err != nil {
		return nil, err
	}
	if f.Project == nil {
		return nil, errors.New("missing field `project`")
	}
	if f.Project.Name == nil {
		return nil, errors.New("missing field `name` in [project]")
	}
	c := &Config{
		Project:  Project{Name: *f.Project.Name, Description: f.Project.Description},
		Profiles: map[string]*Profile{},
	}
	for _, k := range md.Undecoded() {
		c.Warnings = append(c.Warnings, fmt.Sprintf("unknown key %s is ignored", k))
	}
	for _, k := range md.Keys() {
		if len(k) == 2 && k[0] == "profiles" {
			c.order = append(c.order, k[1])
		}
	}
	for name, fp := range f.Profiles {
		if fp.Tool == nil {
			return nil, fmt.Errorf("missing field `tool` in [profiles.%s]", name)
		}
		p := &Profile{
			Name: name, Tool: *fp.Tool, Description: fp.Description,
			SourceDir: deref(fp.SourceDir, "."), OutputDir: deref(fp.OutputDir, "build"),
			Args: fp.Args, Env: fp.Env, DependsOn: fp.DependsOn,
		}
		if p.Env == nil {
			p.Env = map[string]string{}
		}
		if fp.Gdb != nil {
			p.Gdb = &ProfileGdb{Executable: deref(fp.Gdb.Executable, ""), Architecture: deref(fp.Gdb.Architecture, "")}
		}
		c.Profiles[name] = p
	}
	if q := f.Qemu; q != nil {
		c.Qemu = &Qemu{
			Executable: deref(q.Executable, "qemu-system-x86_64"),
			Machine:    deref(q.Machine, "pc"),
			Memory:     deref(q.Memory, "128M"),
			ExtraArgs:  q.ExtraArgs,
			Snapshots:  q.Snapshots,
			Debug:      QemuDebug{Enabled: true, GdbPort: 1234},
		}
		c.Qemu.BootImage, c.Qemu.hasBootImage = deref(q.BootImage, ""), q.BootImage != nil
		c.Qemu.Kernel, c.Qemu.hasKernel = deref(q.Kernel, ""), q.Kernel != nil
		if d := q.Debug; d != nil {
			if d.Enabled != nil {
				c.Qemu.Debug.Enabled = *d.Enabled
			}
			if d.GdbPort != nil {
				c.Qemu.Debug.GdbPort = *d.GdbPort
			}
		}
	}
	if g := f.Gdb; g != nil {
		c.Gdb = &Gdb{Executable: deref(g.Executable, "gdb"), Architecture: deref(g.Architecture, "i8086")}
	}
	return c, nil
}

func deref(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

// Validate checks the rules 2.x enforced, with its messages.
func (c *Config) Validate() error {
	if c.Project.Name == "" {
		return errors.New("project.name must not be empty")
	}
	for _, name := range c.ProfileNames() {
		for _, dep := range c.Profiles[name].DependsOn {
			if _, ok := c.Profiles[dep]; !ok {
				return fmt.Errorf("Profile '%s' depends on '%s', which does not exist", name, dep)
			}
		}
	}
	if q := c.Qemu; q != nil {
		if !q.hasBootImage && !q.hasKernel {
			//lint:ignore ST1005 the message matches the 2.x core word for word (parity)
			return errors.New("Either qemu.boot_image or qemu.kernel must be specified inside [qemu]")
		}
		if q.hasBootImage && q.BootImage == "" {
			return errors.New("qemu.boot_image must not be empty if configured")
		}
		if q.hasKernel && q.Kernel == "" {
			return errors.New("qemu.kernel must not be empty if configured")
		}
	}
	valid := strings.Join(ValidGdbArchitectures, ", ")
	if g := c.Gdb; g != nil && !slices.Contains(ValidGdbArchitectures, g.Architecture) {
		return fmt.Errorf("gdb.architecture '%s' is invalid. Valid values: %s", g.Architecture, valid)
	}
	for _, name := range c.ProfileNames() {
		if g := c.Profiles[name].Gdb; g != nil && g.Architecture != "" && !slices.Contains(ValidGdbArchitectures, g.Architecture) {
			return fmt.Errorf("profile '%s' gdb.architecture '%s' is invalid. Valid values: %s", name, g.Architecture, valid)
		}
	}
	return nil
}

// ProfileNames lists profiles in the order the file defines them.
func (c *Config) ProfileNames() []string {
	if len(c.order) == len(c.Profiles) {
		return slices.Clone(c.order)
	}
	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Profile looks up a profile by name; the error lists the available ones.
func (c *Config) Profile(name string) (*Profile, error) {
	if p, ok := c.Profiles[name]; ok {
		return p, nil
	}
	names := make([]string, 0, len(c.Profiles))
	for n := range c.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	//lint:ignore ST1005 the message matches the 2.x core word for word (parity)
	return nil, fmt.Errorf("Unknown profile '%s'. Available profiles: [%s]", name, strings.Join(names, ", "))
}

// GdbFor resolves the GDB executable and architecture for a profile: the profile's override,
// then [gdb], then the defaults.
func (c *Config) GdbFor(profile string) Gdb {
	g := Gdb{Executable: "gdb", Architecture: "i8086"}
	if c.Gdb != nil {
		g = *c.Gdb
	}
	if p, ok := c.Profiles[profile]; ok && p.Gdb != nil {
		if p.Gdb.Executable != "" {
			g.Executable = p.Gdb.Executable
		}
		if p.Gdb.Architecture != "" {
			g.Architecture = p.Gdb.Architecture
		}
	}
	return g
}
