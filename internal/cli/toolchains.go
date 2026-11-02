package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/obstinix/PyxForge/internal/managed"
)

// newManager is the toolchain manager the command uses; tests replace it.
var newManager = managed.Default

// runToolchains lists, installs, removes or checks the toolchains PyxForge manages.
func runToolchains(env Env, args []string) Result {
	jsonOut, pos, ok := flags(env, "toolchains", args, 2)
	if !ok {
		return Result{Exit: ExitUsage}
	}
	m, err := newManager()
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge toolchains: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	action := "list"
	if len(pos) > 0 {
		action = pos[0]
	}
	if action == "list" {
		return listToolchains(env, m, jsonOut)
	}
	if len(pos) != 2 {
		fmt.Fprintf(env.Stderr, "pyxforge toolchains %s: name one toolchain (see pyxforge toolchains list)\n", action)
		return Result{Exit: ExitUsage}
	}
	p, found := managed.Find(pos[1])
	if !found {
		fmt.Fprintf(env.Stderr, "pyxforge toolchains: no toolchain %q for this system (see pyxforge toolchains list)\n", pos[1])
		return Result{Exit: ExitUsage}
	}
	switch action {
	case "install", "repair", "update":
		fmt.Fprintf(env.Stdout, "Installing %s %s (%s download)\n", p.Name, p.Version, megabytes(p.Size))
		last, lastPhase := time.Time{}, ""
		err = m.Install(env.Ctx, p, func(pr managed.Progress) {
			if pr.Phase == lastPhase && time.Since(last) < 2*time.Second {
				return
			}
			last, lastPhase = time.Now(), pr.Phase
			if pr.Total > 0 {
				fmt.Fprintf(env.Stdout, "  %s %d%%\n", pr.Phase, pr.Done*100/pr.Total)
			} else {
				fmt.Fprintf(env.Stdout, "  %s\n", pr.Phase)
			}
		})
		if err == nil {
			fmt.Fprintf(env.Stdout, "Installed %s in %s\n", p.Name, m.PackageDir(p))
		}
	case "uninstall":
		if err = m.Uninstall(p); err == nil {
			fmt.Fprintf(env.Stdout, "Removed %s\n", p.Name)
		}
	case "verify":
		if err = m.Verify(p); err == nil {
			fmt.Fprintf(env.Stdout, "%s %s is complete\n", p.Name, p.Version)
		}
	default:
		fmt.Fprintf(env.Stderr, "pyxforge toolchains: unknown action %q: list, install, uninstall or verify\n", action)
		return Result{Exit: ExitUsage}
	}
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge toolchains %s: %v\n", action, err)
		return Result{Exit: ExitFailure}
	}
	return Result{}
}

type toolchainJSON struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	State    string   `json:"state"`
	Have     string   `json:"installedVersion,omitempty"`
	Dir      string   `json:"dir,omitempty"`
	Size     int64    `json:"downloadBytes"`
	Provides []string `json:"provides"`
	Problem  string   `json:"problem,omitempty"`
}

func listToolchains(env Env, m *managed.Manager, jsonOut bool) Result {
	pkgs := managed.ForPlatform()
	var rows []toolchainJSON
	for _, p := range pkgs {
		st := m.Status(p)
		r := toolchainJSON{ID: p.ID, Name: p.Name, Version: p.Version, State: st.State.String(), Size: p.Size, Provides: p.Provides}
		if st.State != managed.NotInstalled {
			r.Have, r.Dir = st.Receipt.Version, m.PackageDir(p)
		}
		if st.Problem != nil {
			r.Problem = st.Problem.Error()
		}
		rows = append(rows, r)
	}
	if jsonOut {
		if rows == nil {
			rows = []toolchainJSON{}
		}
		writeJSON(env.Stdout, rows)
		return Result{}
	}
	if len(rows) == 0 {
		fmt.Fprintln(env.Stdout, "PyxForge installs no toolchains on this system; install them with your package manager (pyxforge doctor --commands).")
		return Result{}
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTOOLCHAIN\tVERSION\tSTATE\tDOWNLOAD")
	for _, r := range rows {
		state := r.State
		if r.Have != "" && r.Have != r.Version {
			state += " (installed " + r.Have + ")"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Name, r.Version, state, megabytes(r.Size))
	}
	tw.Flush()
	fmt.Fprintf(env.Stdout, "\nInstalled in %s. pyxforge toolchains install <id> installs or repairs one.\n", m.Root)
	return Result{}
}

func megabytes(n int64) string { return fmt.Sprintf("%.0f MB", float64(n)/1e6) }
