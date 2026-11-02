// Command linuxpkg packages a Linux build of PyxForge for x86-64:
//
//	go run ./tools/linuxpkg -version 3.0.0 -binary path/to/pyxforge [-out dist/linux]
//
// It writes PyxForge-<version>-linux-x64.deb, a Debian package that installs the program in
// /usr/bin with a desktop entry and icon, and PyxForge-Portable-<version>-linux-x64.tar.gz, the
// same files in one folder to run from anywhere, and SHA256SUMS-linux.txt. Both are built
// without dpkg, byte for byte the same for the same input. The binary is built on Linux (Fyne
// needs cgo); this tool runs anywhere.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// stamp is the time of every file, so the same input gives the same package.
var stamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

const appID = "io.github.obstinix.pyxforge"

var versionRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

func main() {
	version := flag.String("version", "", "the release version, such as 3.0.0 or 3.1.0-rc.1")
	binary := flag.String("binary", "", "the PyxForge program built for linux/amd64")
	out := flag.String("out", filepath.Join("dist", "linux"), "the folder for the results")
	appimage := flag.Bool("appimage", false, "also build PyxForge-<version>-linux-x64.AppImage (on Linux x86-64; downloads the pinned appimagetool and runtime once)")
	flag.Parse()
	if err := run(*version, *binary, *out, *appimage); err != nil {
		fmt.Fprintln(os.Stderr, "linuxpkg:", err)
		os.Exit(1)
	}
}

// file is one file of the package: its path inside it, its mode and its bytes.
type file struct {
	path string
	mode int64
	data []byte
}

func run(version, binary, out string, appimage bool) error {
	if !versionRE.MatchString(version) {
		return fmt.Errorf("-version %q is not a version such as 3.0.0 or 3.1.0-rc.1", version)
	}
	bin, err := os.ReadFile(binary)
	if err != nil {
		return err
	}
	if len(bin) < 4 || string(bin[:4]) != "\x7fELF" {
		return fmt.Errorf("%s is not a Linux program", binary)
	}
	icon, err := os.ReadFile(filepath.Join("internal", "ui", "brand", "icon.png"))
	if err != nil {
		return err
	}
	docs := map[string]string{"LICENSE": "LICENSE", "README.md": "README.md", "INSTALL.md": filepath.Join("docs", "INSTALL.md"),
		"Geist-OFL.txt":         filepath.Join("internal", "ui", "theme", "fonts", "Geist-OFL.txt"),
		"JetBrainsMono-OFL.txt": filepath.Join("internal", "ui", "theme", "fonts", "JetBrainsMono-OFL.txt"),
		"Syne-OFL.txt":          filepath.Join("internal", "ui", "theme", "fonts", "Syne-OFL.txt"),
		"Lucide-LICENSE":        filepath.Join("internal", "ui", "icons", "LICENSE")}
	docBytes := map[string][]byte{}
	for name, src := range docs {
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		docBytes[name] = b
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	// The Debian package.
	files := []file{
		{"usr/bin/pyxforge", 0o755, bin},
		{"usr/share/applications/" + appID + ".desktop", 0o644, []byte(desktopEntry("pyxforge"))},
		{"usr/share/icons/hicolor/256x256/apps/" + appID + ".png", 0o644, icon},
		{"usr/share/doc/pyxforge/copyright", 0o644, []byte(copyright())},
	}
	for _, name := range sortedKeys(docBytes) {
		if name != "LICENSE" {
			files = append(files, file{"usr/share/doc/pyxforge/" + name, 0o644, docBytes[name]})
		}
	}
	deb, err := buildDeb(version, files)
	if err != nil {
		return err
	}
	debName := "PyxForge-" + version + "-linux-x64.deb"

	// The portable archive: one folder.
	top := "PyxForge-" + version
	portable := []file{{top + "/pyxforge", 0o755, bin}, {top + "/" + appID + ".png", 0o644, icon},
		{top + "/pyxforge.desktop", 0o644, []byte(desktopEntry("pyxforge"))}}
	for _, name := range sortedKeys(docBytes) {
		portable = append(portable, file{top + "/" + name, 0o644, docBytes[name]})
	}
	tgz, err := tarGz(portable, false)
	if err != nil {
		return err
	}
	tgzName := "PyxForge-Portable-" + version + "-linux-x64.tar.gz"

	type artifact struct {
		name string
		data []byte
	}
	artifacts := []artifact{{debName, deb}, {tgzName, tgz}}
	for _, a := range artifacts {
		if err := os.WriteFile(filepath.Join(out, a.name), a.data, 0o644); err != nil {
			return err
		}
	}
	if appimage {
		appDir := filepath.Join(out, "PyxForge.AppDir")
		if err := os.RemoveAll(appDir); err != nil {
			return err
		}
		if err := writeAppDir(appDir, files, icon); err != nil {
			return err
		}
		name := "PyxForge-" + version + "-linux-x64.AppImage"
		if err := buildAppImage(appDir, filepath.Join(out, name)); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			return err
		}
		artifacts = append(artifacts, artifact{name, data})
	}
	var sums strings.Builder
	for _, a := range artifacts {
		s := sha256.Sum256(a.data)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(s[:]), a.name)
	}
	if err := os.WriteFile(filepath.Join(out, "SHA256SUMS-linux.txt"), []byte(sums.String()), 0o644); err != nil {
		return err
	}
	fmt.Print(sums.String())
	return nil
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// debVersion is version in Debian's ordering: a pre-release such as 3.1.0-rc.1 becomes
// 3.1.0~rc.1, which sorts before 3.1.0.
func debVersion(v string) string { return strings.Replace(v, "-", "~", 1) }

func desktopEntry(exec string) string {
	return `[Desktop Entry]
Type=Application
Name=PyxForge
GenericName=Operating-system development workbench
Comment=Write assembly, build kernels, boot QEMU and debug with GDB
Exec=` + exec + ` %F
Icon=` + appID + `
Terminal=false
Categories=Development;IDE;
Keywords=assembly;kernel;bootloader;QEMU;GDB;Neovim;
StartupNotify=true
`
}

func copyright() string {
	return `Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: PyxForge
Source: https://github.com/obstinix/PyxForge

Files: *
Copyright: obstinix and the PyxForge contributors
License: Apache-2.0
 On Debian systems the full text is in /usr/share/common-licenses/Apache-2.0.
Comment: The program includes the Geist, JetBrains Mono and Syne fonts (SIL Open
 Font License 1.1) and Lucide icons (ISC); their licenses are in this folder.
`
}

// buildDeb assembles a Debian binary package: an ar archive of debian-binary, control.tar.gz
// and data.tar.gz.
func buildDeb(version string, files []file) ([]byte, error) {
	var size int64
	for _, f := range files {
		size += int64(len(f.data))
	}
	control := fmt.Sprintf(`Package: pyxforge
Version: %s
Architecture: amd64
Maintainer: obstinix <obstinix@gmail.com>
Installed-Size: %d
Depends: libc6, libgl1, libx11-6, libwayland-client0
Recommends: neovim, git
Suggests: qemu-system-x86, qemu-utils, gdb, nasm, gcc, make
Section: devel
Priority: optional
Homepage: https://github.com/obstinix/PyxForge
Description: native workbench for bootloader, kernel and bare-metal work
 PyxForge edits code in an embedded Neovim, builds with the profiles of a
 project's pyxforge.toml, boots images in QEMU and debugs them with GDB, with
 Git, a terminal and a binary inspector in one desktop window.
`, debVersion(version), (size+1023)/1024)
	sums := ""
	for _, f := range files {
		s := md5sum(f.data)
		sums += s + "  " + f.path + "\n"
	}
	controlTar, err := tarGz([]file{{"control", 0o644, []byte(control)}, {"md5sums", 0o644, []byte(sums)}}, true)
	if err != nil {
		return nil, err
	}
	dataTar, err := tarGz(files, true)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("!<arch>\n")
	for _, m := range []struct {
		name string
		data []byte
	}{{"debian-binary", []byte("2.0\n")}, {"control.tar.gz", controlTar}, {"data.tar.gz", dataTar}} {
		fmt.Fprintf(&b, "%-16s%-12d%-6d%-6d%-8s%-10d`\n", m.name, stamp.Unix(), 0, 0, "100644", len(m.data))
		b.Write(m.data)
		if len(m.data)%2 == 1 {
			b.WriteByte('\n')
		}
	}
	return b.Bytes(), nil
}

// tarGz packs files, each folder above them first. With dot, paths start with ./ and files are
// owned by root, as a Debian package's are.
func tarGz(files []file, dot bool) ([]byte, error) {
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	gz.ModTime = stamp
	tw := tar.NewWriter(gz)
	prefix := ""
	if dot {
		prefix = "./"
		if err := tw.WriteHeader(&tar.Header{Name: "./", Mode: 0o755, Typeflag: tar.TypeDir, ModTime: stamp, Format: tar.FormatGNU}); err != nil {
			return nil, err
		}
	}
	dirs := map[string]bool{}
	for _, f := range files {
		var parents []string
		for d := path.Dir(f.path); d != "." && !dirs[d]; d = path.Dir(d) {
			parents = append(parents, d)
			dirs[d] = true
		}
		for i := len(parents) - 1; i >= 0; i-- {
			if err := tw.WriteHeader(&tar.Header{Name: prefix + parents[i] + "/", Mode: 0o755, Typeflag: tar.TypeDir, ModTime: stamp, Format: tar.FormatGNU}); err != nil {
				return nil, err
			}
		}
		if err := tw.WriteHeader(&tar.Header{Name: prefix + f.path, Mode: f.mode, Size: int64(len(f.data)), Typeflag: tar.TypeReg, ModTime: stamp, Format: tar.FormatGNU}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
