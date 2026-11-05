// Command depreport lists the dependency updates available to PyxForge, as Markdown for the
// Dependencies workflow's summary (#36): the Go modules go.mod requires, and the actions the
// workflows pin by commit (#32), each against its latest release. It changes nothing and opens
// no pull request; updates are applied, tested and committed by a maintainer.
//
//	go run ./tools/depreport
//
// Modules come from `go list -m -u -json all`; action pins from .github/workflows/*.yml,
// compared with the latest release on GitHub (GH_TOKEN or GITHUB_TOKEN, when set, raises the
// API's rate limit). It exits 0 whatever it finds; known vulnerabilities fail govulncheck, a
// step of their own.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// module is one entry of `go list -m -u -json all`.
type module struct {
	Path     string
	Version  string
	Main     bool
	Indirect bool
	Update   *struct{ Version string }
}

// pin is an action pinned by commit, with the version its comment names.
type pin struct {
	Repo    string // owner/repo
	Version string // such as v5.1.0
	Files   []string
}

func main() {
	mods, err := listModules()
	if err != nil {
		fmt.Fprintln(os.Stderr, "depreport:", err)
		os.Exit(1)
	}
	pins, err := actionPins(filepath.Join(".github", "workflows"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "depreport:", err)
		os.Exit(1)
	}
	gh := &github{base: "https://api.github.com", token: firstEnv("GH_TOKEN", "GITHUB_TOKEN"),
		client: &http.Client{Timeout: 20 * time.Second}}
	n := report(os.Stdout, mods, pins, gh.latest)
	if os.Getenv("GITHUB_ACTIONS") == "true" && n > 0 {
		fmt.Printf("::notice title=Dependency updates::%d updates are available; see the job summary\n", n)
	}
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

func listModules() ([]module, error) {
	cmd := exec.Command("go", "list", "-m", "-u", "-json", "all")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -m -u: %w", err)
	}
	return parseModules(strings.NewReader(string(out)))
}

// parseModules reads the stream of JSON objects `go list -m -json` prints.
func parseModules(r io.Reader) ([]module, error) {
	var mods []module
	dec := json.NewDecoder(r)
	for {
		var m module
		if err := dec.Decode(&m); errors.Is(err, io.EOF) {
			return mods, nil
		} else if err != nil {
			return nil, err
		}
		mods = append(mods, m)
	}
}

var pinLine = regexp.MustCompile(`uses:\s*([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)(?:/[A-Za-z0-9_./-]+)?@[0-9a-f]{40}\s*#\s*(v\d+(?:\.\d+)*)`)

// actionPins reads every pinned action in the workflows in dir.
func actionPins(dir string) ([]pin, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yml"))
	if err != nil {
		return nil, err
	}
	byKey := map[string]*pin{}
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(file)
		for sc.Scan() {
			m := pinLine.FindStringSubmatch(sc.Text())
			if m == nil {
				continue
			}
			key := m[1] + "@" + m[2]
			p := byKey[key]
			if p == nil {
				p = &pin{Repo: m[1], Version: m[2]}
				byKey[key] = p
			}
			if name := filepath.Base(f); !contains(p.Files, name) {
				p.Files = append(p.Files, name)
			}
		}
		file.Close()
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	pins := make([]pin, 0, len(byKey))
	for _, p := range byKey {
		pins = append(pins, *p)
	}
	sort.Slice(pins, func(i, j int) bool { return pins[i].Repo+pins[i].Version < pins[j].Repo+pins[j].Version })
	return pins, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// github asks GitHub for a repository's latest release.
type github struct {
	base, token string
	client      *http.Client
}

func (g *github) latest(repo string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, g.base+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pyxforge-depreport")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// newer reports whether version b is later than a (both such as v1.2.3).
func newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return y > x
		}
	}
	return false
}

func parts(v string) []int {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out []int
	for _, f := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(f)
		out = append(out, n)
	}
	return out
}

// report writes the Markdown report and returns how many updates it lists.
func report(w io.Writer, mods []module, pins []pin, latest func(repo string) (string, error)) int {
	n := 0
	var direct, indirect []module
	for _, m := range mods {
		if m.Main || m.Update == nil {
			continue
		}
		if m.Indirect {
			indirect = append(indirect, m)
		} else {
			direct = append(direct, m)
		}
	}
	fmt.Fprintln(w, "## Dependency updates")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Nothing here is applied automatically: update, run the gate, and commit by hand.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Go modules")
	fmt.Fprintln(w)
	if len(direct)+len(indirect) == 0 {
		fmt.Fprintln(w, "Every module is at its latest version.")
	} else {
		fmt.Fprintln(w, "| Module | Required | Latest | |")
		fmt.Fprintln(w, "|---|---|---|---|")
		for _, group := range []struct {
			mods []module
			kind string
		}{{direct, "direct"}, {indirect, "indirect"}} {
			for _, m := range group.mods {
				fmt.Fprintf(w, "| %s | %s | %s | %s |\n", m.Path, m.Version, m.Update.Version, group.kind)
				n++
			}
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Actions (pinned by commit)")
	fmt.Fprintln(w)
	if len(pins) == 0 {
		fmt.Fprintln(w, "No pinned actions found.")
		return n
	}
	fmt.Fprintln(w, "| Action | Pinned | Latest release | Used in |")
	fmt.Fprintln(w, "|---|---|---|---|")
	checked := map[string]string{}
	for _, p := range pins {
		tag, ok := checked[p.Repo]
		if !ok {
			t, err := latest(p.Repo)
			if err != nil {
				t = "could not check (" + err.Error() + ")"
			}
			tag, checked[p.Repo] = t, t
		}
		state := tag
		if strings.HasPrefix(tag, "v") && newer(p.Version, tag) {
			state = "**" + tag + "**"
			n++
		} else if strings.HasPrefix(tag, "v") {
			state = tag + " (current)"
		}
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n", p.Repo, p.Version, state, strings.Join(p.Files, ", "))
	}
	return n
}
