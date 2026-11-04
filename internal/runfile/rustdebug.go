package runfile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/obstinix/PyxForge/internal/proc"
)

// gnuWindows is the Rust target whose debug information GDB reads on Windows (DWARF); the MSVC
// one writes PDB files, which it does not (#64).
const gnuWindows = "x86_64-pc-windows-gnu"

// rustInfo reports the host rustc builds for and whether the GNU Windows target's standard
// library is installed. Tests replace it.
var rustInfo = func(rustc string) (host string, hasGNU bool) {
	run := func(args ...string) string {
		cmd := exec.Command(rustc, args...)
		proc.NoWindow(cmd)
		out, _ := cmd.Output()
		return string(out)
	}
	for _, l := range strings.Split(run("-vV"), "\n") {
		if h, ok := strings.CutPrefix(strings.TrimSpace(l), "host: "); ok {
			host = h
		}
	}
	sysroot := strings.TrimSpace(run("--print", "sysroot"))
	_, err := os.Stat(filepath.Join(sysroot, "lib", "rustlib", gnuWindows))
	return host, sysroot != "" && err == nil
}

// rustDebug plans Rust under GDB: built without optimisation and with debug information, stopped
// at the editor's breakpoints in the file or at the crate's main. On Windows with the MSVC
// toolchain it builds for the GNU target, whose debug information GDB can read, when that target
// is installed, and says how to install it when not.
func rustDebug(file, dir, out string, look func(string) (string, error), breaks Breakpoints) (Plan, error) {
	gdbHint := hint("winget install --id BrechtSanders.WinLibs.POSIX.UCRT --exact", "sudo apt-get install gdb")
	if _, err := look("gdb"); err != nil {
		return Plan{}, &MissingError{Language: "Rust debugging", Tools: []string{"gdb"}, Hint: gdbHint}
	}
	p, err := For(file, out, look)
	if err != nil {
		return Plan{}, err
	}
	var target []string
	if runtime.GOOS == "windows" {
		rustc, err := look("rustc")
		if err != nil {
			return Plan{}, &MissingError{Language: "Rust debugging", Tools: []string{"rustc"}, Hint: "install Rust with rustup from rustup.rs"}
		}
		if host, hasGNU := rustInfo(rustc); strings.HasSuffix(host, "-msvc") {
			if !hasGNU {
				return Plan{}, &MissingError{Language: "Rust debugging (GDB cannot read the MSVC toolchain's debug information)",
					Tools: []string{"the " + gnuWindows + " target"}, Hint: "rustup target add " + gnuWindows}
			}
			target = []string{"--target", gnuWindows}
		}
	}
	var exe, crate string
	if p.Run.Tool == "cargo" { // a crate: built by cargo, debugged where cargo puts it
		name, err := cargoPackage(filepath.Join(p.Run.Dir, "Cargo.toml"))
		if err != nil {
			return Plan{}, err
		}
		crate = name
		p.Build = []Step{{Tool: "cargo", Args: append([]string{"build"}, target...), Dir: p.Run.Dir}}
		folder := filepath.Join(p.Run.Dir, "target", "debug")
		if target != nil {
			folder = filepath.Join(p.Run.Dir, "target", gnuWindows, "debug")
		}
		exe = filepath.Join(folder, name)
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		p.Out = ""
	} else {
		crate = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		exe = p.Run.Tool
		p.Build[0].Args = append(append([]string{"-g", "-C", "opt-level=0"}, target...), p.Build[0].Args...)
	}
	crate = strings.ReplaceAll(crate, "-", "_")
	start := ""
	for _, b := range breaks.at(func(f string) bool { return samePath(f, file) }) {
		start += fmt.Sprintf("break %s:%d\n", filepath.Base(file), b.line)
	}
	if start == "" {
		start = "break " + crate + "::main\n"
	}
	setup, script := filepath.Join(outFor(out, file), crate+".setup.gdb"), filepath.Join(outFor(out, file), crate+".start.gdb")
	p.Files = map[string]string{setup: "set pagination off\nset debuginfod enabled off\n", script: start + "run\n"}
	p.Run = Step{Tool: "gdb", Args: []string{"-q", "-x", setup, "-x", script, "--args", exe}, Dir: dir}
	return p, nil
}

// cargoPackage is the package name in a Cargo.toml: the name of the program cargo builds.
func cargoPackage(manifest string) (string, error) {
	var m struct {
		Package struct{ Name string } `toml:"package"`
	}
	if _, err := toml.DecodeFile(manifest, &m); err != nil {
		return "", fmt.Errorf("%s: %w", manifest, err)
	}
	if m.Package.Name == "" {
		return "", fmt.Errorf("%s names no package", manifest)
	}
	return m.Package.Name, nil
}
