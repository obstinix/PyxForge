package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// arMembers reads an ar archive's members in order.
func arMembers(t *testing.T, b []byte) (names []string, data map[string][]byte) {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("!<arch>\n")) {
		t.Fatal("not an ar archive")
	}
	data = map[string][]byte{}
	for off := 8; off < len(b); {
		h := b[off : off+60]
		if string(h[58:60]) != "`\n" {
			t.Fatalf("bad member header at %d", off)
		}
		name := strings.TrimSpace(string(h[:16]))
		size, err := strconv.Atoi(strings.TrimSpace(string(h[48:58])))
		if err != nil {
			t.Fatal(err)
		}
		off += 60
		names = append(names, name)
		data[name] = b[off : off+size]
		off += size + size%2
	}
	return names, data
}

func tarFiles(t *testing.T, b []byte) map[string]*tar.Header {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	out := map[string]*tar.Header{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			body, _ := io.ReadAll(tr)
			h.PAXRecords = map[string]string{"body": string(body)}
		}
		out[h.Name] = h
	}
}

// TestPackages builds the Debian package and the portable archive from a stand-in program: the
// package has the three members dpkg reads, a control file with the Debian version and
// dependencies, the program executable in /usr/bin with its desktop entry and icon; the same
// input gives the same bytes.
func TestPackages(t *testing.T) {
	wd, _ := os.Getwd()
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil { // the repository, for the icon and docs
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
	bin := filepath.Join(t.TempDir(), "pyxforge")
	if err := os.WriteFile(bin, []byte("\x7fELF stand-in"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := run("3.1.0-rc.1", bin, out, false); err != nil {
		t.Fatal(err)
	}
	deb, err := os.ReadFile(filepath.Join(out, "PyxForge-3.1.0-rc.1-linux-x64.deb"))
	if err != nil {
		t.Fatal(err)
	}
	names, members := arMembers(t, deb)
	if strings.Join(names, ",") != "debian-binary,control.tar.gz,data.tar.gz" || string(members["debian-binary"]) != "2.0\n" {
		t.Fatalf("members %v", names)
	}
	control := tarFiles(t, members["control.tar.gz"])["./control"].PAXRecords["body"]
	for _, want := range []string{"Package: pyxforge\n", "Version: 3.1.0~rc.1\n", "Architecture: amd64\n",
		"Depends: libc6, libgl1, libx11-6, libwayland-client0\n", "Recommends: neovim, git\n"} {
		if !strings.Contains(control, want) {
			t.Errorf("control lacks %q:\n%s", want, control)
		}
	}
	data := tarFiles(t, members["data.tar.gz"])
	if h := data["./usr/bin/pyxforge"]; h == nil || h.Mode != 0o755 || h.PAXRecords["body"] != "\x7fELF stand-in" {
		t.Errorf("program: %+v", h)
	}
	for _, p := range []string{"./usr/share/applications/" + appID + ".desktop", "./usr/share/icons/hicolor/256x256/apps/" + appID + ".png",
		"./usr/share/doc/pyxforge/copyright", "./usr/share/doc/pyxforge/Syne-OFL.txt"} {
		if data[p] == nil {
			t.Errorf("the package has no %s", p)
		}
	}
	if d := data["./usr/share/applications/"+appID+".desktop"].PAXRecords["body"]; !strings.Contains(d, "Exec=pyxforge %F") || !strings.Contains(d, "Icon="+appID) {
		t.Errorf("desktop entry:\n%s", d)
	}

	tgz, err := os.ReadFile(filepath.Join(out, "PyxForge-Portable-3.1.0-rc.1-linux-x64.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if h := tarFiles(t, tgz)["PyxForge-3.1.0-rc.1/pyxforge"]; h == nil || h.Mode != 0o755 {
		t.Errorf("portable program: %+v", h)
	}
	sums, _ := os.ReadFile(filepath.Join(out, "SHA256SUMS-linux.txt"))
	if !strings.Contains(string(sums), "PyxForge-3.1.0-rc.1-linux-x64.deb") || !strings.Contains(string(sums), "linux-x64.tar.gz") {
		t.Errorf("sums:\n%s", sums)
	}

	again := t.TempDir()
	if err := run("3.1.0-rc.1", bin, again, false); err != nil {
		t.Fatal(err)
	}
	if deb2, _ := os.ReadFile(filepath.Join(again, "PyxForge-3.1.0-rc.1-linux-x64.deb")); !bytes.Equal(deb, deb2) {
		t.Error("the same input gave another package")
	}
	if err := run("v3", bin, t.TempDir(), false); err == nil {
		t.Error("a bad version was accepted")
	}
	if err := run("3.0.0", filepath.Join(wd, "main.go"), t.TempDir(), false); err == nil || !strings.Contains(err.Error(), "not a Linux program") {
		t.Errorf("a non-ELF binary: %v", err)
	}
}

// TestAppDir: the AppDir has AppRun (executable), the desktop entry and its icon at the top, as
// appimagetool requires, and the program under usr/bin.
func TestAppDir(t *testing.T) {
	dir := t.TempDir()
	files := []file{{"usr/bin/pyxforge", 0o755, []byte("\x7fELF")}}
	if err := writeAppDir(dir, files, []byte("png")); err != nil {
		t.Fatal(err)
	}
	for name, exec := range map[string]bool{"AppRun": true, "pyxforge.desktop": false, appID + ".png": false, ".DirIcon": false, "usr/bin/pyxforge": true} {
		fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("no %s: %v", name, err)
			continue
		}
		if runtime.GOOS != "windows" && (fi.Mode()&0o111 != 0) != exec {
			t.Errorf("%s has mode %v", name, fi.Mode())
		}
	}
	run, _ := os.ReadFile(filepath.Join(dir, "AppRun"))
	if !strings.Contains(string(run), `exec "$HERE/usr/bin/pyxforge" "$@"`) {
		t.Errorf("AppRun:\n%s", run)
	}
}
