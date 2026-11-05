package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goList = `{"Path": "github.com/obstinix/PyxForge", "Main": true}
{"Path": "fyne.io/fyne/v2", "Version": "v2.8.1", "Update": {"Path": "fyne.io/fyne/v2", "Version": "v2.8.2"}}
{"Path": "github.com/BurntSushi/toml", "Version": "v1.6.0"}
{"Path": "golang.org/x/image", "Version": "v0.47.0", "Indirect": true, "Update": {"Path": "golang.org/x/image", "Version": "v0.48.0"}}
`

func TestReportListsUpdates(t *testing.T) {
	mods, err := parseModules(strings.NewReader(goList))
	if err != nil {
		t.Fatal(err)
	}
	pins := []pin{
		{Repo: "actions/checkout", Version: "v5.1.0", Files: []string{"ci.yml"}},
		{Repo: "actions/setup-go", Version: "v6.5.0", Files: []string{"ci.yml", "fuzz.yml"}},
		{Repo: "gone/action", Version: "v1.0.0", Files: []string{"ci.yml"}},
	}
	latest := func(repo string) (string, error) {
		switch repo {
		case "actions/checkout":
			return "v7.0.1", nil
		case "actions/setup-go":
			return "v6.5.0", nil
		}
		return "", errors.New("GitHub answered 404 Not Found")
	}
	var b strings.Builder
	n := report(&b, mods, pins, latest)
	out := b.String()
	if n != 3 { // fyne, x/image, checkout
		t.Errorf("%d updates, want 3:\n%s", n, out)
	}
	for _, want := range []string{
		"| fyne.io/fyne/v2 | v2.8.1 | v2.8.2 | direct |",
		"| golang.org/x/image | v0.47.0 | v0.48.0 | indirect |",
		"| actions/checkout | v5.1.0 | **v7.0.1** | ci.yml |",
		"| actions/setup-go | v6.5.0 | v6.5.0 (current) | ci.yml, fuzz.yml |",
		"| gone/action | v1.0.0 | could not check (GitHub answered 404 Not Found) | ci.yml |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "BurntSushi") || strings.Contains(out, "obstinix/PyxForge") {
		t.Errorf("up-to-date or main modules are listed:\n%s", out)
	}
}

func TestActionPins(t *testing.T) {
	dir := t.TempDir()
	wf := "steps:\n  - uses: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09 # v5.1.0\n" +
		"  - uses: github/codeql-action/init@0123456789abcdef0123456789abcdef01234567 # v3.30.1\n" +
		"  - uses: actions/checkout@v5\n" + // not pinned: workflowcheck refuses it; not reported
		"  - uses: ./.github/actions/local\n"
	if err := os.WriteFile(filepath.Join(dir, "a.yml"), []byte(wf), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yml"), []byte("- uses: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09 # v5.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pins, err := actionPins(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 2 || pins[0].Repo != "actions/checkout" || strings.Join(pins[0].Files, ",") != "a.yml,b.yml" ||
		pins[1].Repo != "github/codeql-action" || pins[1].Version != "v3.30.1" {
		t.Errorf("pins: %+v", pins)
	}
}

func TestLatestRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/actions/checkout/releases/latest" && r.Header.Get("Authorization") == "Bearer secret" {
			_, _ = w.Write([]byte(`{"tag_name": "v7.0.1"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := &github{base: srv.URL, token: "secret", client: srv.Client()}
	if tag, err := g.latest("actions/checkout"); err != nil || tag != "v7.0.1" {
		t.Errorf("latest = %q, %v", tag, err)
	}
	if _, err := g.latest("nobody/nothing"); err == nil {
		t.Error("a missing repository gave no error")
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"v5.1.0", "v7.0.1", true}, {"v6.5.0", "v6.5.0", false}, {"v4.6.2", "v4.6.10", true}, {"v2.9.3", "v2", false}, {"v1.2", "v1.2.1", true}} {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%s, %s) = %v", c.a, c.b, got)
		}
	}
}
