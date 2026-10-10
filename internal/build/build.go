// Package build runs the build profiles of a pyxforge.toml: each profile's dependencies
// first, one tool at a time, stopping at the first failure. Output streams line by line as it
// is produced and is parsed into diagnostics when a step ends. The desktop app and the
// `pyxforge build` command share it.
package build

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// Step is one profile's run.
type Step struct {
	Profile     string        `json:"profile"`
	Tool        string        `json:"tool"`           // as configured
	Path        string        `json:"path,omitempty"` // the executable that ran
	Args        []string      `json:"args"`
	Dir         string        `json:"dir"`  // working folder
	Exit        int           `json:"exit"` // -1 when the tool did not run or was stopped
	Err         string        `json:"error,omitempty"`
	Output      string        `json:"output"` // stdout and stderr, interleaved as produced
	Duration    time.Duration `json:"duration_ns"`
	Diagnostics []Diagnostic  `json:"diagnostics"` // File is absolute
}

// OK reports whether the step ran and exited with status 0.
func (s Step) OK() bool { return s.Err == "" && s.Exit == 0 }

// Result is a whole build.
type Result struct {
	Steps []Step `json:"steps"`
}

// OK reports whether every step succeeded.
func (r Result) OK() bool {
	for _, s := range r.Steps {
		if !s.OK() {
			return false
		}
	}
	return len(r.Steps) > 0
}

// Diagnostics gathers every step's diagnostics.
func (r Result) Diagnostics() []Diagnostic {
	var out []Diagnostic
	for _, s := range r.Steps {
		out = append(out, s.Diagnostics...)
	}
	return out
}

// Options control a build.
type Options struct {
	Root string // the project root, where pyxforge.toml is
	// OnStep runs before each step, with the command line about to run.
	OnStep func(profile string, argv []string)
	// OnLine runs for each line of output as the tool prints it.
	OnLine func(profile, line string)
	// LookPath finds tools; nil means exec.LookPath.
	LookPath func(string) (string, error)
}

// Order returns the profiles that building names involves, each after the profiles it
// depends on and each once. A dependency cycle or an unknown profile is an error.
func Order(c *config.Config, names ...string) ([]string, error) {
	var order []string
	done := map[string]bool{}
	visiting := map[string]bool{}
	var visit func(name string) error
	visit = func(name string) error {
		if visiting[name] {
			//lint:ignore ST1005 the message matches the 2.x core word for word (parity)
			return fmt.Errorf("Circular dependency detected involving '%s'", name)
		}
		if done[name] {
			return nil
		}
		p, err := c.Profile(name)
		if err != nil {
			return err
		}
		visiting[name] = true
		for _, dep := range p.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[name] = false
		done[name] = true
		order = append(order, name)
		return nil
	}
	for _, n := range names {
		if err := visit(n); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// Targets lists the profiles no other profile depends on, in file order: the project's end
// products, which a build without a profile name builds.
func Targets(c *config.Config) []string {
	needed := map[string]bool{}
	for _, p := range c.Profiles {
		for _, d := range p.DependsOn {
			needed[d] = true
		}
	}
	var out []string
	for _, n := range c.ProfileNames() {
		if !needed[n] {
			out = append(out, n)
		}
	}
	return out
}

// Run builds names (Targets when empty). It returns an error only when the build cannot be
// planned; a tool that is missing or fails is reported in its Step, and later steps do not run.
// Cancelling ctx stops the running tool and everything it started.
func Run(ctx context.Context, c *config.Config, names []string, o Options) (Result, error) {
	if len(names) == 0 {
		names = Targets(c)
	}
	if len(names) == 0 {
		return Result{}, errors.New("pyxforge.toml has no build profiles")
	}
	order, err := Order(c, names...)
	if err != nil {
		return Result{}, err
	}
	if o.LookPath == nil {
		o.LookPath = exec.LookPath
	}
	var res Result
	for _, name := range order {
		p, _ := c.Profile(name)
		st := runStep(ctx, name, p, o)
		res.Steps = append(res.Steps, st)
		if !st.OK() {
			break
		}
	}
	return res, nil
}

func runStep(ctx context.Context, name string, p *config.Profile, o Options) Step {
	st := Step{Profile: name, Tool: p.Tool, Args: slices.Clone(p.Args), Exit: -1,
		Dir: filepath.Join(o.Root, filepath.FromSlash(p.SourceDir))}
	if ctx.Err() != nil {
		st.Err = "stopped"
		return st
	}
	if fi, err := os.Stat(st.Dir); err != nil || !fi.IsDir() {
		st.Err = fmt.Sprintf("Profile '%s': source_dir '%s' does not exist (resolved to '%s')", name, p.SourceDir, st.Dir)
		return st
	}
	out := filepath.Join(o.Root, filepath.FromSlash(p.OutputDir))
	if err := os.MkdirAll(out, 0o755); err != nil {
		st.Err = fmt.Sprintf("Profile '%s': failed to create output_dir '%s': %v", name, out, err)
		return st
	}
	path, err := o.LookPath(p.Tool)
	if err != nil {
		st.Err = MissingTool(p.Tool)
		return st
	}
	st.Path = path
	if o.OnStep != nil {
		o.OnStep(name, append([]string{p.Tool}, p.Args...))
	}

	cmd := exec.CommandContext(ctx, path, p.Args...)
	cmd.Dir = st.Dir
	cmd.Env = os.Environ()
	for k, v := range p.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	killTree(cmd)
	proc.Bind(cmd)
	cmd.WaitDelay = 2 * time.Second
	pr, pw, err := os.Pipe()
	if err != nil {
		st.Err = err.Error()
		return st
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	start := time.Now()
	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		st.Err = fmt.Sprintf("Profile '%s': failed to execute '%s': %v", name, p.Tool, err)
		if ctx.Err() != nil {
			st.Err = "stopped"
		}
		return st
	}
	pw.Close() // the child holds its own copy; reading ends when every writer closes
	var all strings.Builder
	read := make(chan struct{})
	go func() {
		defer close(read)
		r := bufio.NewReaderSize(pr, 64*1024)
		for {
			line, err := r.ReadString('\n')
			if line != "" {
				all.WriteString(line)
				if o.OnLine != nil {
					o.OnLine(name, strings.TrimRight(line, "\r\n"))
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					all.WriteString(err.Error())
				}
				return
			}
		}
	}()
	waitErr := cmd.Wait()
	pr.Close() // a grandchild that kept the pipe open must not hold the build
	<-read
	st.Duration = time.Since(start)
	st.Output = all.String()
	if ctx.Err() != nil {
		st.Err = "stopped"
	} else if ee := (*exec.ExitError)(nil); errors.As(waitErr, &ee) {
		st.Exit = ee.ExitCode()
	} else if waitErr != nil && !errors.Is(waitErr, exec.ErrWaitDelay) {
		st.Err = waitErr.Error()
	} else {
		st.Exit = cmd.ProcessState.ExitCode()
	}
	st.Diagnostics = ParseDiagnostics(st.Output)
	for i, d := range st.Diagnostics {
		if !filepath.IsAbs(d.File) {
			st.Diagnostics[i].File = filepath.Join(st.Dir, filepath.FromSlash(d.File))
		}
	}
	return st
}

// MissingTool explains that a profile's tool is not installed, with the install hint from
// the toolchain list when PyxForge knows the tool.
func MissingTool(tool string) string {
	msg := fmt.Sprintf("%s is not installed or not on PATH", tool)
	base := strings.TrimSuffix(filepath.Base(tool), ".exe")
	for _, t := range toolchain.Tools {
		if slices.Contains(t.Candidates, base) {
			if h := (toolchain.Status{Tool: t}).Hint(); h != "" {
				msg += "; install: " + h
			}
			break
		}
	}
	return msg
}
