package managed

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// fakeVendor stands in for a vendor installer and uninstaller run with administrator rights:
// it records what it was asked and writes or removes the files the real ones would.
type fakeVendor struct {
	calls   [][]string
	decline bool
	files   []string
}

func (f *fakeVendor) elevate(_ context.Context, exe string, args []string) error {
	f.calls = append(f.calls, append([]string{filepath.Base(exe)}, args...))
	if f.decline {
		return ErrDeclined
	}
	if dir, ok := strings.CutPrefix(args[len(args)-1], "/D="); ok { // the installer
		for _, name := range f.files {
			p := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	return os.RemoveAll(filepath.Dir(exe)) // the uninstaller removes its folder
}

func installerPkg(url string, setup []byte) Package {
	return Package{ID: "qemu", Name: "QEMU", Version: "1", GOOS: "windows", GOARCH: "amd64", URL: url,
		SHA256: sum(setup), Size: int64(len(setup)), Installer: []string{"/S"}, InstallDir: "${PROGRAMS}/qemu",
		Uninstaller: "qemu-uninstall.exe", Bin: []string{"."}, Requires: []string{"qemu-system-x86_64.exe"}}
}

// TestInstallerPackage: an installer package is downloaded and checked like an archive, run
// silently with its folder last, recorded once its files are there, found in BinDirs, and
// removed with its uninstaller.
func TestInstallerPackage(t *testing.T) {
	setup := []byte("MZ pretend installer")
	srv, _ := server(t, map[string][]byte{"/setup.exe": setup})
	programs := t.TempDir()
	m := newManager(t)
	m.Getenv = func(k string) string { return map[string]string{"PROGRAMS": programs}[k] }
	vendor := &fakeVendor{files: []string{"qemu-system-x86_64.exe", "qemu-uninstall.exe"}}
	m.Elevate = vendor.elevate
	p := installerPkg(srv.URL+"/setup.exe", setup)
	p.GOOS, p.GOARCH = runtime.GOOS, runtime.GOARCH // BinDirs lists this platform's packages
	old := Packages
	t.Cleanup(func() { Packages = old })
	Packages = []Package{p}
	dir := filepath.Join(programs, "qemu")

	if err := m.Install(context.Background(), p, nil); err != nil {
		t.Fatal(err)
	}
	if len(vendor.calls) != 1 || vendor.calls[0][0] != p.SHA256+".exe" || !slices.Equal(vendor.calls[0][1:], []string{"/S", "/D=" + dir}) {
		t.Errorf("installer run as %q", vendor.calls)
	}
	if st := m.Status(p); st.State != Installed {
		t.Errorf("after install: %v %v", st.State, st.Problem)
	}
	if bins := m.BinDirs(); len(bins) != 1 || bins[0] != dir {
		t.Errorf("BinDirs %v, want %s", bins, dir)
	}
	if left, _ := os.ReadDir(m.Downloads); len(left) != 0 {
		t.Errorf("the installer was kept: %v", left)
	}

	uninstallWait = 0
	if err := m.Uninstall(p); err != nil {
		t.Fatal(err)
	}
	if last := vendor.calls[len(vendor.calls)-1]; !slices.Equal(last, []string{"qemu-uninstall.exe", "/S"}) {
		t.Errorf("uninstaller run as %q", last)
	}
	if st := m.Status(p); st.State != NotInstalled {
		t.Errorf("after uninstall: %v", st.State)
	}
}

// TestInstallerLeavesOthersAlone: a program someone else installed where the installer would put
// it shows as installed outside PyxForge; PyxForge neither installs over it nor removes it.
func TestInstallerLeavesOthersAlone(t *testing.T) {
	setup := []byte("MZ pretend installer")
	srv, hits := server(t, map[string][]byte{"/setup.exe": setup})
	programs := t.TempDir()
	if err := os.MkdirAll(filepath.Join(programs, "qemu"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(programs, "qemu", "qemu-system-x86_64.exe"), []byte("theirs"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newManager(t)
	m.Getenv = func(string) string { return programs }
	vendor := &fakeVendor{}
	m.Elevate = vendor.elevate
	p := installerPkg(srv.URL+"/setup.exe", setup)
	if st := m.Status(p); st.State != External {
		t.Errorf("state %v, want installed outside PyxForge", st.State)
	}
	if err := m.Install(context.Background(), p, nil); err == nil || !strings.Contains(err.Error(), "someone other than PyxForge") {
		t.Errorf("install over theirs: %v", err)
	}
	if err := m.Uninstall(p); err == nil || len(vendor.calls) != 0 {
		t.Errorf("uninstall of theirs: %v, runs %q", err, vendor.calls)
	}
	if hits.Load() != 0 {
		t.Error("the installer was downloaded")
	}
}

// TestInstallerDeclined: when the user declines Windows' prompt nothing is recorded.
func TestInstallerDeclined(t *testing.T) {
	setup := []byte("MZ pretend installer")
	srv, _ := server(t, map[string][]byte{"/setup.exe": setup})
	m := newManager(t)
	m.Getenv = func(string) string { return t.TempDir() }
	m.Elevate = (&fakeVendor{decline: true}).elevate
	p := installerPkg(srv.URL+"/setup.exe", setup)
	if err := m.Install(context.Background(), p, nil); !errors.Is(err, ErrDeclined) {
		t.Fatalf("install: %v", err)
	}
	if st := m.Status(p); st.State != NotInstalled {
		t.Errorf("state %v", st.State)
	}
}

// TestInstallDirNeedsItsVariable: an install folder naming a variable the system does not set
// is no folder, not a relative path.
func TestInstallDirNeedsItsVariable(t *testing.T) {
	m := newManager(t)
	m.Getenv = func(string) string { return "" }
	p := installerPkg("https://example.invalid/setup.exe", []byte("x"))
	if d := m.PackageDir(p); d != "" {
		t.Errorf("PackageDir = %q", d)
	}
	if st := m.Status(p); st.State != NotInstalled {
		t.Errorf("state %v", st.State)
	}
}
