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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/managed"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// findCheck finds a check box by its label under o.
func findCheck(o fyne.CanvasObject, label string) *widget.Check {
	switch w := o.(type) {
	case *widget.Check:
		if w.Text == label {
			return w
		}
	case *fyne.Container:
		for _, c := range w.Objects {
			if found := findCheck(c, label); found != nil {
				return found
			}
		}
	}
	return nil
}

// TestSetupWizard walks the first-start wizard as someone new would: it opens after the first
// tool check, the systems profile's tools are listed as found or missing, the missing NASM is
// offered from a toolchain PyxForge manages and really installed (from a local test server),
// and finishing records that setup was done, so it does not open again; Help brings it back.
func TestSetupWizard(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("tc/bin/nasm.exe")
	f.Write([]byte("assembler"))
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
	s := NewWithOptions(test.NewTempApp(t), t.TempDir(), Options{Dispatch: q.post, Toolchains: m, SetupWizard: true})
	noToastExpiry(s)
	t.Cleanup(s.Shutdown)
	tool := func(id string) toolchain.Tool {
		return toolchain.Tools[slices.IndexFunc(toolchain.Tools, func(tl toolchain.Tool) bool { return tl.ID == id })]
	}
	st := []toolchain.Status{{Tool: tool("nasm")}, {Tool: tool("git"), Path: "git", Version: "2.55.0", Source: toolchain.SourcePath}}

	s.applyTools(st, false) // the first check opens the wizard
	w := s.setup
	if w == nil {
		t.Fatal("the first tool check did not open the setup wizard")
	}
	page := func() string { return strings.Join(texts(w.body), "\n") }
	if w.step != 0 || !strings.Contains(w.count.Text, "Step 1 of 5") || !w.back.Disabled() {
		t.Fatalf("not on its first step: %q, back enabled %v", w.count.Text, !w.back.Disabled())
	}
	test.Tap(w.next)
	if w.profile.id != "systems" {
		t.Errorf("default profile %q", w.profile.id)
	}
	test.Tap(w.next) // Your tools
	if p := page(); !strings.Contains(p, "NASM") || !strings.Contains(p, "missing") || !strings.Contains(p, "Git") {
		t.Errorf("the tools step says:\n%s", p)
	}
	test.Tap(w.next) // Install
	c := findCheck(w.body, "Test toolchain 1.0 (2 MB)")
	if c == nil || !c.Checked {
		t.Fatalf("the missing NASM's toolchain is not offered, checked: %q", page())
	}
	install := findButton(w.body, "Install Selected")
	if install == nil || install.Disabled() {
		t.Fatal("no enabled Install Selected")
	}
	test.Tap(install)
	q.pumpUntil(t, "the install", func() bool { return len(s.depInstalls) == 0 && m.Status(pkg).State == managed.Installed })
	if p := page(); !strings.Contains(p, "Test toolchain 1.0 (2 MB) · installed") {
		t.Errorf("after the install the step says:\n%s", p)
	}
	test.Tap(w.next) // First project
	if findButton(w.body, "New Project…") == nil || findButton(w.body, "Open Folder…") == nil || w.next.Text != "Finish" {
		t.Fatalf("the last step offers %q, next is %q", page(), w.next.Text)
	}
	test.Tap(w.next)
	if s.setup != nil || !s.app.Preferences().Bool(prefSetupDone) {
		t.Fatal("Finish did not close the wizard and record it")
	}

	s.applyTools(st, false)
	if s.setup != nil {
		t.Error("the wizard opened again after it was finished")
	}
	if !s.Commands().Run("help.setup") || s.setup == nil {
		t.Error("Help > Setup Wizard… does not open it")
	}
}
