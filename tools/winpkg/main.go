// Command winpkg builds PyxForge's Windows x64 distribution:
//
//	dist/windows/PyxForge-<version>/       the files the installer installs
//	dist/windows/PyxForge-Setup-<version>.exe     Inno Setup installer, per user, no admin
//	dist/windows/PyxForge-Portable-<version>.zip  the same files to unzip and run
//	dist/windows/SHA256SUMS.txt
//
// The installed folder holds PyxForge.exe (the windowed app: no console), pyxforge.com (the
// same program as a console app, so `pyxforge doctor` prints in a terminal), a pinned and
// checksum-verified Neovim, license texts and notices. Git, QEMU, GDB and compilers are not
// bundled; Tools: Doctor… in the app finds them or lets the user point at them.
//
// It runs on Windows with Go, a C compiler for cgo, Git and Inno Setup 6, and downloads Neovim
// once (into the user cache) and go-winres through the Go module proxy. Every program it builds
// is built twice and must come out byte-identical. A run that fails removes what it wrote.
//
//	go run ./tools/winpkg -version 3.0.0 [-out dist/windows] [-no-installer] [-iscc path]
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Neovim bundled with PyxForge: the official Windows build, pinned by version and SHA-256.
const (
	nvimVersion    = "0.12.6"
	nvimURL        = "https://github.com/neovim/neovim/releases/download/v" + nvimVersion + "/nvim-win64.zip"
	nvimSHA256     = "4e12b7103b601ffc8f25954b4da100f339faa73dccb69a5b135e897fa3d893ed"
	nvimLicenseURL = "https://raw.githubusercontent.com/neovim/neovim/v" + nvimVersion + "/LICENSE.txt"
	nvimLicenseSHA = "de23202ef9a51f5e654034190539739a4abece5505f2ce75e780791106707011"
	goWinres       = "github.com/tc-hib/go-winres@v0.3.3"
)

var versionRE = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// stamp is the modification time of every file in the portable archive, so the same files
// give the same archive.
var stamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func main() {
	version := flag.String("version", "", "the release version, such as 3.0.0 or 3.1.0-rc.1")
	out := flag.String("out", filepath.Join("dist", "windows"), "the folder for the results")
	noInstaller := flag.Bool("no-installer", false, "skip the Inno Setup installer")
	iscc := flag.String("iscc", "", "Inno Setup's ISCC.exe; found in the usual places when empty")
	nvimZip := flag.String("nvim-zip", "", "a local nvim-win64.zip to use instead of downloading it (checked against the pinned SHA-256)")
	flag.Parse()
	if err := run(*version, *out, *iscc, *nvimZip, !*noInstaller); err != nil {
		fmt.Fprintln(os.Stderr, "winpkg:", err)
		os.Exit(1)
	}
}

// numeric turns a semantic version into the four numbers Windows version resources hold.
func numeric(version string) (string, error) {
	m := versionRE.FindStringSubmatch(version)
	if m == nil {
		return "", fmt.Errorf("-version %q is not a version such as 3.0.0 or 3.1.0-rc.1", version)
	}
	return m[1] + "." + m[2] + "." + m[3] + ".0", nil
}

func run(version, out, iscc, nvimZip string, installer bool) (err error) {
	num, err := numeric(version)
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return errors.New("the Windows package is built on Windows: Fyne needs cgo for the target")
	}
	if installer && iscc == "" {
		if iscc, err = findISCC(); err != nil {
			return err
		}
	}
	stageName := "PyxForge-" + version
	stage := filepath.Join(out, stageName)
	example := filepath.Join(out, "example", "boot-sector")
	setup := filepath.Join(out, "PyxForge-Setup-"+version+".exe")
	portable := filepath.Join(out, "PyxForge-Portable-"+version+".zip")
	sums := filepath.Join(out, "SHA256SUMS.txt")
	for _, p := range []string{stage, filepath.Join(out, "example")} {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			for _, p := range []string{stage, filepath.Join(out, "example"), setup, portable, sums} {
				os.RemoveAll(p)
			}
		}
	}()
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}

	step("Version resource and icon")
	syso := filepath.Join("cmd", "pyxforge", "rsrc_windows_amd64.syso")
	if err := runCmd("go", "run", goWinres, "make", "--in", filepath.Join("packaging", "windows", "winres.json"),
		"--arch", "amd64", "--out", filepath.Join("cmd", "pyxforge", "rsrc"), "--product-version", version,
		"--file-version", num); err != nil {
		return fmt.Errorf("go-winres: %w", err)
	}
	defer os.Remove(syso) // only release builds carry it

	step("PyxForge.exe and pyxforge.com, each built twice")
	gui, con := filepath.Join(stage, "PyxForge.exe"), filepath.Join(stage, "pyxforge.com")
	if err := buildTwice(version, gui, true); err != nil {
		return err
	}
	if err := buildTwice(version, con, false); err != nil {
		return err
	}

	step("Neovim " + nvimVersion)
	if err := bundleNeovim(stage, nvimZip); err != nil {
		return err
	}

	step("Documents, licenses and notices")
	for src, dst := range map[string]string{"LICENSE": "LICENSE.txt", "README.md": "README.md",
		filepath.Join("docs", "INSTALL.md"): "INSTALL.md"} {
		if err := copyFile(src, filepath.Join(stage, dst)); err != nil {
			return err
		}
	}
	if err := writeNotices(stage); err != nil {
		return err
	}
	if err := exportExample(example); err != nil {
		return err
	}

	step("Portable archive")
	if err := writeZip(portable, stageName, stage, example); err != nil {
		return err
	}

	if installer {
		step("Installer")
		absStage, _ := filepath.Abs(stage)
		absExample, _ := filepath.Abs(example)
		absOut, _ := filepath.Abs(out)
		if err := runCmd(iscc, "/Q", "/DAppVersion="+version, "/DNumericVersion="+num, "/DStageDir="+absStage,
			"/DExampleDir="+absExample, "/DOutputDir="+absOut, filepath.Join("packaging", "windows", "pyxforge.iss")); err != nil {
			return fmt.Errorf("inno setup: %w", err)
		}
	}

	step("Checks")
	if err := validate(stage, version, portable, stageName); err != nil {
		return err
	}
	artifacts := []string{portable}
	if installer {
		if fi, err := os.Stat(setup); err != nil || fi.Size() < 1<<20 {
			return fmt.Errorf("the installer %s is missing or too small (%v)", setup, err)
		}
		artifacts = append([]string{setup}, artifacts...)
	}
	var lines []string
	for _, a := range artifacts {
		sum, err := fileSum(a)
		if err != nil {
			return err
		}
		lines = append(lines, sum+"  "+filepath.Base(a))
	}
	if err := os.WriteFile(sums, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Println()
	for _, l := range lines {
		fmt.Println(l)
	}
	return nil
}

func step(s string) { fmt.Println("==> " + s) }

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// buildTwice builds cmd/pyxforge into dst twice, with paths, build ID and debug tables left
// out, and fails unless both builds are identical. gui makes a windowed program, which
// Windows starts without a console.
func buildTwice(version, dst string, gui bool) error {
	ldflags := "-s -w -buildid= -X github.com/obstinix/PyxForge/internal/buildinfo.Version=" + version
	if gui {
		ldflags += " -H windowsgui"
	}
	build := func(to string) error {
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", to, "./cmd/pyxforge")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOOS=windows", "GOARCH=amd64")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go build %s: %w", filepath.Base(dst), err)
		}
		return nil
	}
	again := dst + ".check"
	defer os.Remove(again)
	if err := build(dst); err != nil {
		return err
	}
	if err := build(again); err != nil {
		return err
	}
	a, err := fileSum(dst)
	if err != nil {
		return err
	}
	b, err := fileSum(again)
	if err != nil {
		return err
	}
	if a != b {
		return fmt.Errorf("two builds of %s differ (%s, %s): the build is not reproducible here", filepath.Base(dst), a[:12], b[:12])
	}
	return nil
}

// fetch returns a pinned download from the user cache, downloading it first if needed, and
// fails unless its SHA-256 is the pinned one.
func fetch(url, sum, name, local string) ([]byte, error) {
	if local != "" {
		data, err := os.ReadFile(local)
		if err != nil {
			return nil, err
		}
		return checked(data, sum, local)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	cached := filepath.Join(base, "PyxForge", "winpkg", name)
	if data, err := os.ReadFile(cached); err == nil {
		if _, err := checked(data, sum, cached); err == nil {
			return data, nil
		}
	}
	fmt.Println("    downloading", url)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if _, err := checked(data, sum, url); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err == nil {
		_ = os.WriteFile(cached, data, 0o644)
	}
	return data, nil
}

func checked(data []byte, sum, from string) ([]byte, error) {
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != sum {
		return nil, fmt.Errorf("%s: SHA-256 %s, want the pinned %s", from, hex.EncodeToString(got[:]), sum)
	}
	return data, nil
}

// bundleNeovim unpacks the pinned Neovim into stage/nvim, with its license.
func bundleNeovim(stage, local string) error {
	data, err := fetch(nvimURL, nvimSHA256, "nvim-"+nvimVersion+"-win64.zip", local)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	dst := filepath.Join(stage, "nvim")
	for _, f := range zr.File {
		rel, ok := strings.CutPrefix(f.Name, "nvim-win64/")
		if !ok || rel == "" {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return fmt.Errorf("the Neovim archive entry %q leaves its folder", f.Name)
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, b, 0o644); err != nil {
			return err
		}
	}
	lic, err := fetch(nvimLicenseURL, nvimLicenseSHA, "nvim-"+nvimVersion+"-LICENSE.txt", "")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dst, "LICENSE.txt"), lic, 0o644)
}

// writeNotices writes THIRD-PARTY-NOTICES.txt: Neovim, the fonts and icons, and the license
// of every Go module compiled into PyxForge, read from the module cache.
func writeNotices(stage string) error {
	var b strings.Builder
	b.WriteString("PyxForge third-party notices\n\nPyxForge is licensed under the Apache License 2.0 (LICENSE.txt). It includes the\nfollowing software, each under its own license.\n")
	section := func(title, text string) {
		b.WriteString("\n" + strings.Repeat("=", 78) + "\n" + title + "\n" + strings.Repeat("=", 78) + "\n\n")
		b.WriteString(strings.TrimSpace(text) + "\n")
	}
	section("Neovim "+nvimVersion+" (folder nvim)", "The official Windows build of Neovim, unmodified, from "+nvimURL+
		".\nNeovim is licensed under the Apache License 2.0, except for parts contributed under the Vim\n"+
		"license; the full text is in nvim\\LICENSE.txt. The build includes LuaJIT (MIT), tree-sitter\n"+
		"parsers (MIT), win32yank (ISC) and Microsoft's DbgHelp.dll, as Neovim distributes them.")
	for _, f := range []struct{ title, path string }{
		{"Geist font", "internal/ui/theme/fonts/Geist-OFL.txt"},
		{"JetBrains Mono font", "internal/ui/theme/fonts/JetBrainsMono-OFL.txt"},
		{"Syne font", "internal/ui/theme/fonts/Syne-OFL.txt"},
		{"Lucide icons", "internal/ui/icons/LICENSE"},
	} {
		text, err := os.ReadFile(filepath.FromSlash(f.path))
		if err != nil {
			return err
		}
		section(f.title, string(text))
	}
	cmd := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}{{end}}", "./cmd/pyxforge")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOOS=windows", "GOARCH=amd64")
	listed, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go list: %w", err)
	}
	seen := map[string]bool{}
	var mods [][3]string
	for _, line := range strings.Split(strings.TrimSpace(string(listed)), "\n") {
		f := strings.Split(line, "\t")
		if len(f) == 3 && !seen[f[0]] {
			seen[f[0]] = true
			mods = append(mods, [3]string{f[0], f[1], f[2]})
		}
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i][0] < mods[j][0] })
	for _, m := range mods {
		texts, err := licenseTexts(m[2])
		if err != nil || len(texts) == 0 {
			return fmt.Errorf("the Go module %s %s has no license file in %s (%v)", m[0], m[1], m[2], err)
		}
		section("Go module "+m[0]+" "+m[1], strings.Join(texts, "\n\n"))
	}
	return os.WriteFile(filepath.Join(stage, "THIRD-PARTY-NOTICES.txt"), []byte(strings.ReplaceAll(b.String(), "\r\n", "\n")), 0o644)
}

var licenseName = regexp.MustCompile(`(?i)^(LICEN[CS]E|COPYING|NOTICE)([.-].*)?$`)

func licenseTexts(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && licenseName.MatchString(e.Name()) {
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, err
			}
			out = append(out, string(b))
		}
	}
	return out, nil
}

// exportExample writes examples/boot-sector as committed (git archive HEAD), so local edits
// never reach a release.
func exportExample(dst string) error {
	out, err := exec.Command("git", "archive", "--format=tar", "HEAD", "examples/boot-sector").Output()
	if err != nil {
		return fmt.Errorf("git archive examples/boot-sector: %w", err)
	}
	tr := tar.NewReader(bytes.NewReader(out))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel, ok := strings.CutPrefix(h.Name, "examples/boot-sector/")
		if !ok || rel == "" || h.Typeflag != tar.TypeReg {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
}

// writeZip packs stage under top/ and the example under top/examples/, sorted, with one fixed
// time, so the same files always give the same archive.
func writeZip(dst, top, stage, example string) error {
	type entry struct{ name, path string }
	var entries []entry
	add := func(root, prefix string) error {
		return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			entries = append(entries, entry{path.Join(prefix, filepath.ToSlash(rel)), p})
			return nil
		})
	}
	if err := add(stage, top); err != nil {
		return err
	}
	if err := add(example, path.Join(top, "examples", "boot-sector")); err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		data, err := os.ReadFile(e.path)
		if err != nil {
			return err
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Deflate, Modified: stamp})
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644)
}

// required are files the installed folder and the portable archive must hold.
var required = []string{"PyxForge.exe", "pyxforge.com", "LICENSE.txt", "THIRD-PARTY-NOTICES.txt", "README.md",
	"INSTALL.md", "nvim/bin/nvim.exe", "nvim/LICENSE.txt", "nvim/share/nvim/runtime/filetype.lua"}

// validate checks what was built: each program's subsystem and version resource, that the
// console program runs and reports the version, that the bundled Neovim runs, and that the
// portable archive holds every required file.
func validate(stage, version, portable, top string) error {
	for _, f := range required {
		if _, err := os.Stat(filepath.Join(stage, filepath.FromSlash(f))); err != nil {
			return fmt.Errorf("staged folder: %w", err)
		}
	}
	for name, want := range map[string]uint16{"PyxForge.exe": pe.IMAGE_SUBSYSTEM_WINDOWS_GUI, "pyxforge.com": pe.IMAGE_SUBSYSTEM_WINDOWS_CUI} {
		p := filepath.Join(stage, name)
		got, err := subsystem(p)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("%s has subsystem %d, want %d", name, got, want)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !bytes.Contains(data, utf16("ProductVersion")) || !bytes.Contains(data, utf16(version)) {
			return fmt.Errorf("%s has no version resource for %s", name, version)
		}
	}
	out, err := exec.Command(filepath.Join(stage, "pyxforge.com"), "version").CombinedOutput()
	if err != nil || !strings.HasPrefix(string(out), "pyxforge "+version+" ") {
		return fmt.Errorf("pyxforge.com version said %q (%v)", out, err)
	}
	out, err = exec.Command(filepath.Join(stage, "nvim", "bin", "nvim.exe"), "--version").CombinedOutput()
	if err != nil || !strings.HasPrefix(string(out), "NVIM v"+nvimVersion) {
		return fmt.Errorf("the bundled Neovim said %q (%v)", firstLine(string(out)), err)
	}
	zr, err := zip.OpenReader(portable)
	if err != nil {
		return err
	}
	defer zr.Close()
	have := map[string]bool{}
	for _, f := range zr.File {
		have[f.Name] = true
	}
	for _, f := range append(required, "examples/boot-sector/pyxforge.toml", "examples/boot-sector/boot.asm") {
		if !have[top+"/"+f] {
			return fmt.Errorf("the portable archive has no %s", f)
		}
	}
	return nil
}

func subsystem(p string) (uint16, error) {
	f, err := pe.Open(p)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", p, err)
	}
	defer f.Close()
	if oh, ok := f.OptionalHeader.(*pe.OptionalHeader64); ok && f.Machine == pe.IMAGE_FILE_MACHINE_AMD64 {
		return oh.Subsystem, nil
	}
	return 0, fmt.Errorf("%s is not an x86-64 program", p)
}

func utf16(s string) []byte {
	var b []byte
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(l)
}

// findISCC looks for Inno Setup 6's compiler on PATH and where its installer puts it.
func findISCC() (string, error) {
	if p, err := exec.LookPath("ISCC"); err == nil {
		return p, nil
	}
	for _, dir := range []string{os.Getenv("LOCALAPPDATA") + `\Programs\Inno Setup 6`, os.Getenv("ProgramFiles(x86)") + `\Inno Setup 6`,
		os.Getenv("ProgramFiles") + `\Inno Setup 6`} {
		if p := filepath.Join(dir, "ISCC.exe"); fileExists(p) {
			return p, nil
		}
	}
	return "", errors.New("ISCC.exe (Inno Setup 6) was not found: install it (winget install JRSoftware.InnoSetup), pass -iscc, or use -no-installer")
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func fileSum(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
