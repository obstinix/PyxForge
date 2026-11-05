package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTrustStore(t *testing.T) {
	st := TrustStore{Path: filepath.Join(t.TempDir(), "cfg", "trusted-folders.json")}
	projects := t.TempDir()
	os2 := filepath.Join(projects, "os")
	kernel := filepath.Join(os2, "kernel")
	other := filepath.Join(t.TempDir(), "other")
	for _, d := range []string{kernel, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if st.Trusted(os2) {
		t.Fatal("a store that doesn't exist yet trusts a folder")
	}
	if err := st.Trust(os2); err != nil {
		t.Fatal(err)
	}
	if !st.Trusted(os2) || !st.Trusted(kernel) {
		t.Error("the trusted folder, or a folder inside it, is not trusted")
	}
	if st.Trusted(projects) || st.Trusted(other) || st.Trusted(os2+"2") || st.Trusted("") {
		t.Error("a folder around, beside or next to the trusted one is trusted")
	}
	if by := st.TrustedBy(kernel); Canonical(by) != Canonical(os2) {
		t.Errorf("TrustedBy = %q, want %q", by, os2)
	}
	if runtime.GOOS == "windows" && !st.Trusted(strings.ToUpper(kernel)) {
		t.Error("on Windows, trust ignores case, as the file system does")
	}
	if err := st.Trust(kernel); err != nil { // inside a trusted folder: a second entry
		t.Fatal(err)
	}
	if err := st.Trust(os2); err != nil { // already there: no duplicate
		t.Fatal(err)
	}
	if list, _ := st.List(); len(list) != 2 {
		t.Errorf("trusted folders %q, want os and os/kernel once each", list)
	}
	if err := st.Revoke(os2); err != nil {
		t.Fatal(err)
	}
	if st.Trusted(os2) || !st.Trusted(kernel) {
		t.Error("revoking os took kernel's own trust, or kept os's")
	}
	if err := st.Revoke(kernel); err != nil {
		t.Fatal(err)
	}
	if list, err := st.List(); err != nil || len(list) != 0 || st.Trusted(kernel) {
		t.Errorf("after revoking both: %q, %v", list, err)
	}
	// Persisted, and a store that can't be read trusts nothing.
	if err := st.Trust(other); err != nil {
		t.Fatal(err)
	}
	if !(TrustStore{Path: st.Path}).Trusted(other) {
		t.Error("trust was not kept in the file")
	}
	if err := os.WriteFile(st.Path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st.Trusted(other) {
		t.Error("an unreadable store trusts a folder")
	}
	if (TrustStore{}).Trusted(other) {
		t.Error("a store without a file trusts a folder")
	}
}
