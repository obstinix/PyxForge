package qemu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
)

// MaxMachineSnapshots bounds the snapshots one overlay keeps. Each holds the guest's whole RAM
// ([qemu] memory), so they add up quickly.
const MaxMachineSnapshots = 10

// Machine snapshots need a disk QEMU can write snapshots into, and a raw boot image is not one.
// With [qemu] snapshots = true PyxForge boots the image through a qcow2 overlay whose backing
// file is the image: the guest's writes and the snapshots go to the overlay, the image stays as
// the build left it. The overlay lives in the user cache folder, one per workspace, and is
// created again (losing its snapshots) when the image changes.

// overlayInfo records which build of the image an overlay was made for.
type overlayInfo struct {
	Image  string `json:"image"`
	SHA256 string `json:"sha256"`
}

// OverlayDir is where a workspace's overlay is kept: PyxForge/qemu/<hash of the workspace>
// under the user cache folder.
func OverlayDir(workspace string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	key := filepath.Clean(workspace)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(base, "PyxForge", "qemu", hex.EncodeToString(sum[:8])), nil
}

// PrepareOverlay returns the overlay for image in dir, creating it with qemu-img when there is
// none or when the image is not the build it was made for. fresh reports a new overlay: any
// earlier machine snapshots are gone.
func PrepareOverlay(ctx context.Context, image, dir string) (overlay string, fresh bool, err error) {
	abs, err := filepath.Abs(image)
	if err != nil {
		return "", false, err
	}
	sum, err := fileSHA256(abs)
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", image, err)
	}
	overlay = filepath.Join(dir, "machine.qcow2")
	infoPath := filepath.Join(dir, "machine.json")
	want := overlayInfo{Image: abs, SHA256: sum}
	if data, err := os.ReadFile(infoPath); err == nil {
		var have overlayInfo
		if json.Unmarshal(data, &have) == nil && have == want {
			if _, err := os.Stat(overlay); err == nil {
				return overlay, false, nil
			}
		}
	}
	img, err := toolchain.LookPath("qemu-img")
	if err != nil {
		return "", false, errors.New(missing("qemu-img") + "; machine snapshots need it")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	_ = os.Remove(overlay)
	cmd := exec.CommandContext(ctx, img, "create", "-q", "-f", "qcow2", "-F", "raw", "-b", abs, overlay)
	proc.Bind(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("qemu-img create: %v: %s", err, strings.TrimSpace(string(out)))
	}
	data, _ := json.Marshal(want)
	if err := os.WriteFile(infoPath, data, 0o644); err != nil {
		return "", false, err
	}
	return overlay, true, nil
}

func fileSHA256(path string) (string, error) {
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

// MachineSnapshot is one entry of QEMU's `info snapshots`.
type MachineSnapshot struct {
	ID     string
	Tag    string // its name
	VMSize string // the saved RAM and device state, as QEMU prints it ("2.15 MiB")
	Date   string
	Clock  string // the guest clock when it was saved
}

var snapshotName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,40}$`)

// CheckSnapshotName reports why a name cannot be used: names are 1-40 letters, digits, dots,
// hyphens and underscores, so QEMU's listing reads back unambiguously.
func CheckSnapshotName(name string) error {
	if !snapshotName.MatchString(name) {
		return fmt.Errorf("%q: use 1-40 letters, digits, '.', '-' or '_'", name)
	}
	return nil
}

// ErrTooManySnapshots means the overlay holds MaxMachineSnapshots already.
var ErrTooManySnapshots = fmt.Errorf("%d machine snapshots are kept at most; delete one first", MaxMachineSnapshots)

// hmp runs a monitor command and turns the monitor's own error text into an error.
func (i *Instance) hmp(ctx context.Context, line string) (string, error) {
	if i.QMP == nil {
		return "", errors.New("QEMU's machine protocol is not connected")
	}
	out, err := i.QMP.HMP(ctx, line)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(strings.ReplaceAll(out, "\r\n", "\n"))
	if strings.HasPrefix(out, "Error:") || strings.Contains(out, "\nError:") {
		msg := out[strings.Index(out, "Error:"):]
		if low := strings.ToLower(msg); strings.Contains(low, "vmstate") || strings.Contains(low, "accept snapshots") ||
			strings.Contains(low, "does not support snapshots") {
			msg += " (set snapshots = true under [qemu] in pyxforge.toml to boot through a snapshot-capable overlay)"
		}
		return out, errors.New(msg)
	}
	return out, nil
}

// MachineSnapshots lists the saved machine states.
func (i *Instance) MachineSnapshots(ctx context.Context) ([]MachineSnapshot, error) {
	out, err := i.hmp(ctx, "info snapshots")
	if err != nil {
		return nil, err
	}
	return parseSnapshots(out), nil
}

// parseSnapshots reads `info snapshots` (QEMU 11):
//
//	List of snapshots present on all disks:
//	ID      TAG               VM_SIZE                DATE        VM_CLOCK     ICOUNT
//	--      after-ok         1.18 MiB 2026-10-10 18:43:23  0000:00:00.079         --
//
// The ID is "--" for snapshots present on every disk and a number in per-disk listings.
func parseSnapshots(out string) []MachineSnapshot {
	var list []MachineSnapshot
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 7 || (f[0] != "--" && !isNumber(f[0])) || !strings.HasSuffix(f[3], "B") {
			continue
		}
		list = append(list, MachineSnapshot{ID: f[0], Tag: f[1], VMSize: f[2] + " " + f[3], Date: f[4] + " " + f[5], Clock: f[6]})
	}
	return list
}

func isNumber(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// SaveMachine saves the machine's state under name, replacing a snapshot with that name.
func (i *Instance) SaveMachine(ctx context.Context, name string) error {
	if err := CheckSnapshotName(name); err != nil {
		return err
	}
	list, err := i.MachineSnapshots(ctx)
	if err != nil {
		return err
	}
	replacing := false
	for _, s := range list {
		replacing = replacing || s.Tag == name
	}
	if !replacing && len(list) >= MaxMachineSnapshots {
		return ErrTooManySnapshots
	}
	_, err = i.hmp(ctx, "savevm "+name)
	return err
}

// LoadMachine restores a saved state: memory, registers and devices return to that moment.
func (i *Instance) LoadMachine(ctx context.Context, name string) error {
	if err := CheckSnapshotName(name); err != nil {
		return err
	}
	_, err := i.hmp(ctx, "loadvm "+name)
	return err
}

// DeleteMachine removes a saved state.
func (i *Instance) DeleteMachine(ctx context.Context, name string) error {
	if err := CheckSnapshotName(name); err != nil {
		return err
	}
	_, err := i.hmp(ctx, "delvm "+name)
	return err
}
