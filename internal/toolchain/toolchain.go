// Package toolchain finds the external tools PyxForge drives (Neovim, assemblers, compilers,
// linkers, QEMU, GDB, Git) and reports their versions. The CLI's `doctor` command and the
// desktop app share it, so both say the same thing about the same machine.
package toolchain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/obstinix/PyxForge/internal/proc"
)

// Area groups tools by the workflow step they serve.
type Area string

const (
	AreaEditor  Area = "Editor"
	AreaBuild   Area = "Build"
	AreaRun     Area = "Run"
	AreaDebug   Area = "Debug"
	AreaInspect Area = "Inspect"
	AreaSource  Area = "Source control"
	AreaLang    Area = "Language servers"
)

// Areas lists the areas in the order reports show them.
var Areas = []Area{AreaEditor, AreaLang, AreaBuild, AreaRun, AreaDebug, AreaInspect, AreaSource}

// Tool is one external program PyxForge can use.
type Tool struct {
	ID          string   // stable identifier, e.g. "qemu-x86_64"
	Label       string   // shown to people, e.g. "QEMU (x86-64)"
	Area        Area     // workflow step
	Candidates  []string // executable names to try, in order
	VersionArgs []string // arguments that print a version and exit
	Optional    bool     // missing it limits some targets only
	// Dirs are extra install folders searched after PATH, for installers that do not add
	// themselves to it (QEMU on Windows). Environment variables are expanded.
	Dirs []string
	// Bundled are folders, relative to PyxForge's own, where a release ships the tool; they
	// are searched before PATH, so the copy PyxForge was tested with is the one used.
	Bundled []string
	Hint    map[string]string // install hint by GOOS
	Feature string            // what PyxForge does with it, for people deciding to install it
}

// Tools is everything PyxForge drives, in report order.
var Tools = []Tool{
	{ID: "nvim", Label: "Neovim", Area: AreaEditor, Candidates: []string{"nvim"}, VersionArgs: []string{"--version"},
		Bundled: []string{filepath.Join("nvim", "bin")},
		Hint:    map[string]string{"windows": "winget install --id Neovim.Neovim --exact", "linux": "install Neovim 0.12 or newer from your distribution or neovim.io"}, Feature: "the editor"},
	{ID: "tree-sitter", Label: "tree-sitter CLI", Area: AreaEditor, Candidates: []string{"tree-sitter"}, VersionArgs: []string{"--version"}, Optional: true,
		Hint: map[string]string{"windows": "cargo install tree-sitter-cli --locked, then pyxforge setup editor", "linux": "cargo install tree-sitter-cli --locked, then pyxforge setup editor"}, Feature: "compiling extra syntax parsers in pyxforge setup editor"},
	{ID: "clangd", Label: "clangd (C, C++)", Area: AreaLang, Candidates: []string{"clangd"}, VersionArgs: []string{"--version"}, Optional: true,
		Hint: map[string]string{"windows": "installed with LLVM-MinGW", "linux": "sudo apt-get install clangd"}, Feature: "completion and diagnostics while editing C and C++"},
	{ID: "gopls", Label: "gopls (Go)", Area: AreaLang, Candidates: []string{"gopls"}, VersionArgs: []string{"version"}, Optional: true,
		Hint: map[string]string{"windows": "go install golang.org/x/tools/gopls@latest", "linux": "go install golang.org/x/tools/gopls@latest"}, Feature: "completion and diagnostics while editing Go"},
	{ID: "rust-analyzer", Label: "rust-analyzer (Rust)", Area: AreaLang, Candidates: []string{"rust-analyzer"}, VersionArgs: []string{"--version"}, Optional: true,
		Hint: map[string]string{"windows": "rustup component add rust-analyzer", "linux": "rustup component add rust-analyzer"}, Feature: "completion and diagnostics while editing Rust"},
	{ID: "asm-lsp", Label: "asm-lsp (assembly)", Area: AreaLang, Candidates: []string{"asm-lsp"}, VersionArgs: []string{"version"}, Optional: true,
		Hint: map[string]string{"windows": "cargo install asm-lsp --locked", "linux": "cargo install asm-lsp --locked"}, Feature: "completion and hover help while editing assembly"},
	{ID: "lua-language-server", Label: "lua-language-server (Lua)", Area: AreaLang, Candidates: []string{"lua-language-server"}, VersionArgs: []string{"--version"}, Optional: true,
		Hint: map[string]string{"windows": "download a release from github.com/LuaLS/lua-language-server", "linux": "download a release from github.com/LuaLS/lua-language-server"}, Feature: "completion and diagnostics while editing Lua"},
	{ID: "nasm", Label: "NASM", Area: AreaBuild, Candidates: []string{"nasm"}, VersionArgs: []string{"-v"},
		Hint: map[string]string{"windows": "winget install --id NASM.NASM --exact", "linux": "sudo apt-get install nasm"}, Feature: "building assembly projects that use it"},
	{ID: "cc", Label: "C compiler", Area: AreaBuild, Candidates: []string{"gcc", "clang", "cc"}, VersionArgs: []string{"--version"},
		Hint: map[string]string{"windows": "winget install --id MartinStorsjo.LLVM-MinGW.UCRT --exact", "linux": "sudo apt-get install gcc"}, Feature: "building C code in projects that use it"},
	{ID: "ld", Label: "Linker", Area: AreaBuild, Candidates: []string{"ld", "ld.lld"}, VersionArgs: []string{"--version"},
		Hint: map[string]string{"windows": "installed with LLVM-MinGW or WinLibs", "linux": "sudo apt-get install binutils"}, Feature: "linking kernels in projects that use it"},
	{ID: "make", Label: "Make", Area: AreaBuild, Candidates: []string{"make", "mingw32-make"}, VersionArgs: []string{"--version"}, Optional: true,
		Hint: map[string]string{"windows": "installed with LLVM-MinGW or WinLibs as mingw32-make", "linux": "sudo apt-get install make"}, Feature: "projects whose build profiles run make"},
	{ID: "qemu-x86_64", Label: "QEMU (x86-64)", Area: AreaRun, Candidates: []string{"qemu-system-x86_64"}, VersionArgs: []string{"--version"},
		Dirs: []string{`${ProgramFiles}\qemu`},
		Hint: map[string]string{"windows": "winget install --id SoftwareFreedomConservancy.QEMU --exact", "linux": "sudo apt-get install qemu-system-x86"}, Feature: "Run and Debug of x86 boot sectors and kernels"},
	{ID: "qemu-i386", Label: "QEMU (i386)", Area: AreaRun, Candidates: []string{"qemu-system-i386"}, VersionArgs: []string{"--version"}, Optional: true,
		Dirs: []string{`${ProgramFiles}\qemu`},
		Hint: map[string]string{"windows": "installed with QEMU", "linux": "sudo apt-get install qemu-system-x86"}, Feature: "projects that set qemu-system-i386"},
	{ID: "qemu-arm", Label: "QEMU (ARM)", Area: AreaRun, Candidates: []string{"qemu-system-arm"}, VersionArgs: []string{"--version"}, Optional: true,
		Dirs: []string{`${ProgramFiles}\qemu`},
		Hint: map[string]string{"windows": "installed with QEMU", "linux": "sudo apt-get install qemu-system-arm"}, Feature: "projects that set qemu-system-arm"},
	{ID: "qemu-img", Label: "qemu-img (machine snapshots)", Area: AreaRun, Candidates: []string{"qemu-img"}, VersionArgs: []string{"--version"}, Optional: true,
		Dirs: []string{`${ProgramFiles}\qemu`},
		Hint: map[string]string{"windows": "installed with QEMU", "linux": "sudo apt-get install qemu-utils"}, Feature: "saving and restoring machine states (Snapshots tab)"},
	{ID: "gdb", Label: "GDB", Area: AreaDebug, Candidates: []string{"gdb"}, VersionArgs: []string{"--version"},
		Hint: map[string]string{"windows": "winget install --id BrechtSanders.WinLibs.POSIX.UCRT --exact", "linux": "sudo apt-get install gdb"}, Feature: "Debug in QEMU: breakpoints, stepping, registers, memory"},
	{ID: "gdb-multiarch", Label: "GDB (multi-arch)", Area: AreaDebug, Candidates: []string{"gdb-multiarch"}, VersionArgs: []string{"--version"}, Optional: true,
		Hint: map[string]string{"linux": "sudo apt-get install gdb-multiarch", "windows": "use gdb-multiarch in WSL for ARM targets"}, Feature: "debugging ARM targets"},
	{ID: "objdump", Label: "Disassembler", Area: AreaInspect, Candidates: []string{"objdump", "llvm-objdump"}, VersionArgs: []string{"--version"},
		Hint: map[string]string{"windows": "installed with LLVM-MinGW or WinLibs", "linux": "sudo apt-get install binutils"}, Feature: "disassembly by external tools; PyxForge has its own decoder"},
	{ID: "git", Label: "Git", Area: AreaSource, Candidates: []string{"git"}, VersionArgs: []string{"--version"},
		Dirs: []string{`${ProgramFiles}\Git\cmd`, `${LOCALAPPDATA}\Programs\Git\cmd`}, // Git for Windows, all users or one
		Hint: map[string]string{"windows": "winget install --id Git.Git --exact", "linux": "sudo apt-get install git"}, Feature: "the Git tab, and fetching editor plugins in pyxforge setup editor"},
}

// Status is what detection found for one tool.
type Status struct {
	Tool    Tool
	Path    string // absolute path of the executable found; empty when missing
	Source  string // where it was found: SourceConfigured, SourceBundled, SourcePath or SourceFolder
	Version string // version number, or the first line of its version output
	Err     error  // why the version could not be read, when the tool was found
}

// Found reports whether the tool's executable exists.
func (s Status) Found() bool { return s.Path != "" }

// Hint is the install suggestion for the running platform, or "".
func (s Status) Hint() string { return s.Tool.Hint[runtime.GOOS] }

// ByID returns the status for a tool ID.
func ByID(st []Status, id string) (Status, bool) {
	for _, s := range st {
		if s.Tool.ID == id {
			return s, true
		}
	}
	return Status{}, false
}

// Detector runs detection. Its hooks exist so tests can replace the operating system.
type Detector struct {
	LookPath  func(name string) (string, error)
	Run       func(ctx context.Context, path string, args []string) (string, error)
	Getenv    func(string) string
	Overrides func() Overrides // paths the user set, by tool ID; nil means none
	BundleDir string           // the folder Bundled paths are relative to; "" skips them
	Timeout   time.Duration    // per version query
}

// Detect checks every tool in Tools with the default detector.
func Detect(ctx context.Context) []Status { return New().Detect(ctx, Tools) }

// New returns a detector that uses the real PATH and runs version queries directly, never
// through a shell.
func New() *Detector {
	return &Detector{LookPath: exec.LookPath, Run: runVersion, Getenv: os.Getenv, Overrides: CurrentOverrides,
		BundleDir: bundleDir(), Timeout: 5 * time.Second}
}

// Detect checks tools concurrently and returns their statuses in the same order.
func (d *Detector) Detect(ctx context.Context, tools []Tool) []Status {
	out := make([]Status, len(tools))
	var wg sync.WaitGroup
	for i, t := range tools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = d.detect(ctx, t)
		}()
	}
	wg.Wait()
	return out
}

func (d *Detector) detect(ctx context.Context, t Tool) Status {
	s := Status{Tool: t}
	s.Path, s.Source, s.Err = d.find(t)
	if s.Path == "" {
		return s
	}
	vctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()
	out, err := d.Run(vctx, s.Path, t.VersionArgs)
	if err != nil && strings.TrimSpace(out) == "" {
		if errors.Is(vctx.Err(), context.DeadlineExceeded) {
			err = errors.New("version query timed out")
		}
		s.Err = err
		return s
	}
	first, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	if first = strings.TrimSpace(first); strings.HasPrefix(strings.ToLower(first), "error") {
		s.Err = errors.New(first) // the tool rejected the version arguments
		return s
	}
	s.Version = ParseVersion(out)
	return s
}

// find looks for a tool: the path the user set, a copy bundled with PyxForge, PATH, then the
// tool's install folders. A path the user set that is gone is reported rather than passed
// over: the user chose it, and quietly using another copy would hide that.
func (d *Detector) find(t Tool) (path, source string, err error) {
	if d.Overrides != nil {
		if p := d.Overrides()[t.ID]; p != "" {
			found, err := d.LookPath(p)
			if err != nil {
				return "", SourceConfigured, fmt.Errorf("the path set in Doctor, %s, is not an executable", p)
			}
			return found, SourceConfigured, nil
		}
	}
	if d.BundleDir != "" {
		for _, dir := range t.Bundled {
			for _, name := range t.Candidates {
				if p, err := d.LookPath(filepath.Join(d.BundleDir, dir, name)); err == nil {
					return p, SourceBundled, nil
				}
			}
		}
	}
	// exec.LookPath returns absolute paths: it refuses matches in relative PATH entries.
	for _, name := range t.Candidates {
		if p, err := d.LookPath(name); err == nil {
			return p, SourcePath, nil
		}
	}
	for _, dir := range t.Dirs {
		dir = os.Expand(dir, d.Getenv)
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		for _, name := range t.Candidates {
			if p, err := d.LookPath(filepath.Join(dir, name)); err == nil {
				return p, SourceFolder, nil
			}
		}
	}
	return "", "", nil
}

// LookPath finds an executable by name in the order doctor reports it: the path set in
// Doctor, a bundled copy, PATH, then the tool's install folders, so running a tool uses what
// doctor showed. A name no tool lists is looked up on PATH only; a path is used as given.
func LookPath(name string) (string, error) {
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return exec.LookPath(name)
	}
	base := strings.TrimSuffix(strings.ToLower(name), ".exe")
	for _, t := range Tools {
		if !slices.Contains(t.Candidates, base) {
			continue
		}
		t.Candidates = []string{base} // the program asked for, not another candidate
		p, _, err := New().find(t)
		if err != nil {
			return "", err
		}
		if p != "" {
			return p, nil
		}
		return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	return exec.LookPath(name)
}

func runVersion(ctx context.Context, path string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = time.Second // do not wait on pipes a killed child left open
	proc.NoWindow(cmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var versionRE = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)

// ParseVersion extracts the version number from a tool's version output: the first dotted
// number on the first non-empty line, or that whole line when it has none.
func ParseVersion(out string) string {
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if v := versionRE.FindString(line); v != "" {
			return v
		}
		return line
	}
	return ""
}
