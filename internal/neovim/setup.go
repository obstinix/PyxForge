package neovim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/obstinix/PyxForge/nvim"
)

// SetupOptions configure Setup.
type SetupOptions struct {
	Nvim    string        // the nvim executable; empty searches PATH
	Git     string        // the git executable; empty searches PATH
	Config  string        // PyxForge's init.lua (nvim.Install)
	AppName string        // NVIM_APPNAME; empty means nvim.AppName
	Env     []string      // extra environment
	Lock    nvim.Lockfile // what to install
	Parsers bool          // also build the Tree-sitter parsers
	Out     io.Writer     // progress
}

// Setup installs PyxForge's pinned Neovim plugins into its own data folder and, with Parsers,
// builds the Tree-sitter parsers. It is the explicit, networked first-run step; PyxForge never
// downloads anything on its own at startup. Plugins already at their pinned commit are left
// alone; a plugin at another commit is moved to the pinned one.
func Setup(ctx context.Context, o SetupOptions) error {
	if o.Out == nil {
		o.Out = io.Discard
	}
	nv, err := look(o.Nvim, "nvim")
	if err != nil {
		return err
	}
	git, err := look(o.Git, "git")
	if err != nil {
		return err
	}
	if o.AppName == "" {
		o.AppName = nvim.AppName
	}
	env := append(append(os.Environ(), o.Env...), "NVIM_APPNAME="+o.AppName)

	data, err := headless(ctx, nv, env, `lua io.stdout:write(vim.fn.stdpath("data"))`)
	if err != nil {
		return fmt.Errorf("find Neovim's data folder: %w", err)
	}
	packDir := filepath.Join(data, "site", "pack", "pyxforge", "opt")
	for _, p := range o.Lock.Plugins {
		if err := installPlugin(ctx, git, filepath.Join(packDir, p.Name), p, o.Out); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
	}
	if !o.Parsers || len(o.Lock.Parsers) == 0 {
		return nil
	}

	if _, err := exec.LookPath("tree-sitter"); err != nil {
		return errors.New("the tree-sitter CLI is not on PATH: parsers are built with it (cargo install tree-sitter-cli), then rerun pyxforge setup editor")
	}
	if v, err := headless(ctx, nv, env, `lua io.stdout:write(vim.fn.has("nvim-0.11"))`); err != nil || v != "1" {
		return errors.New("building parsers needs Neovim 0.11 or newer; editing still works with the parsers Neovim ships")
	}
	quoted := make([]string, len(o.Lock.Parsers))
	for i, l := range o.Lock.Parsers {
		quoted[i] = strconv.Quote(l)
	}
	fmt.Fprintf(o.Out, "building parsers: %s\n", strings.Join(o.Lock.Parsers, ", "))
	cmd := exec.CommandContext(ctx, nv, "--headless", "-u", o.Config, "-n",
		"-c", "lua require('pyxforge.setup').parsers({"+strings.Join(quoted, ", ")+"})")
	cmd.Env, cmd.Stdout, cmd.Stderr = env, o.Out, o.Out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build parsers: %w", err)
	}
	return nil
}

func look(path, name string) (string, error) {
	if path != "" {
		return path, nil
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s is not installed (not found on PATH)", name)
	}
	return p, nil
}

// headless runs one Ex command in a clean, headless Neovim and returns what it wrote.
func headless(ctx context.Context, nv string, env []string, command string) (string, error) {
	cmd := exec.CommandContext(ctx, nv, "--headless", "--clean", "-n", "-c", command, "-c", "qall!")
	cmd.Env = env
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func installPlugin(ctx context.Context, git, dir string, p nvim.Plugin, out io.Writer) error {
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, git, args...)
		b, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
		}
		return strings.TrimSpace(string(b)), nil
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		head, err := run("-C", dir, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if head == p.Rev {
			fmt.Fprintf(out, "%s: up to date at %s\n", p.Name, p.Rev[:12])
			return nil
		}
		fmt.Fprintf(out, "%s: moving from %s to %s\n", p.Name, head[:min(12, len(head))], p.Rev[:12])
		if _, err := run("-C", dir, "fetch", "--quiet", "origin", p.Rev); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "%s: cloning %s\n", p.Name, p.URL)
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if _, err := run("clone", "--quiet", "--filter=blob:none", "--no-checkout", p.URL, dir); err != nil {
			return err
		}
	}
	if _, err := run("-C", dir, "-c", "advice.detachedHead=false", "checkout", "--quiet", "--detach", p.Rev); err != nil {
		return err
	}
	head, err := run("-C", dir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != p.Rev {
		return fmt.Errorf("checked out %s, but the lockfile pins %s", head, p.Rev)
	}
	fmt.Fprintf(out, "%s: installed at %s\n", p.Name, p.Rev[:12])
	return nil
}
