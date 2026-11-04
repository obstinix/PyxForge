package toolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRealAppExecLinks reads this machine's App Execution Aliases, when it has any: App
// Installer's Python redirector is a stand-in, anything else is not, and a plain executable is
// no alias at all (#25).
func TestRealAppExecLinks(t *testing.T) {
	if self, err := os.Executable(); err != nil || storeStub(self) {
		t.Fatalf("this test program counted as a stand-in (%v)", err)
	}
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WindowsApps")
	aliases, _ := filepath.Glob(filepath.Join(dir, "*.exe"))
	if len(aliases) == 0 {
		t.Skipf("no App Execution Aliases in %s", dir)
	}
	read := 0
	for _, a := range aliases {
		l, ok := readAppExecLink(a)
		if !ok {
			continue
		}
		read++
		redirector := strings.EqualFold(filepath.Base(l.Target), "AppInstallerPythonRedirector.exe")
		if redirector != storeStub(a) {
			t.Errorf("%s starts %s (%s); stand-in: %v", a, l.Target, l.Package, storeStub(a))
		}
		t.Logf("%s: %s -> %s (stand-in: %v)", filepath.Base(a), l.Package, filepath.Base(l.Target), storeStub(a))
	}
	if read == 0 {
		t.Errorf("none of the %d aliases in %s could be read", len(aliases), dir)
	}
}
