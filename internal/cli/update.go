package cli

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/managed"
	"github.com/obstinix/PyxForge/internal/update"
)

// runUpdate checks GitHub Releases for a newer PyxForge and, with --download, fetches its
// installer or package and checks it. It never runs what it downloads.
func runUpdate(env Env, args []string) Result {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	download := fs.Bool("download", false, "download the update's installer or package and check it against the release's SHA256SUMS")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		return Result{Exit: ExitUsage}
	}
	info := buildinfo.Read()
	fmt.Fprintf(env.Stdout, "This is PyxForge %s.\n", info.Version)
	rel, ok, err := update.Check(env.Ctx, nil, update.Current{Version: info.Version, Built: info.Time})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge update: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	if !ok {
		fmt.Fprintln(env.Stdout, "No newer release: this is the latest.")
		return Result{}
	}
	fmt.Fprintf(env.Stdout, "PyxForge %s is available (%s, published %s).\n", rel.Version(), rel.Page, rel.Published.Format("2 January 2006"))
	if notes := strings.TrimSpace(rel.Notes); notes != "" {
		fmt.Fprintf(env.Stdout, "\n%s\n\n", notes)
	}
	asset, has := update.Installer(rel)
	if !has {
		fmt.Fprintln(env.Stdout, "The release has no installer for this system; see its page.")
		return Result{}
	}
	if !*download {
		fmt.Fprintf(env.Stdout, "pyxforge update --download fetches %s and checks it.\n", asset.Name)
		return Result{}
	}
	dir, err := managed.DataDir()
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge update: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	path, err := update.Download(env.Ctx, nil, rel, asset, filepath.Join(dir, "cache", "updates"), nil)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge update: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	fmt.Fprintf(env.Stdout, "Downloaded and checked against the release's SHA-256: %s\nRun it to update; your settings and projects are kept.\n", path)
	return Result{}
}
