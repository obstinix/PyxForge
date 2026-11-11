package main

import (
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// An SPDX 2.3 software bill of materials for the Windows release (#34): what the installer and
// the portable archive contain (PyxForge, the bundled Neovim and Git) and what built them (Go,
// the C compiler cgo used, go-winres, Inno Setup). PyxForge's Go modules come from the build
// information Go records in PyxForge.exe, so the list is what was linked, not what go.mod
// asks for. The document is deterministic: its time is the commit's, not the build's.

type spdxDoc struct {
	SPDXVersion       string        `json:"spdxVersion"`
	DataLicense       string        `json:"dataLicense"`
	SPDXID            string        `json:"SPDXID"`
	Name              string        `json:"name"`
	DocumentNamespace string        `json:"documentNamespace"`
	CreationInfo      spdxCreation  `json:"creationInfo"`
	DocumentDescribes []string      `json:"documentDescribes"`
	Packages          []spdxPackage `json:"packages"`
	Relationships     []spdxRel     `json:"relationships"`
}

type spdxCreation struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
	Comment  string   `json:"comment,omitempty"`
}

type spdxPackage struct {
	SPDXID           string         `json:"SPDXID"`
	Name             string         `json:"name"`
	VersionInfo      string         `json:"versionInfo,omitempty"`
	Supplier         string         `json:"supplier,omitempty"`
	DownloadLocation string         `json:"downloadLocation"`
	FilesAnalyzed    bool           `json:"filesAnalyzed"`
	Checksums        []spdxChecksum `json:"checksums,omitempty"`
	LicenseDeclared  string         `json:"licenseDeclared,omitempty"`
	Purpose          string         `json:"primaryPackagePurpose,omitempty"`
	ExternalRefs     []spdxRef      `json:"externalRefs,omitempty"`
	Comment          string         `json:"comment,omitempty"`
}

type spdxChecksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"checksumValue"`
}

type spdxRef struct {
	Category string `json:"referenceCategory"`
	Type     string `json:"referenceType"`
	Locator  string `json:"referenceLocator"`
}

type spdxRel struct {
	Element string `json:"spdxElementId"`
	Type    string `json:"relationshipType"`
	Related string `json:"relatedSpdxElement"`
}

// sbomInput is what writeSBOM describes.
type sbomInput struct {
	version    string
	commit     string    // the source commit
	committed  time.Time // its commit time
	exe        string    // PyxForge.exe, for its Go build information
	artifacts  []string  // the installer and the portable archive
	programs   []string  // PyxForge.exe and pyxforge.com on their own, without Neovim or Git
	cc         string    // the C compiler's first `--version` line
	isccFolder string    // where ISCC.exe lives, to name the Inno Setup version; may be empty
	// The Visual C++ runtime staged beside Neovim: its version and its files' paths.
	runtimeVersion string
	runtimeFiles   []string
}

var spdxIDChars = regexp.MustCompile(`[^A-Za-z0-9.-]+`)

func spdxID(kind, name string) string {
	return "SPDXRef-" + kind + "-" + strings.Trim(spdxIDChars.ReplaceAllString(name, "-"), "-")
}

func sbom(in sbomInput) (*spdxDoc, error) {
	bi, err := buildinfo.ReadFile(in.exe)
	if err != nil {
		return nil, fmt.Errorf("reading the build information of %s: %w", in.exe, err)
	}
	doc := &spdxDoc{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              "PyxForge " + in.version + " for Windows x64",
		DocumentNamespace: "https://github.com/obstinix/PyxForge/releases/spdx/" + in.version + "-" + in.commit,
		CreationInfo: spdxCreation{
			Created:  in.committed.UTC().Format(time.RFC3339),
			Creators: []string{"Tool: pyxforge-winpkg-" + in.version, "Organization: PyxForge (https://github.com/obstinix/PyxForge)"},
			Comment:  "Written by tools/winpkg from the Go build information in PyxForge.exe and the pinned downloads it bundled. The creation time is the source commit's.",
		},
	}
	add := func(p spdxPackage) string {
		doc.Packages = append(doc.Packages, p)
		return p.SPDXID
	}
	rel := func(a, typ, b string) { doc.Relationships = append(doc.Relationships, spdxRel{a, typ, b}) }

	app := add(spdxPackage{
		SPDXID: "SPDXRef-Package-PyxForge", Name: "PyxForge", VersionInfo: in.version,
		Supplier:         "Organization: PyxForge",
		DownloadLocation: "git+https://github.com/obstinix/PyxForge@" + in.commit,
		LicenseDeclared:  "Apache-2.0", Purpose: "APPLICATION",
		ExternalRefs: []spdxRef{{"PACKAGE-MANAGER", "purl", "pkg:github/obstinix/PyxForge@" + in.commit}},
		Comment:      "PyxForge.exe and pyxforge.com, built with " + bi.GoVersion + " from commit " + in.commit + buildSettings(bi),
	})
	var bundles []string
	installer := ""
	for _, a := range in.artifacts {
		sum, err := fileSum(a)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(a)
		purpose := "ARCHIVE"
		if strings.HasSuffix(name, ".exe") {
			purpose = "INSTALL"
		}
		id := add(spdxPackage{
			SPDXID: spdxID("Package", name), Name: name, VersionInfo: in.version, Supplier: "Organization: PyxForge",
			DownloadLocation: "https://github.com/obstinix/PyxForge/releases/download/v" + in.version + "/" + name,
			Checksums:        []spdxChecksum{{"SHA256", sum}}, LicenseDeclared: "NOASSERTION", Purpose: purpose,
		})
		bundles = append(bundles, id)
		doc.DocumentDescribes = append(doc.DocumentDescribes, id)
		if purpose == "INSTALL" {
			installer = id
		}
	}
	nvim := add(spdxPackage{
		SPDXID: "SPDXRef-Package-Neovim", Name: "Neovim", VersionInfo: nvimVersion, Supplier: "Organization: Neovim",
		DownloadLocation: nvimURL, Checksums: []spdxChecksum{{"SHA256", nvimSHA256}},
		LicenseDeclared: "Apache-2.0 AND Vim", Purpose: "APPLICATION",
		ExternalRefs: []spdxRef{{"PACKAGE-MANAGER", "purl", "pkg:github/neovim/neovim@v" + nvimVersion}},
		Comment:      "The official nvim-win64.zip, unmodified, in the folder nvim.",
	})
	g := bundledGit()
	git := add(spdxPackage{
		SPDXID: "SPDXRef-Package-MinGit", Name: "MinGit (Git for Windows)", VersionInfo: g.Version,
		Supplier: "Organization: Git for Windows", DownloadLocation: g.URL,
		Checksums: []spdxChecksum{{"SHA256", g.SHA256}}, LicenseDeclared: "GPL-2.0-only", Purpose: "APPLICATION",
		Comment: "Unmodified, in the folder git. Source: " + g.Source,
	})
	parts := []string{app, nvim, git}
	if len(in.runtimeFiles) > 0 {
		var sums []spdxChecksum
		for _, f := range in.runtimeFiles {
			sum, err := fileSum(f)
			if err != nil {
				return nil, err
			}
			sums = append(sums, spdxChecksum{"SHA256", sum})
		}
		parts = append(parts, add(spdxPackage{
			SPDXID: "SPDXRef-Package-VC-Runtime", Name: "Microsoft Visual C++ runtime", VersionInfo: in.runtimeVersion,
			Supplier: "Organization: Microsoft Corporation", DownloadLocation: "NOASSERTION",
			Checksums: sums[:1], LicenseDeclared: "NOASSERTION", Purpose: "LIBRARY",
			Comment: "vcruntime140.dll, unmodified, from Visual Studio's Microsoft.VC143.CRT redistributable folder, beside nvim.exe, " +
				"which loads it. Redistributed under the Microsoft Visual Studio license terms for Distributable Code.",
		}))
	}
	for _, b := range bundles {
		for _, part := range parts {
			rel(b, "CONTAINS", part)
		}
	}
	for _, pr := range in.programs {
		sum, err := fileSum(pr)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(pr)
		id := add(spdxPackage{
			SPDXID: spdxID("Package", name), Name: name, VersionInfo: in.version, Supplier: "Organization: PyxForge",
			DownloadLocation: "https://github.com/obstinix/PyxForge/releases/download/v" + in.version + "/" + name,
			Checksums:        []spdxChecksum{{"SHA256", sum}}, LicenseDeclared: "Apache-2.0", Purpose: "APPLICATION",
			Comment: "PyxForge's program on its own; Neovim and Git come with the installer and the archive.",
		})
		doc.DocumentDescribes = append(doc.DocumentDescribes, id)
		rel(id, "CONTAINS", app)
	}

	stdlib := add(spdxPackage{
		SPDXID: "SPDXRef-Package-Go-stdlib", Name: "Go standard library", VersionInfo: bi.GoVersion,
		Supplier: "Organization: The Go Authors", DownloadLocation: "https://go.dev/dl/",
		LicenseDeclared: "BSD-3-Clause", Purpose: "LIBRARY",
		ExternalRefs: []spdxRef{{"PACKAGE-MANAGER", "purl", "pkg:golang/stdlib@" + strings.TrimPrefix(bi.GoVersion, "go")}},
	})
	rel(app, "STATIC_LINK", stdlib)
	deps := modules(bi)
	sort.Slice(deps, func(i, j int) bool { return deps[i].path < deps[j].path })
	for _, m := range deps {
		p := spdxPackage{
			SPDXID: spdxID("Package-Go", m.path+"-"+m.version), Name: m.path, VersionInfo: m.version,
			DownloadLocation: "https://proxy.golang.org/" + escapeModule(m.path) + "/@v/" + m.version + ".zip",
			LicenseDeclared:  "NOASSERTION", Purpose: "LIBRARY",
			ExternalRefs: []spdxRef{{"PACKAGE-MANAGER", "purl", "pkg:golang/" + m.path + "@" + m.version}},
		}
		if m.sum != "" {
			p.Comment = "go.sum " + m.sum
		}
		rel(app, "STATIC_LINK", add(p))
	}

	tools := []spdxPackage{{
		SPDXID: "SPDXRef-Tool-Go", Name: "Go toolchain", VersionInfo: bi.GoVersion, Supplier: "Organization: The Go Authors",
		DownloadLocation: "https://go.dev/dl/", LicenseDeclared: "BSD-3-Clause", Purpose: "APPLICATION",
	}, {
		SPDXID: "SPDXRef-Tool-go-winres", Name: "go-winres", VersionInfo: strings.SplitN(goWinres, "@", 2)[1],
		DownloadLocation: "https://proxy.golang.org/" + escapeModule(strings.SplitN(goWinres, "@", 2)[0]) + "/@v/" + strings.SplitN(goWinres, "@", 2)[1] + ".zip",
		LicenseDeclared:  "NOASSERTION", Purpose: "APPLICATION", Comment: "Writes PyxForge.exe's version resource and icon.",
	}}
	if in.cc != "" {
		tools = append(tools, spdxPackage{
			SPDXID: "SPDXRef-Tool-C-compiler", Name: "C compiler for cgo", VersionInfo: ccVersion(in.cc),
			DownloadLocation: "NOASSERTION", LicenseDeclared: "NOASSERTION", Purpose: "APPLICATION",
			Comment: "gcc --version: " + in.cc,
		})
	}
	for _, t := range tools {
		rel(add(t), "BUILD_TOOL_OF", app)
	}
	if installer != "" && in.isccFolder != "" {
		rel(add(spdxPackage{
			SPDXID: "SPDXRef-Tool-Inno-Setup", Name: "Inno Setup", VersionInfo: innoVersion(in.isccFolder),
			DownloadLocation: "https://jrsoftware.org/isinfo.php", LicenseDeclared: "NOASSERTION", Purpose: "APPLICATION",
			Comment: "Builds the installer.",
		}), "BUILD_TOOL_OF", installer)
	}
	return doc, nil
}

type debugModule struct{ path, version, sum string }

// modules lists the Go modules linked into the program, with replacements applied.
func modules(bi *buildinfo.BuildInfo) []*debugModule {
	var out []*debugModule
	for _, d := range bi.Deps {
		m := d
		if d.Replace != nil {
			m = d.Replace
		}
		out = append(out, &debugModule{m.Path, m.Version, m.Sum})
	}
	return out
}

// buildSettings names the settings that matter for reproducing the build.
func buildSettings(bi *buildinfo.BuildInfo) string {
	var keep []string
	for _, s := range bi.Settings {
		switch s.Key {
		case "-trimpath", "CGO_ENABLED", "GOOS", "GOARCH", "GOAMD64", "vcs.revision", "vcs.modified", "-ldflags":
			keep = append(keep, s.Key+"="+s.Value)
		}
	}
	if len(keep) == 0 {
		return "."
	}
	return " (" + strings.Join(keep, ", ") + ")."
}

// escapeModule applies the module proxy's case encoding: an upper-case letter becomes "!" and
// its lower-case form.
func escapeModule(p string) string {
	var b strings.Builder
	for _, r := range p {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

var versionNumber = regexp.MustCompile(`\d+\.\d+(\.\d+)?`)

// ccVersion picks the version number out of a `gcc --version` line, the last one on the line
// (MinGW builds put their own release name first).
func ccVersion(line string) string {
	all := versionNumber.FindAllString(line, -1)
	if len(all) == 0 {
		return "NOASSERTION"
	}
	return all[len(all)-1]
}

// innoVersion reads the version from the folder Inno Setup was installed to, which the release
// workflow names after it; otherwise NOASSERTION.
func innoVersion(folder string) string {
	if v := versionNumber.FindString(filepath.Base(folder)); v != "" {
		return v
	}
	return "NOASSERTION"
}

// commitInfo returns HEAD and its commit time.
func commitInfo() (string, time.Time, error) {
	out, err := exec.Command("git", "log", "-1", "--format=%H %cI").Output()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("git log: %w", err)
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return "", time.Time{}, fmt.Errorf("git log printed %q", out)
	}
	t, err := time.Parse(time.RFC3339, f[1])
	return f[0], t, err
}

// cCompiler is the first line of `gcc --version`, or empty when there's no gcc on PATH.
func cCompiler() string {
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "gcc"
	}
	out, err := exec.Command(cc, "--version").Output()
	if err != nil {
		return ""
	}
	return firstLine(string(out))
}

func writeSBOM(path string, in sbomInput) error {
	doc, err := sbom(in)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
