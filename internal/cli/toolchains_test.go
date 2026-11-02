package cli

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/obstinix/PyxForge/internal/managed"
)

// TestToolchains drives pyxforge toolchains against a local server: list shows the package,
// install downloads, checks and unpacks it, verify confirms it, uninstall removes it, and an
// unknown name or action is a usage error.
func TestToolchains(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("tc/bin/cc.exe")
	w.Write([]byte("compiler"))
	zw.Close()
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(archive) }))
	defer srv.Close()

	oldPkgs, oldNew := managed.Packages, newManager
	t.Cleanup(func() { managed.Packages, newManager = oldPkgs, oldNew })
	managed.Packages = []managed.Package{{ID: "tc", Name: "Test toolchain", Version: "1.0", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		URL: srv.URL + "/tc.zip", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(archive)),
		Bin: []string{"tc/bin"}, Requires: []string{"tc/bin/cc.exe"}, Provides: []string{"cc"}, License: "MIT", Source: "here"}}
	root := t.TempDir()
	m := &managed.Manager{Root: filepath.Join(root, "toolchains"), Downloads: filepath.Join(root, "dl")}
	newManager = func() (*managed.Manager, error) { return m, nil }

	list := func() []toolchainJSON {
		t.Helper()
		r := do(t, ".", nil, "toolchains", "--json")
		var rows []toolchainJSON
		if r.res.Exit != 0 || json.Unmarshal([]byte(r.stdout), &rows) != nil || len(rows) != 1 {
			t.Fatalf("list: %+v %q %q", r.res, r.stdout, r.stderr)
		}
		return rows
	}
	if got := list()[0]; got.ID != "tc" || got.State != "Not installed" {
		t.Errorf("before install: %+v", got)
	}
	if r := do(t, ".", nil, "toolchains", "install", "tc"); r.res.Exit != 0 || !strings.Contains(r.stdout, "Installed Test toolchain") {
		t.Fatalf("install: %+v %q %q", r.res, r.stdout, r.stderr)
	}
	if got := list()[0]; got.State != "Installed" || got.Have != "1.0" || got.Dir != m.Dir("tc") {
		t.Errorf("after install: %+v", got)
	}
	if r := do(t, ".", nil, "toolchains", "verify", "tc"); r.res.Exit != 0 || !strings.Contains(r.stdout, "complete") {
		t.Errorf("verify: %+v %q", r.res, r.stderr)
	}
	if r := do(t, ".", nil, "toolchains"); r.res.Exit != 0 || !strings.Contains(r.stdout, "Test toolchain") || !strings.Contains(r.stdout, "Installed") {
		t.Errorf("table: %q", r.stdout)
	}
	if r := do(t, ".", nil, "toolchains", "uninstall", "tc"); r.res.Exit != 0 {
		t.Errorf("uninstall: %+v %q", r.res, r.stderr)
	}
	if got := list()[0]; got.State != "Not installed" {
		t.Errorf("after uninstall: %+v", got)
	}
	if r := do(t, ".", nil, "toolchains", "verify", "tc"); r.res.Exit != ExitFailure {
		t.Errorf("verify of a removed toolchain: %+v", r.res)
	}
	for _, args := range [][]string{{"toolchains", "install", "nope"}, {"toolchains", "install"}, {"toolchains", "frobnicate", "tc"}} {
		if r := do(t, ".", nil, args...); r.res.Exit != ExitUsage {
			t.Errorf("%v: %+v", args, r.res)
		}
	}
}
