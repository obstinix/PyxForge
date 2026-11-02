package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// The AppImage tools, pinned by version and SHA-256 as published by their releases. The runtime
// is given to appimagetool as a file, so it downloads nothing itself.
var (
	appimagetool = pinned{"https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage",
		"ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0", "appimagetool-1.9.1-x86_64.AppImage"}
	appimageRuntime = pinned{"https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime-x86_64",
		"2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d", "type2-runtime-20251108-x86_64"}
)

type pinned struct{ url, sha256, name string }

// appRun starts the program from wherever the AppImage is mounted.
const appRun = `#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/pyxforge" "$@"
`

// writeAppDir lays out an AppDir: AppRun, the desktop entry and its icon at the top, as the
// AppImage format asks, and the same files as the Debian package under usr/.
func writeAppDir(dir string, files []file, icon []byte) error {
	all := append([]file{{"AppRun", 0o755, []byte(appRun)},
		{"pyxforge.desktop", 0o644, []byte(desktopEntry("pyxforge"))},
		{appID + ".png", 0o644, icon}, {".DirIcon", 0o644, icon}}, files...)
	for _, f := range all {
		p := filepath.Join(dir, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, f.data, os.FileMode(f.mode)); err != nil {
			return err
		}
		if err := os.Chmod(p, os.FileMode(f.mode)); err != nil { // WriteFile keeps an existing file's mode
			return err
		}
	}
	return nil
}

// buildAppImage packs dir into dst with the pinned appimagetool and runtime. It runs on Linux
// x86-64; appimagetool unpacks itself instead of mounting with FUSE, which CI runners lack.
func buildAppImage(dir, dst string) error {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return fmt.Errorf("an AppImage is built on Linux x86-64, not %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	tool, err := fetchPinned(appimagetool, 0o755)
	if err != nil {
		return err
	}
	rt, err := fetchPinned(appimageRuntime, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(tool, "--no-appstream", "--runtime-file", rt, dir, dst)
	cmd.Env = append(os.Environ(), "ARCH=x86_64", "APPIMAGE_EXTRACT_AND_RUN=1")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("appimagetool: %w", err)
	}
	return nil
}

// fetchPinned returns a pinned file from the user cache, downloading it first when it is
// missing, and refuses one whose SHA-256 differs.
func fetchPinned(p pinned, mode os.FileMode) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, "PyxForge", "linuxpkg", p.name)
	if sum, err := fileSum(path); err == nil && sum == p.sha256 {
		return path, os.Chmod(path, mode)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	resp, err := http.Get(p.url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", p.url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(data)
	if got := hex.EncodeToString(s[:]); got != p.sha256 {
		return "", fmt.Errorf("%s has SHA-256 %s, not the pinned %s", p.url, got, p.sha256)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return "", err
	}
	return path, os.Chmod(path, mode)
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
