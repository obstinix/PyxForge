// Package cli is PyxForge's command line. It shares its packages with the desktop app (tool
// detection, workspace discovery, build information), so both report the same facts.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/workspace"
	"github.com/obstinix/PyxForge/nvim"
)

// Exit codes.
const (
	ExitOK      = 0
	ExitFailure = 1 // the command ran and found a problem (a required tool is missing)
	ExitUsage   = 2 // the command line was wrong
)

// Env is what commands read and write. Tests replace every part of it.
type Env struct {
	Ctx    context.Context
	Stdout io.Writer
	Stderr io.Writer
	Getwd  func() (string, error)
	Detect func(context.Context) []toolchain.Status
}

// DefaultEnv uses the real process: its streams and working folder, and a context that Ctrl+C
// cancels, which also stops any tool a command is waiting on. Call stop when done.
func DefaultEnv() (env Env, stop func()) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	return Env{Ctx: ctx, Stdout: os.Stdout, Stderr: os.Stderr, Getwd: os.Getwd, Detect: toolchain.Detect}, stop
}

// Result says what to do after Run: exit with Exit, or open the desktop app on Folder, with
// Files open in the editor.
type Result struct {
	Exit    int
	OpenGUI bool
	Folder  string
	Files   []string
}

type command struct {
	name, args, summary string
	run                 func(env Env, args []string) Result
}

var commands []command

func init() {
	commands = []command{
		{"open", "[folder|file]", "Open the desktop app on a folder, or on a file's project with the file open (the default)", runOpen},
		{"build", "[profile...] [--list] [--json] [-C folder]", "Run build profiles from pyxforge.toml, dependencies first (default: the profiles nothing depends on)", runBuild},
		{"info", "[folder] [--json]", "Show the project, configuration file and Git checkout a folder belongs to", runInfo},
		{"doctor", "[--json]", "Check the tools PyxForge drives and how to install missing ones", runDoctor},
		{"setup", "editor [--no-parsers]", "Install the editor's pinned plugins and Tree-sitter parsers (uses the network)", runSetup},
		{"version", "[--json]", "Print the PyxForge version and how it was built", runVersion},
		{"help", "[command]", "Show help for PyxForge or one command", runHelp},
	}
}

func find(name string) (command, bool) {
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// Run executes a command line (without the program name).
func Run(args []string, env Env) Result {
	if len(args) == 0 {
		return runOpen(env, nil)
	}
	switch args[0] {
	case "-h", "--help":
		return runHelp(env, args[1:])
	case "--version":
		return runVersion(env, args[1:])
	}
	if c, ok := find(args[0]); ok {
		return c.run(env, args[1:])
	}
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(env.Stderr, "pyxforge: unknown option %s\nRun 'pyxforge help' for usage.\n", args[0])
		return Result{Exit: ExitUsage}
	}
	// pyxforge <folder> opens it, as the desktop app always has.
	return runOpen(env, args)
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `PyxForge %s: a native environment for bootloader, kernel and bare-metal work.

Usage:
  pyxforge [folder|file]       open the desktop app (the current folder by default)
  pyxforge <command> [options]

Commands:
`, buildinfo.Version)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, c := range commands {
		fmt.Fprintf(tw, "  %s %s\t%s\n", c.name, c.args, c.summary)
	}
	tw.Flush()
	fmt.Fprint(w, `
A folder named like a command opens with 'pyxforge open <folder>'.
Exit status: 0 success, 1 a problem was found, 2 the command line was wrong.
`)
}

func runHelp(env Env, args []string) Result {
	if len(args) == 0 {
		usage(env.Stdout)
		return Result{Exit: ExitOK}
	}
	c, ok := find(args[0])
	if !ok {
		fmt.Fprintf(env.Stderr, "pyxforge: no command %q\nRun 'pyxforge help' for the list.\n", args[0])
		return Result{Exit: ExitUsage}
	}
	fmt.Fprintf(env.Stdout, "Usage: pyxforge %s %s\n\n%s.\n", c.name, c.args, c.summary)
	return Result{Exit: ExitOK}
}

// flags parses a command's options; folder commands take one optional positional argument.
func flags(env Env, name string, args []string, maxPositional int) (jsonOut bool, pos []string, ok bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	fs.BoolVar(&jsonOut, "json", false, "print machine-readable JSON")
	// Accept the folder before or after the options.
	var rest []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return false, nil, false
		}
		args = fs.Args()
		if len(args) > 0 {
			rest = append(rest, args[0])
			args = args[1:]
		}
	}
	if len(rest) > maxPositional {
		fmt.Fprintf(env.Stderr, "pyxforge %s: unexpected argument %q\n", name, rest[maxPositional])
		return false, nil, false
	}
	return jsonOut, rest, true
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false) // keep compiler messages readable: -> not >
	_ = enc.Encode(v)
}

func runOpen(env Env, args []string) Result {
	if len(args) > 1 {
		fmt.Fprintf(env.Stderr, "pyxforge open: expected one folder, got %d arguments\n", len(args))
		return Result{Exit: ExitUsage}
	}
	dir, err := folderArg(env, args)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge: %v\n", err)
		return Result{Exit: ExitUsage}
	}
	// A file opens its project: the folder holding pyxforge.toml, else its Git checkout, else
	// its own folder.
	if abs, err := filepath.Abs(dir); err == nil {
		if fi, err := os.Stat(abs); err == nil && fi.Mode().IsRegular() {
			w, err := workspace.Discover(filepath.Dir(abs))
			if err != nil {
				fmt.Fprintf(env.Stderr, "pyxforge: %v\n", err)
				return Result{Exit: ExitUsage}
			}
			root := w.Root
			if w.ProjectFile == "" && w.GitRoot != "" {
				root = w.GitRoot
			}
			return Result{OpenGUI: true, Folder: root, Files: []string{abs}}
		}
	}
	w, err := workspace.Discover(dir)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge: %v\nTo run a command instead, see 'pyxforge help'.\n", err)
		return Result{Exit: ExitUsage}
	}
	return Result{OpenGUI: true, Folder: w.Dir}
}

func folderArg(env Env, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	return env.Getwd()
}

func runSetup(env Env, args []string) Result {
	if len(args) == 0 || args[0] != "editor" {
		fmt.Fprintln(env.Stderr, "pyxforge setup: say what to set up: pyxforge setup editor")
		return Result{Exit: ExitUsage}
	}
	fs := flag.NewFlagSet("setup editor", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	noParsers := fs.Bool("no-parsers", false, "install the plugins only")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 0 {
		return Result{Exit: ExitUsage}
	}
	lock, err := nvim.Lock()
	if err == nil {
		var base, init string
		if base, err = nvim.DefaultBase(); err == nil {
			if init, err = nvim.Install(base); err == nil {
				err = neovim.Setup(env.Ctx, neovim.SetupOptions{Config: init, Lock: lock, Parsers: !*noParsers, Out: env.Stdout})
			}
		}
	}
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge setup editor: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	fmt.Fprintln(env.Stdout, "Editor setup complete. PyxForge's Neovim no longer needs the network.")
	return Result{Exit: ExitOK}
}

func runVersion(env Env, args []string) Result {
	jsonOut, _, ok := flags(env, "version", args, 0)
	if !ok {
		return Result{Exit: ExitUsage}
	}
	info := buildinfo.Read()
	if jsonOut {
		writeJSON(env.Stdout, info)
		return Result{Exit: ExitOK}
	}
	rev := ""
	if r := info.ShortRevision(); r != "" {
		rev = ", " + r
		if info.Modified {
			rev += "+modified"
		}
	}
	fmt.Fprintf(env.Stdout, "pyxforge %s (%s, Fyne %s, %s/%s%s)\n", info.Version, info.Go, info.Fyne, info.OS, info.Arch, rev)
	return Result{Exit: ExitOK}
}

type infoJSON struct {
	Folder      string        `json:"folder"`
	ProjectRoot string        `json:"projectRoot"`
	ProjectFile string        `json:"projectFile,omitempty"`
	GitRoot     string        `json:"gitRoot,omitempty"`
	GitBranch   string        `json:"gitBranch,omitempty"`
	Project     *projectJSON  `json:"project,omitempty"`
	Profiles    []profileJSON `json:"profiles,omitempty"`
	Qemu        *config.Qemu  `json:"qemu,omitempty"`
	Gdb         *config.Gdb   `json:"gdb,omitempty"`
	Warnings    []string      `json:"warnings,omitempty"`
	ConfigError string        `json:"configError,omitempty"`
}

type projectJSON struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type profileJSON struct {
	Name      string            `json:"name"`
	Tool      string            `json:"tool"`
	Args      []string          `json:"args,omitempty"`
	SourceDir string            `json:"sourceDir"`
	OutputDir string            `json:"outputDir"`
	DependsOn []string          `json:"dependsOn,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

func runInfo(env Env, args []string) Result {
	jsonOut, pos, ok := flags(env, "info", args, 1)
	if !ok {
		return Result{Exit: ExitUsage}
	}
	dir, err := folderArg(env, pos)
	if err == nil {
		var w workspace.Workspace
		if w, err = workspace.Discover(dir); err == nil {
			return printInfo(env, w, jsonOut)
		}
	}
	fmt.Fprintf(env.Stderr, "pyxforge info: %v\n", err)
	return Result{Exit: ExitUsage}
}

func printInfo(env Env, w workspace.Workspace, jsonOut bool) Result {
	var cfg *config.Config
	var cfgErr error
	if w.ProjectFile != "" {
		cfg, cfgErr = config.Load(w.Root)
	}
	exit := ExitOK
	if cfgErr != nil {
		exit = ExitFailure // an invalid pyxforge.toml is a problem found, for scripts and CI
	}
	if jsonOut {
		out := infoJSON{Folder: w.Dir, ProjectRoot: w.Root, ProjectFile: w.ProjectFile, GitRoot: w.GitRoot, GitBranch: w.Branch}
		if cfgErr != nil {
			out.ConfigError = cfgErr.Error()
		}
		if cfg != nil {
			out.Project = &projectJSON{cfg.Project.Name, cfg.Project.Description}
			for _, n := range cfg.ProfileNames() {
				p := cfg.Profiles[n]
				out.Profiles = append(out.Profiles, profileJSON{p.Name, p.Tool, p.Args, p.SourceDir, p.OutputDir, p.DependsOn, p.Env})
			}
			out.Qemu, out.Gdb, out.Warnings = cfg.Qemu, cfg.Gdb, cfg.Warnings
		}
		writeJSON(env.Stdout, out)
		return Result{Exit: exit}
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "Folder\t%s\n", w.Dir)
	fmt.Fprintf(tw, "Project root\t%s\n", w.Root)
	if w.ProjectFile != "" {
		fmt.Fprintf(tw, "Project file\t%s\n", w.ProjectFile)
	} else {
		fmt.Fprintf(tw, "Project file\tnone (no %s here or in a parent folder)\n", workspace.ProjectFile)
	}
	switch {
	case w.GitRoot == "":
		fmt.Fprintf(tw, "Git\tnot a Git checkout\n")
	case w.Branch == "":
		fmt.Fprintf(tw, "Git\t%s (HEAD unreadable)\n", w.GitRoot)
	default:
		fmt.Fprintf(tw, "Git\t%s on %s\n", w.GitRoot, w.Branch)
	}
	if cfg != nil {
		printConfig(tw, cfg)
	}
	tw.Flush()
	if cfgErr != nil {
		fmt.Fprintf(env.Stderr, "pyxforge info: %v\n", cfgErr)
	}
	return Result{Exit: exit}
}

func printConfig(tw io.Writer, c *config.Config) {
	project := c.Project.Name
	if c.Project.Description != "" {
		project += ": " + c.Project.Description
	}
	fmt.Fprintf(tw, "Project\t%s\n", project)
	if len(c.Profiles) == 0 {
		fmt.Fprintf(tw, "Profiles\tnone\n")
	}
	for i, n := range c.ProfileNames() {
		p := c.Profiles[n]
		label := ""
		if i == 0 {
			label = "Profiles"
		}
		line := fmt.Sprintf("%s: %s", p.Name, strings.TrimSpace(p.Tool+" "+strings.Join(p.Args, " ")))
		if len(p.DependsOn) > 0 {
			line += " (after " + strings.Join(p.DependsOn, ", ") + ")"
		}
		fmt.Fprintf(tw, "%s\t%s\n", label, line)
	}
	if q := c.Qemu; q != nil {
		image := "boots " + q.BootImage
		if q.BootImage == "" {
			image = "kernel " + q.Kernel
		}
		debug := "no GDB stub"
		if q.Debug.Enabled {
			debug = fmt.Sprintf("GDB stub on port %d", q.Debug.GdbPort)
		}
		fmt.Fprintf(tw, "QEMU\t%s, machine %s, %s, %s, %s\n", q.Executable, q.Machine, q.Memory, image, debug)
	}
	if g := c.Gdb; g != nil {
		fmt.Fprintf(tw, "GDB\t%s, architecture %s\n", g.Executable, g.Architecture)
	}
	for _, w := range c.Warnings {
		fmt.Fprintf(tw, "Warning\t%s\n", w)
	}
}

type toolJSON struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Area     string `json:"area"`
	Optional bool   `json:"optional"`
	Found    bool   `json:"found"`
	Path     string `json:"path,omitempty"`
	Version  string `json:"version,omitempty"`
	Error    string `json:"error,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

func runDoctor(env Env, args []string) Result {
	jsonOut, _, ok := flags(env, "doctor", args, 0)
	if !ok {
		return Result{Exit: ExitUsage}
	}
	st := env.Detect(env.Ctx)
	if err := env.Ctx.Err(); err != nil {
		fmt.Fprintln(env.Stderr, "pyxforge doctor: interrupted")
		return Result{Exit: ExitFailure}
	}
	missing := 0
	for _, s := range st {
		if !s.Found() && !s.Tool.Optional {
			missing++
		}
	}
	exit := ExitOK
	if missing > 0 {
		exit = ExitFailure
	}
	if jsonOut {
		out := make([]toolJSON, len(st))
		for i, s := range st {
			out[i] = toolJSON{ID: s.Tool.ID, Label: s.Tool.Label, Area: string(s.Tool.Area), Optional: s.Tool.Optional,
				Found: s.Found(), Path: s.Path, Version: s.Version, Hint: s.Hint()}
			if s.Err != nil {
				out[i].Error = s.Err.Error()
			}
		}
		writeJSON(env.Stdout, struct {
			Tools           []toolJSON `json:"tools"`
			MissingRequired int        `json:"missingRequired"`
		}{out, missing})
		return Result{Exit: exit}
	}

	info := buildinfo.Read()
	fmt.Fprintf(env.Stdout, "PyxForge %s on %s/%s\n", info.Version, info.OS, info.Arch)
	// One set of column widths for every area, so the report reads as one table.
	label := func(s toolchain.Status) string {
		if s.Tool.Optional {
			return s.Tool.Label + " (optional)"
		}
		return s.Tool.Label
	}
	lw, vw := 0, len("unknown")
	for _, s := range st {
		lw, vw = max(lw, len(label(s))), max(vw, len(s.Version))
	}
	for _, area := range toolchain.Areas {
		fmt.Fprintf(env.Stdout, "\n%s\n", area)
		for _, s := range st {
			if s.Tool.Area != area {
				continue
			}
			switch {
			case !s.Found():
				fmt.Fprintf(env.Stdout, "  %-7s  %-*s  %-*s  install: %s\n", "missing", lw, label(s), vw, "", s.Hint())
			case s.Err != nil:
				fmt.Fprintf(env.Stdout, "  %-7s  %-*s  %-*s  %s (%v)\n", "found", lw, label(s), vw, "unknown", s.Path, s.Err)
			default:
				fmt.Fprintf(env.Stdout, "  %-7s  %-*s  %-*s  %s\n", "ok", lw, label(s), vw, s.Version, s.Path)
			}
		}
	}
	found := 0
	for _, s := range st {
		if s.Found() {
			found++
		}
	}
	fmt.Fprintf(env.Stdout, "\n%d of %d tools found", found, len(st))
	if missing > 0 {
		fmt.Fprintf(env.Stdout, "; %d required tool(s) missing.\n", missing)
	} else {
		fmt.Fprintln(env.Stdout, "; every required tool is present.")
	}
	return Result{Exit: exit}
}
