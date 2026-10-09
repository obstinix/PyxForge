package cli

import (
	"flag"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/obstinix/PyxForge/internal/build"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// runBuild runs pyxforge.toml build profiles from the project around the working folder.
func runBuild(env Env, args []string) Result {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	jsonOut := fs.Bool("json", false, "print the result as JSON instead of streaming output")
	list := fs.Bool("list", false, "list the profiles instead of building")
	dir := fs.String("C", "", "run in this folder instead of the working folder")
	var names []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return Result{Exit: ExitUsage}
		}
		args = fs.Args()
		if len(args) > 0 {
			names = append(names, args[0])
			args = args[1:]
		}
	}

	folder := *dir
	if folder == "" {
		wd, err := env.Getwd()
		if err != nil {
			fmt.Fprintf(env.Stderr, "pyxforge build: %v\n", err)
			return Result{Exit: ExitFailure}
		}
		folder = wd
	}
	w, err := workspace.Discover(folder)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge build: %v\n", err)
		return Result{Exit: ExitUsage}
	}
	if w.ProjectFile == "" {
		fmt.Fprintf(env.Stderr, "pyxforge build: no %s in %s or its parents\n", config.FileName, w.Dir)
		return Result{Exit: ExitFailure}
	}
	c, err := config.Load(w.Root)
	if err == nil {
		err = c.Validate()
	}
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge build: %s: %v\n", w.ProjectFile, err)
		return Result{Exit: ExitFailure}
	}
	for _, warn := range c.Warnings {
		fmt.Fprintf(env.Stderr, "pyxforge build: warning: %s\n", warn)
	}

	if *list {
		return listProfiles(env, c, *jsonOut)
	}

	order, err := build.Order(c, orTargets(c, names)...)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge build: %v\n", err)
		return Result{Exit: ExitUsage}
	}
	opts := build.Options{Root: w.Root}
	if !*jsonOut {
		fmt.Fprintf(env.Stdout, "Building %s: %s\n", c.Project.Name, strings.Join(order, ", "))
		opts.OnStep = func(p string, argv []string) {
			fmt.Fprintf(env.Stdout, "\n==> %s: %s\n", p, strings.Join(argv, " "))
		}
		opts.OnLine = func(_, line string) { fmt.Fprintln(env.Stdout, line) }
	}
	res, err := build.Run(env.Ctx, c, names, opts)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge build: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	exit := ExitOK
	if !res.OK() {
		exit = ExitFailure
	}
	if *jsonOut {
		writeJSON(env.Stdout, struct {
			Project string `json:"project"`
			OK      bool   `json:"ok"`
			build.Result
		}{c.Project.Name, res.OK(), res})
		return Result{Exit: exit}
	}
	for _, st := range res.Steps {
		took := st.Duration.Round(10 * time.Millisecond)
		switch {
		case st.Err != "":
			fmt.Fprintf(env.Stdout, "FAILED %s: %s\n", st.Profile, st.Err)
		case st.Exit != 0:
			fmt.Fprintf(env.Stdout, "FAILED %s: exit status %d (%v)\n", st.Profile, st.Exit, took)
		default:
			fmt.Fprintf(env.Stdout, "ok     %s (%v)\n", st.Profile, took)
		}
	}
	errs, warns := 0, 0
	for _, d := range res.Diagnostics() {
		switch d.Severity {
		case "error":
			errs++
		case "warning":
			warns++
		}
	}
	verdict := "Build succeeded"
	if exit != ExitOK {
		verdict = "Build failed"
	}
	fmt.Fprintf(env.Stdout, "\n%s: %s, %s.\n", verdict, plural(errs, "error"), plural(warns, "warning"))
	return Result{Exit: exit}
}

func orTargets(c *config.Config, names []string) []string {
	if len(names) == 0 {
		return build.Targets(c)
	}
	return names
}

func listProfiles(env Env, c *config.Config, jsonOut bool) Result {
	type profileJSON struct {
		Name        string   `json:"name"`
		Tool        string   `json:"tool"`
		Description string   `json:"description,omitempty"`
		DependsOn   []string `json:"depends_on"`
		Target      bool     `json:"target"`
	}
	targets := build.Targets(c)
	var out []profileJSON
	for _, n := range c.ProfileNames() {
		p := c.Profiles[n]
		deps := p.DependsOn
		if deps == nil {
			deps = []string{}
		}
		out = append(out, profileJSON{n, p.Tool, p.Description, deps, slices.Contains(targets, n)})
	}
	if jsonOut {
		writeJSON(env.Stdout, out)
		return Result{Exit: ExitOK}
	}
	for _, p := range out {
		line := fmt.Sprintf("%-16s %s", p.Name, p.Tool)
		if len(p.DependsOn) > 0 {
			line += "  (after " + strings.Join(p.DependsOn, ", ") + ")"
		}
		if p.Target {
			line += "  [default]"
		}
		if p.Description != "" {
			line += "  " + p.Description
		}
		fmt.Fprintln(env.Stdout, strings.TrimRight(line, " "))
	}
	return Result{Exit: ExitOK}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
