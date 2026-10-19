package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNumericVersions(t *testing.T) {
	for in, want := range map[string]string{"3.0.0": "3.0.0.0", "3.1.0-rc.1": "3.1.0.0", "10.2.3-dev+1a2b3c4": "10.2.3.0"} {
		if got, err := numeric(in); err != nil || got != want {
			t.Errorf("numeric(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "3.0", "v3.0.0", "3.0.0 x", "../3.0.0", `3.0.0\x`, "3.0.0/x"} {
		if _, err := numeric(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
		if err := run(bad, t.TempDir(), "", "", false); err == nil || !strings.Contains(err.Error(), "is not a version") {
			t.Errorf("run(%q): %v", bad, err)
		}
	}
}

// TestPortableArchive packs a staged folder and an example the way a release does: every file
// under one top folder, the example under examples/, and the same bytes on every run.
func TestPortableArchive(t *testing.T) {
	dir := t.TempDir()
	stage, example := filepath.Join(dir, "stage"), filepath.Join(dir, "example")
	for p, body := range map[string]string{
		filepath.Join(stage, "PyxForge.exe"):                 "gui",
		filepath.Join(stage, "nvim", "bin", "nvim.exe"):      "nvim",
		filepath.Join(stage, "THIRD-PARTY-NOTICES.txt"):      "notices",
		filepath.Join(example, "boot.asm"):                   "org 0x7c00",
		filepath.Join(example, "sub dir ü", "pyxforge.toml"): "[project]",
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, b := filepath.Join(dir, "a.zip"), filepath.Join(dir, "b.zip")
	for _, z := range []string{a, b} {
		if err := writeZip(z, "PyxForge-3.0.0", stage, example); err != nil {
			t.Fatal(err)
		}
	}
	da, _ := os.ReadFile(a)
	db, _ := os.ReadFile(b)
	if !bytes.Equal(da, db) {
		t.Error("two archives of the same files differ")
	}
	zr, err := zip.OpenReader(a)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if !f.Modified.Equal(stamp) {
			t.Errorf("%s has time %v", f.Name, f.Modified)
		}
	}
	want := "PyxForge-3.0.0/PyxForge.exe|PyxForge-3.0.0/THIRD-PARTY-NOTICES.txt|PyxForge-3.0.0/examples/boot-sector/boot.asm|" +
		"PyxForge-3.0.0/examples/boot-sector/sub dir ü/pyxforge.toml|PyxForge-3.0.0/nvim/bin/nvim.exe"
	if got := strings.Join(names, "|"); got != want {
		t.Errorf("entries\n got %s\nwant %s", got, want)
	}
}

// TestSubsystem reads the subsystem of this test program, a console program.
func TestSubsystem(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	got, err := subsystem(exe)
	if err != nil {
		t.Skip("not a Windows x86-64 executable here:", err)
	}
	if got != 3 { // IMAGE_SUBSYSTEM_WINDOWS_CUI
		t.Errorf("subsystem %d, want the console's 3", got)
	}
	if _, err := subsystem(filepath.Join(t.TempDir(), "missing.exe")); err == nil {
		t.Error("a missing file has a subsystem")
	}
}

// TestNoticesCoverEveryModule writes the third-party notices for the real build and checks
// they name Neovim, the fonts, the icons and the Go modules compiled in.
func TestNoticesCoverEveryModule(t *testing.T) {
	t.Chdir("../..")
	dir := t.TempDir()
	if err := writeNotices(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "THIRD-PARTY-NOTICES.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Neovim " + nvimVersion, "Geist font", "JetBrains Mono font", "Syne font", "Lucide icons",
		"Go module fyne.io/fyne/v2", "Go module github.com/neovim/go-client", "Go module golang.org/x/sys"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("the notices do not mention %q", want)
		}
	}
	if strings.Contains(string(data), "Go module github.com/obstinix/PyxForge") {
		t.Error("PyxForge itself is listed as a third party")
	}
}
