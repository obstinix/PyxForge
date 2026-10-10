package qemu

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obstinix/PyxForge/internal/toolchain"
)

func TestParseSnapshots(t *testing.T) {
	// Real QEMU 11.1 output.
	out := "List of snapshots present on all disks:\n" +
		"ID      TAG               VM_SIZE                DATE        VM_CLOCK     ICOUNT\n" +
		"--      after-ok         1.18 MiB 2026-10-10 18:43:23  0000:00:00.079         --\n" +
		"--      second.try-1     1.18 MiB 2026-10-10 18:44:01  0000:00:02.001         --\n"
	got := parseSnapshots(out)
	if len(got) != 2 || got[0].Tag != "after-ok" || got[0].VMSize != "1.18 MiB" || got[0].Date != "2026-10-10 18:43:23" ||
		got[1].Tag != "second.try-1" || got[1].Clock != "0000:00:02.001" {
		t.Errorf("parsed %+v", got)
	}
	perDisk := "ID  TAG  VM SIZE  DATE  VM CLOCK\n1   boot  2.2 MiB 2020-01-01 00:00:00 00:00:00.000\n"
	if got := parseSnapshots(perDisk); len(got) != 1 || got[0].ID != "1" {
		t.Errorf("per-disk listing: %+v", got)
	}
	if parseSnapshots("There is no snapshot available.") != nil {
		t.Error("parsed snapshots from an empty listing")
	}
	for _, bad := range []string{"", "has space", "semi;colon", strings.Repeat("x", 41), "../up"} {
		if CheckSnapshotName(bad) == nil {
			t.Errorf("name %q accepted", bad)
		}
	}
}

// TestMachineSnapshots saves, lists, restores and deletes machine state in a real QEMU booted
// through an overlay, and checks the overlay follows the image's builds.
func TestMachineSnapshots(t *testing.T) {
	needQEMU(t)
	if _, err := toolchain.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	root := writeImage(t)
	image := filepath.Join(root, "boot.bin")
	before, _ := os.ReadFile(image)
	dir := t.TempDir()

	overlay, fresh, err := PrepareOverlay(ctx, image, dir)
	if err != nil || !fresh {
		t.Fatalf("first overlay: %v, fresh %v", err, fresh)
	}
	if _, again, err := PrepareOverlay(ctx, image, dir); err != nil || again {
		t.Fatalf("same image: fresh %v, %v", again, err)
	}

	c := qemuConfig(t, "[qemu]\nmemory = \"16M\"\nboot_image = \"boot.bin\"\nsnapshots = true\nextra_args = [\"-serial\", \"stdio\", \"-display\", \"none\"]\n")
	if !c.Qemu.Snapshots {
		t.Fatal("snapshots = true was not read")
	}
	var mu sync.Mutex
	var out []string
	inst, err := Launch(ctx, c.Qemu, Options{Root: root, Overlay: overlay, OnOutput: func(l string) {
		mu.Lock()
		out = append(out, l)
		mu.Unlock()
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Kill()
	if !slices.Contains(inst.Args, "format=qcow2,file="+overlay) {
		t.Fatalf("not booted through the overlay: %q", inst.Args)
	}
	for deadline := time.Now().Add(10 * time.Second); ; {
		mu.Lock()
		ok := slices.Contains(out, "OK")
		mu.Unlock()
		if ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no serial output: %q", out)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := inst.SaveMachine(ctx, "after-ok"); err != nil {
		t.Fatal(err)
	}
	raw, _ := inst.QMP.HMP(ctx, "info snapshots")
	list, err := inst.MachineSnapshots(ctx)
	if err != nil || len(list) != 1 || list[0].Tag != "after-ok" || list[0].VMSize == "" {
		t.Fatalf("list %+v, %v; QEMU said:\n%s", list, err, raw)
	}
	if err := inst.SaveMachine(ctx, "bad name"); err == nil {
		t.Error("a bad name was saved")
	}
	if err := inst.LoadMachine(ctx, "after-ok"); err != nil {
		t.Fatal(err)
	}
	if err := inst.LoadMachine(ctx, "nonesuch"); err == nil {
		t.Error("loaded a snapshot that does not exist")
	}
	if err := inst.DeleteMachine(ctx, "after-ok"); err != nil {
		t.Fatal(err)
	}
	if list, _ := inst.MachineSnapshots(ctx); len(list) != 0 {
		t.Errorf("after delete: %+v", list)
	}
	inst.Stop()
	if after, _ := os.ReadFile(image); string(after) != string(before) {
		t.Error("the boot image changed: writes did not go to the overlay")
	}

	// A rebuilt image gets a new overlay.
	changed := append([]byte(nil), before...)
	changed[4] = 'X'
	if err := os.WriteFile(image, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, fresh, err := PrepareOverlay(ctx, image, dir); err != nil || !fresh {
		t.Errorf("rebuilt image: fresh %v, %v", fresh, err)
	}
}

// TestSnapshotsNeedAnOverlay: a raw image cannot hold snapshots, and the error says what to do.
func TestSnapshotsNeedAnOverlay(t *testing.T) {
	needQEMU(t)
	root := writeImage(t)
	c := qemuConfig(t, "[qemu]\nmemory = \"16M\"\nboot_image = \"boot.bin\"\nextra_args = [\"-display\", \"none\"]\n")
	inst, err := Launch(context.Background(), c.Qemu, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Kill()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := inst.SaveMachine(ctx, "x"); err == nil || !strings.Contains(err.Error(), "snapshots = true") {
		t.Errorf("save on a raw image: %v", err)
	}
}
