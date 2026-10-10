package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sample(name string, at time.Time, ax uint64, stack []byte) *Snapshot {
	return &Snapshot{Name: name, Taken: at, Workspace: "/os", Image: "build/boot.bin", ImageHash: "abc", Mode: 16, PC: 0x7c03,
		Reason: "end-stepping-range", Registers: map[string]uint64{"rax": ax, "rip": 0x7c03, "eflags": 0x2},
		Memory:  []Memory{{Label: "stack", Addr: 0x7bf0, Bytes: stack}, {Label: "code", Addr: 0x7c00, Bytes: []byte{0xba, 0xf8, 0x03}}},
		Listing: []string{"7c00: mov dx, 0x3f8"}}
}

func TestStoreRoundTripAndCompare(t *testing.T) {
	st := Store{Dir: filepath.Join(t.TempDir(), "snaps")}
	if list, err := st.List(); err != nil || list != nil {
		t.Fatalf("empty store: %v %v", list, err)
	}
	t0 := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	a := sample("Before the INT 13h call!", t0, 0x0200, []byte{1, 2, 3, 4, 5, 6})
	b := sample("after", t0.Add(time.Second), 0x0201, []byte{1, 9, 9, 4, 5, 7})
	for _, s := range []*Snapshot{a, b} {
		if _, err := st.Save(s); err != nil {
			t.Fatal(err)
		}
	}
	if a.ID != "20261010T030000000Z-before-the-int-13h-call" {
		t.Errorf("id %q", a.ID)
	}
	list, err := st.List()
	if err != nil || len(list) != 2 || list[0].Name != "after" {
		t.Fatalf("list: %+v, %v", list, err)
	}
	got, err := st.Load(a.ID)
	if err != nil || got.Registers["rax"] != 0x200 || string(got.Memory[0].Bytes) != "\x01\x02\x03\x04\x05\x06" || got.Listing[0] != a.Listing[0] {
		t.Fatalf("load: %+v, %v", got, err)
	}

	d := Compare(got, b)
	if len(d.Registers) != 1 || d.Registers[0] != (RegisterChange{Name: "rax", Old: 0x200, New: 0x201}) {
		t.Errorf("register changes %+v", d.Registers)
	}
	if len(d.Memory) != 1 || d.Memory[0].Label != "stack" || len(d.Memory[0].Changes) != 2 ||
		d.Memory[0].Changes[0].Addr != 0x7bf1 || string(d.Memory[0].Changes[0].New) != "\x09\x09" || d.Memory[0].Changes[1].Addr != 0x7bf5 {
		t.Errorf("memory changes %+v", d.Memory)
	}
	if d.Same() || len(d.Notes) != 0 {
		t.Errorf("notes %v", d.Notes)
	}
	if !Compare(b, b).Same() {
		t.Error("a snapshot differs from itself")
	}

	// Different builds and ranges in only one snapshot are noted, not compared silently.
	c := sample("other build", t0.Add(2*time.Second), 0x200, []byte{1})
	c.ImageHash, c.Version = "def", Version // compared unsaved, so stamped by hand
	c.Memory = c.Memory[:1]
	if n := strings.Join(Compare(a, c).Notes, "; "); !strings.Contains(n, "different builds") || !strings.Contains(n, "code at 0x7c00 is only in the first") {
		t.Errorf("notes %q", n)
	}
	c.Version = 99
	if n := Compare(a, c).Notes; len(n) != 1 || !strings.Contains(n[0], "not compared") {
		t.Errorf("version notes %v", n)
	}

	if err := st.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := st.List(); len(list) != 1 {
		t.Errorf("after delete: %d", len(list))
	}
}

func TestStoreRejectsBadInput(t *testing.T) {
	st := Store{Dir: t.TempDir()}
	if _, err := st.Save(&Snapshot{Name: "   "}); err == nil {
		t.Error("a snapshot without a name was saved")
	}
	for _, id := range []string{"../escape", "x.json", "", "20261010T030000000Z-a/../b"} {
		if _, err := st.Load(id); err == nil {
			t.Errorf("Load(%q) succeeded", id)
		}
		if err := st.Delete(id); err == nil {
			t.Errorf("Delete(%q) succeeded", id)
		}
	}
	// A damaged file is skipped by List and reported by Load.
	if err := os.WriteFile(filepath.Join(st.Dir, "20261010T030000000Z-broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if list, err := st.List(); err != nil || len(list) != 0 {
		t.Errorf("list with a damaged file: %+v, %v", list, err)
	}
	if _, err := st.Load("20261010T030000000Z-broken"); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Errorf("load damaged: %v", err)
	}
}

func TestStoreKeepsAtMostMaxKept(t *testing.T) {
	st := Store{Dir: t.TempDir()}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	removed := 0
	for i := range MaxKept + 3 {
		n, err := st.Save(sample("s", t0.Add(time.Duration(i)*time.Second), uint64(i), nil))
		if err != nil {
			t.Fatal(err)
		}
		removed += n
	}
	list, _ := st.List()
	if len(list) != MaxKept || removed != 3 || list[len(list)-1].Registers["rax"] != 3 {
		t.Errorf("kept %d, removed %d, oldest rax %d", len(list), removed, list[len(list)-1].Registers["rax"])
	}
}
