// Command release builds a PyxForge release for the platform it runs on: the binary, an
// archive with the binary and its documents, and SHA256SUMS. It builds with -trimpath and an
// empty build ID, so the same commit, Go and C compiler give the same bytes; it checks that by
// building twice. Fyne needs cgo, so each platform is built on that platform.
//
//	go run ./tools/release [-version 3.0.0] [-out dist]
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/obstinix/PyxForge/internal/buildinfo"
)

// docs go into the archive beside the binary.
var docs = []string{"README.md", "LICENSE", "docs/INSTALL.md"}

func main() {
	version := flag.String("version", buildinfo.Version, "the release version stamped into the binary")
	out := flag.String("out", "dist", "the folder for the results")
	once := flag.Bool("no-verify", false, "build once, without checking that a second build matches")
	flag.Parse()
	if err := release(*version, *out, !*once); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func release(version, out string, verify bool) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("pyxforge-%s-%s-%s", version, runtime.GOOS, runtime.GOARCH)
	exe, ext := "pyxforge", ""
	if runtime.GOOS == "windows" {
		exe, ext = "pyxforge.exe", ".exe"
	}
	bin := filepath.Join(out, name+ext)
	if err := build(version, bin); err != nil {
		return err
	}
	sum, err := fileSum(bin)
	if err != nil {
		return err
	}
	if verify {
		again := bin + ".check"
		if err := build(version, again); err != nil {
			return err
		}
		sum2, err := fileSum(again)
		os.Remove(again)
		if err != nil {
			return err
		}
		if sum2 != sum {
			return fmt.Errorf("two builds of the same source differ (%s, %s): the build is not reproducible here", sum[:12], sum2[:12])
		}
		fmt.Println("reproducible: a second build has the same SHA-256")
	}
	archive, err := pack(out, name, exe, bin)
	if err != nil {
		return err
	}
	asum, err := fileSum(archive)
	if err != nil {
		return err
	}
	sums := fmt.Sprintf("%s  %s\n%s  %s\n", sum, filepath.Base(bin), asum, filepath.Base(archive))
	if err := os.WriteFile(filepath.Join(out, name+".SHA256SUMS"), []byte(sums), 0o644); err != nil {
		return err
	}
	fmt.Print(sums)
	return nil
}

// build compiles cmd/pyxforge with paths, build ID and debug tables left out of the binary.
func build(version, dst string) error {
	ldflags := "-s -w -buildid= -X github.com/obstinix/PyxForge/internal/buildinfo.Version=" + version
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", dst, "./cmd/pyxforge")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	return nil
}

func fileSum(path string) (string, error) {
	f, err := os.Open(path)
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

// pack writes the archive: a zip on Windows, a .tar.gz elsewhere, with every entry under a
// folder named after the release and a fixed modification time.
func pack(out, name, exe, bin string) (string, error) {
	files := map[string]string{exe: bin}
	for _, d := range docs {
		files[filepath.Base(d)] = d
	}
	order := append([]string{exe}, func() []string {
		var n []string
		for _, d := range docs {
			n = append(n, filepath.Base(d))
		}
		return n
	}()...)
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	var archive string
	if runtime.GOOS == "windows" {
		archive = filepath.Join(out, name+".zip")
		zw := zip.NewWriter(&buf)
		for _, n := range order {
			data, err := os.ReadFile(files[n])
			if err != nil {
				return "", err
			}
			w, err := zw.CreateHeader(&zip.FileHeader{Name: name + "/" + n, Method: zip.Deflate, Modified: stamp})
			if err != nil {
				return "", err
			}
			if _, err := w.Write(data); err != nil {
				return "", err
			}
		}
		if err := zw.Close(); err != nil {
			return "", err
		}
	} else {
		archive = filepath.Join(out, name+".tar.gz")
		gz, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		gz.ModTime = stamp
		tw := tar.NewWriter(gz)
		for _, n := range order {
			data, err := os.ReadFile(files[n])
			if err != nil {
				return "", err
			}
			mode := int64(0o644)
			if n == exe {
				mode = 0o755
			}
			hdr := &tar.Header{Name: name + "/" + n, Mode: mode, Size: int64(len(data)), ModTime: stamp, Format: tar.FormatPAX}
			if err := tw.WriteHeader(hdr); err != nil {
				return "", err
			}
			if _, err := tw.Write(data); err != nil {
				return "", err
			}
		}
		if err := tw.Close(); err != nil {
			return "", err
		}
		if err := gz.Close(); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return archive, nil
}
