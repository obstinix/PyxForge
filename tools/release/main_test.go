package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVersions(t *testing.T) {
	for _, ok := range []string{"3.0.0", "3.1.0-rc.1", "3.0.0-dev+1a2b3c4", "10.20.30"} {
		if !versionRE.MatchString(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "3.0", "v3.0.0", "3.0.0 -X main.x=y", "../../3.0.0", "3.0.0/x", `3.0.0\x`} {
		if err := release(bad, t.TempDir(), false); err == nil || !strings.Contains(err.Error(), "is not a version") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

// TestPackAndCheck packs a stand-in program with the documents, reads the archive back, and
// checks that a changed program is caught.
func TestPackAndCheck(t *testing.T) {
	t.Chdir("../..") // the documents are named from the repository root
	out := t.TempDir()
	exe := "pyxforge"
	if runtime.GOOS == "windows" {
		exe = "pyxforge.exe"
	}
	bin := filepath.Join(out, "program")
	if err := os.WriteFile(bin, []byte("the program"), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("the program"))
	name := "pyxforge-3.0.0-test"
	archive, err := pack(out, name, exe, bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkArchive(archive, name+"/"+exe, hex.EncodeToString(sum[:])); err != nil {
		t.Errorf("the archive does not hold the program: %v", err)
	}
	first, _ := os.ReadFile(archive)
	if again, err := pack(out, name, exe, bin); err != nil {
		t.Fatal(err)
	} else if second, _ := os.ReadFile(again); string(first) != string(second) {
		t.Error("packing twice gave different archives")
	}
	other := sha256.Sum256([]byte("another program"))
	if err := checkArchive(archive, name+"/"+exe, hex.EncodeToString(other[:])); err == nil {
		t.Error("a different program was accepted")
	}
	if err := checkArchive(archive, name+"/missing", hex.EncodeToString(sum[:])); err == nil {
		t.Error("a missing entry was accepted")
	}
}
