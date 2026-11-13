package shell

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/managed"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// TestDependencyManager installs a toolchain from the Dependency Manager as a user would: the
// row offers Install with its size, the install runs in the background and the row then offers
// Verify and Uninstall; uninstalling asks first. Doctor offers PyxForge's install for a missing
// tool the toolchain provides.
func TestDependencyManager(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("tc/bin/nasm.exe")
	w.Write([]byte("assembler"))
	zw.Close()
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(archive) }))
	defer srv.Close()
	old := managed.Packages
	t.Cleanup(func() { managed.Packages = old })
	managed.Packages = []managed.Package{{ID: "tc", Name: "Test toolchain", Version: "1.0", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		URL: srv.URL + "/tc.zip", SHA256: hex.EncodeToString(sum[:]), Size: 2_000_000, Bin: []string{"tc/bin"},
		Requires: []string{"tc/bin/nasm.exe"}, Provides: []string{"nasm"}, About: "An assembler for the test", License: "MIT", Source: "here"}}
	pkg := managed.Packages[0]

	dir := t.TempDir()
	m := &managed.Manager{Root: filepath.Join(dir, "toolchains"), Downloads: filepath.Join(dir, "dl"), LogFile: filepath.Join(dir, "log.txt")}
	q := make(queue, 1024)
	s := NewWithOptions(test.NewTempApp(t), t.TempDir(), Options{Dispatch: q.post, Toolchains: m})
	noToastExpiry(s)
	t.Cleanup(s.Shutdown)
	i := slices.IndexFunc(toolchain.Tools, func(tool toolchain.Tool) bool { return tool.ID == "nasm" })
	s.tools = []toolchain.Status{{Tool: toolchain.Tools[i]}} // the last check: NASM missing

	// Doctor points a missing NASM at PyxForge's install.
	s.openDoctor()
	// Its fresh check reads managed.Packages off the UI thread; it must be done before the
	// cleanup puts the real list back (a data race CI's detector found).
	q.pumpUntil(t, "Doctor's check", func() bool { return s.doctor.checks > 0 })
	s.tools = []toolchain.Status{{Tool: toolchain.Tools[i]}} // NASM missing, as the check may have found it
	s.doctor.show(s.tools)
	install := findButton(s.doctor.body, "Install with PyxForge…")
	if install == nil {
		t.Fatalf("Doctor offers no PyxForge install for a missing tool a toolchain provides: %q", texts(s.doctor.body))
	}
	test.Tap(install) // closes Doctor, opens the Dependency Manager
	if s.deps == nil {
		t.Fatal("the Dependency Manager did not open")
	}
	page := strings.Join(texts(s.deps.body), "\n")
	if !strings.Contains(page, "Test toolchain") || !strings.Contains(page, "Not installed") || !strings.Contains(page, "NASM: missing") {
		t.Errorf("the row says:\n%s", page)
	}

	tap := func(label string) {
		t.Helper()
		b := findButton(s.deps.body, label)
		if b == nil {
			t.Fatalf("no %q in the Dependency Manager: %q", label, texts(s.deps.body))
		}
		test.Tap(b)
	}
	tap("Install (2 MB)")
	q.pumpUntil(t, "the install", func() bool { return len(s.depInstalls) == 0 && m.Status(pkg).State == managed.Installed })
	if !logged(s, "Test toolchain installed") {
		t.Errorf("no success notice: %v", s.logLines)
	}
	tap("Verify")
	if !logged(s, "Test toolchain is complete") {
		t.Errorf("verify: %v", s.logLines)
	}

	tap("Uninstall") // asks first
	if m.Status(pkg).State != managed.Installed {
		t.Fatal("uninstalled without asking")
	}
	tapInDialog(t, s, "Uninstall") // the confirmation's button
	if st := m.Status(pkg); st.State != managed.NotInstalled {
		t.Errorf("after uninstall: %v", st.State)
	}
	if findButton(s.deps.body, "Install (2 MB)") == nil {
		t.Errorf("after uninstall the row offers %q", texts(s.deps.body))
	}
}

// TestFirstRunSetup: the first tool check that finds a required tool missing, which PyxForge
// can install, opens the Dependency Manager; later checks, or a missing optional tool, do not.
func TestFirstRunSetup(t *testing.T) {
	old := managed.Packages
	t.Cleanup(func() { managed.Packages = old })
	managed.Packages = []managed.Package{{ID: "tc", Name: "Test toolchain", Version: "1.0", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		Provides: []string{"nasm", "make"}}}
	byID := func(id string) toolchain.Tool {
		return toolchain.Tools[slices.IndexFunc(toolchain.Tools, func(tool toolchain.Tool) bool { return tool.ID == id })]
	}
	s, _ := newTestShell(t)
	t.Cleanup(s.Shutdown)
	s.applyTools([]toolchain.Status{{Tool: byID("make")}}, false) // optional: no prompt
	if s.deps != nil {
		t.Fatal("a missing optional tool opened the Dependency Manager")
	}
	s.applyTools([]toolchain.Status{{Tool: byID("nasm")}}, false)
	if s.deps == nil || !logged(s, "Set up your toolchains") {
		t.Fatal("the first check with NASM missing did not offer the Dependency Manager")
	}
	s.deps.d.Hide()
	s.applyTools([]toolchain.Status{{Tool: byID("nasm")}}, false)
	if s.deps != nil {
		t.Error("the Dependency Manager was offered again")
	}
}
