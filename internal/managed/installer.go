package managed

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ErrDeclined means the user did not allow an installer to run with administrator rights.
var ErrDeclined = errors.New("the request to run it with administrator rights was declined")

// runInstaller runs an installer package's checked installer silently into its InstallDir,
// then checks it put there what it should and records it.
func (m *Manager) runInstaller(ctx context.Context, p Package, exe string, progress func(Progress)) error {
	dir := m.PackageDir(p)
	progress(Progress{Phase: "Waiting for approval"})
	elevate := m.Elevate
	if elevate == nil {
		elevate = runElevated
	}
	// NSIS installers take the folder last, as /D=, unquoted even when it has spaces.
	if err := elevate(ctx, exe, append(append([]string{}, p.Installer...), "/D="+dir)); err != nil {
		return fmt.Errorf("the %s installer: %w", p.Name, err)
	}
	progress(Progress{Phase: "Installing"})
	if err := m.check(p, dir); err != nil {
		return fmt.Errorf("the %s installer finished, but %w", p.Name, err)
	}
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return err
	}
	if err := writeAtomic(m.receiptPath(p.ID), Receipt{ID: p.ID, Version: p.Version, SHA256: p.SHA256, Bin: p.Bin, Installed: time.Now().UTC()}); err != nil {
		return err
	}
	_ = os.Remove(exe)
	return nil
}

// runUninstaller removes an installer package PyxForge installed, with the vendor's uninstaller.
// One installed by someone else is left alone.
func (m *Manager) runUninstaller(p Package) error {
	if _, mine := m.Receipt(p.ID); !mine {
		return fmt.Errorf("%s was not installed by PyxForge; remove it from Windows' Installed apps", p.Name)
	}
	dir := m.PackageDir(p)
	elevate := m.Elevate
	if elevate == nil {
		elevate = runElevated
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := elevate(ctx, filepath.Join(dir, p.Uninstaller), []string{"/S"}); err != nil {
		return fmt.Errorf("the %s uninstaller: %w", p.Name, err)
	}
	// An NSIS uninstaller carries on from a copy of itself: wait for the files to go.
	for deadline := time.Now().Add(uninstallWait); m.check(p, dir) == nil && time.Now().Before(deadline); {
		time.Sleep(200 * time.Millisecond)
	}
	if m.check(p, dir) == nil {
		return fmt.Errorf("the %s uninstaller ran, but %s is still there", p.Name, dir)
	}
	if err := os.Remove(m.receiptPath(p.ID)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	m.logf("uninstalled %s from %s", p.ID, dir)
	return nil
}

// uninstallWait is how long an uninstaller's copy of itself gets to remove the files.
var uninstallWait = 2 * time.Minute
