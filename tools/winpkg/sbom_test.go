package main

import (
	"bytes"
	"debug/buildinfo"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testSBOMInput(t *testing.T) sbomInput {
	t.Helper()
	exe, err := os.Executable() // a test binary carries Go build information too
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	setup := filepath.Join(dir, "PyxForge-Setup-1.2.3.exe")
	zip := filepath.Join(dir, "PyxForge-Portable-1.2.3.zip")
	if err := os.WriteFile(setup, []byte("installer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zip, []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	return sbomInput{
		version: "1.2.3", commit: "0123456789abcdef0123456789abcdef01234567",
		committed: time.Date(2026, 11, 5, 10, 30, 0, 0, time.FixedZone("IST", 5*3600+1800)),
		exe:       exe, artifacts: []string{setup, zip},
		cc:         "gcc.exe (MinGW-W64 x86_64-ucrt-posix-seh, built by Brecht Sanders, r3) 16.2.0",
		isccFolder: filepath.Join(dir, "InnoSetup-6.7.3"),
	}
}

func TestSBOMDescribesTheReleaseFiles(t *testing.T) {
	in := testSBOMInput(t)
	doc, err := sbom(in)
	if err != nil {
		t.Fatal(err)
	}
	if doc.SPDXVersion != "SPDX-2.3" || doc.DataLicense != "CC0-1.0" || doc.SPDXID != "SPDXRef-DOCUMENT" {
		t.Errorf("document header: %+v", doc)
	}
	if doc.CreationInfo.Created != "2026-11-05T05:00:00Z" {
		t.Errorf("created %q, want the commit time in UTC", doc.CreationInfo.Created)
	}
	ids := map[string]spdxPackage{}
	for _, p := range doc.Packages {
		if _, dup := ids[p.SPDXID]; dup {
			t.Errorf("SPDX ID %s is used twice", p.SPDXID)
		}
		if p.DownloadLocation == "" {
			t.Errorf("%s has no downloadLocation, which SPDX requires", p.SPDXID)
		}
		ids[p.SPDXID] = p
	}
	for _, r := range doc.Relationships {
		if _, ok := ids[r.Element]; !ok {
			t.Errorf("relationship from unknown element %s", r.Element)
		}
		if _, ok := ids[r.Related]; !ok {
			t.Errorf("relationship to unknown element %s", r.Related)
		}
	}
	if len(doc.DocumentDescribes) != 2 {
		t.Fatalf("the document describes %v, want the installer and the archive", doc.DocumentDescribes)
	}
	for i, a := range in.artifacts {
		p := ids[doc.DocumentDescribes[i]]
		want, _ := fileSum(a)
		if p.Name != filepath.Base(a) || len(p.Checksums) != 1 || p.Checksums[0].Value != want {
			t.Errorf("%s: %+v, want SHA-256 %s", filepath.Base(a), p, want)
		}
	}
	nvim := ids["SPDXRef-Package-Neovim"]
	if nvim.VersionInfo != nvimVersion || nvim.Checksums[0].Value != nvimSHA256 || nvim.DownloadLocation != nvimURL {
		t.Errorf("Neovim: %+v", nvim)
	}
	if g := ids["SPDXRef-Package-MinGit"]; g.VersionInfo != bundledGit().Version || g.Checksums[0].Value != bundledGit().SHA256 {
		t.Errorf("MinGit: %+v", g)
	}
	if std := ids["SPDXRef-Package-Go-stdlib"]; std.VersionInfo != runtime.Version() {
		t.Errorf("Go standard library %q, want %q", std.VersionInfo, runtime.Version())
	}
	if cc := ids["SPDXRef-Tool-C-compiler"]; cc.VersionInfo != "16.2.0" || !strings.Contains(cc.Comment, "Brecht Sanders") {
		t.Errorf("C compiler: %+v", cc)
	}
	if inno := ids["SPDXRef-Tool-Inno-Setup"]; inno.VersionInfo != "6.7.3" {
		t.Errorf("Inno Setup: %+v", inno)
	}
	contains := 0
	for _, r := range doc.Relationships {
		if r.Type == "CONTAINS" && (r.Related == "SPDXRef-Package-PyxForge" || r.Related == "SPDXRef-Package-Neovim" || r.Related == "SPDXRef-Package-MinGit") {
			contains++
		}
	}
	if contains != 6 {
		t.Errorf("%d CONTAINS relationships for PyxForge, Neovim and MinGit, want 3 per release file", contains)
	}

	// Every Go module linked into the program is listed.
	bi, err := buildinfo.ReadFile(in.exe)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]string{}
	for _, p := range doc.Packages {
		if strings.HasPrefix(p.SPDXID, "SPDXRef-Package-Go-") && p.SPDXID != "SPDXRef-Package-Go-stdlib" {
			listed[p.Name] = p.VersionInfo
		}
	}
	for _, m := range modules(bi) {
		if listed[m.path] != m.version {
			t.Errorf("module %s %s is listed as %q", m.path, m.version, listed[m.path])
		}
	}
}

func TestSBOMIsDeterministic(t *testing.T) {
	in := testSBOMInput(t)
	a, b := filepath.Join(t.TempDir(), "a.json"), filepath.Join(t.TempDir(), "b.json")
	if err := writeSBOM(a, in); err != nil {
		t.Fatal(err)
	}
	if err := writeSBOM(b, in); err != nil {
		t.Fatal(err)
	}
	da, _ := os.ReadFile(a)
	db, _ := os.ReadFile(b)
	if !bytes.Equal(da, db) {
		t.Error("two SBOMs of the same build differ")
	}
	var anything map[string]any
	if err := json.Unmarshal(da, &anything); err != nil {
		t.Fatalf("the SBOM is not JSON: %v", err)
	}
}

func TestSBOMHelpers(t *testing.T) {
	if got := escapeModule("github.com/BurntSushi/toml"); got != "github.com/!burnt!sushi/toml" {
		t.Errorf("escapeModule = %q", got)
	}
	if got := ccVersion("gcc (GCC) 13.2.0"); got != "13.2.0" {
		t.Errorf("ccVersion = %q", got)
	}
	if got := ccVersion("no numbers"); got != "NOASSERTION" {
		t.Errorf("ccVersion without a version = %q", got)
	}
	if got := innoVersion(`C:\Program Files (x86)\Inno Setup 6`); got != "NOASSERTION" {
		t.Errorf("innoVersion of a folder without a full version = %q", got)
	}
	if got := spdxID("Package", "PyxForge-Setup-3.0.0 rc+1.exe"); got != "SPDXRef-Package-PyxForge-Setup-3.0.0-rc-1.exe" {
		t.Errorf("spdxID = %q", got)
	}
}
