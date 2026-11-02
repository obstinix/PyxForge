package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/obstinix/PyxForge/internal/runfile"
	"github.com/obstinix/PyxForge/internal/workspace"
	"github.com/obstinix/PyxForge/nvim"
)

// runClean removes what PyxForge keeps that can be made again or no longer belongs to
// anything (#54): programs Run Current File compiled, the saved layout and editor history of
// workspaces whose folders are gone, and other versions' editor configurations. Workspaces
// that exist, settings and installed toolchains are left alone.
func runClean(env Env, args []string) Result {
	flags := flag.NewFlagSet("clean", flag.ContinueOnError)
	flags.SetOutput(env.Stderr)
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 {
		return Result{Exit: ExitUsage}
	}
	var freed int64
	var errs []error
	report := func(what string, paths []string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", what, err))
		}
		fmt.Fprintf(env.Stdout, "%s: %d removed\n", what, len(paths))
	}

	// Compiled programs: the whole folder, which Run Current File makes again as needed.
	run := runfile.CacheDir()
	size := diskSize(run)
	var runs []string
	entries, err := os.ReadDir(run)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		err = nil
		for _, e := range entries {
			p := filepath.Join(run, e.Name())
			if rerr := os.RemoveAll(p); rerr != nil {
				err = errors.Join(err, rerr)
				continue
			}
			runs = append(runs, p)
		}
	}
	freed += size - diskSize(run)
	report("Compiled programs (Run Current File)", runs, err)

	if st, err := workspace.DefaultStateStore(); err == nil {
		before := diskSize(st.Dir)
		removed, err := st.Prune()
		freed += before - diskSize(st.Dir)
		report("Layouts and editor history of workspaces that no longer exist", removed, err)
	}

	if base, err := nvim.DefaultBase(); err == nil {
		before := diskSize(base)
		removed, err := nvim.Prune(base, 0)
		freed += before - diskSize(base)
		report("Other versions' editor configurations", removed, err)
	}

	fmt.Fprintf(env.Stdout, "Freed %s.\n", humanSize(freed))
	if err := errors.Join(errs...); err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge clean: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	return Result{}
}

// diskSize is the total size of the files under dir; 0 if there is none.
func diskSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

// humanSize prints a byte count the way file managers do.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d bytes", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
