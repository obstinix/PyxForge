// Package snapshot records what a paused machine looked like (registers, flags, the code at the
// program counter, the stack and chosen memory) so that two moments of a debug session can be
// compared later. A diagnostic snapshot is a record, not machine state: it cannot restore a
// machine. QEMU's own snapshots (internal/qemu) do that.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"
)

// Version is the file format's version; files of other versions are listed but not compared.
const Version = 1

// MaxKept is how many snapshots a workspace keeps; saving another removes the oldest.
const MaxKept = 50

// Memory is a range of bytes read from the machine.
type Memory struct {
	Label string `json:"label"` // "code", "stack", or the address the user asked for
	Addr  uint64 `json:"addr"`
	Bytes []byte `json:"bytes"` // base64 in the file
}

// Snapshot is one capture.
type Snapshot struct {
	Version   int               `json:"version"`
	ID        string            `json:"id"` // file name without extension
	Name      string            `json:"name"`
	Taken     time.Time         `json:"taken"`
	Workspace string            `json:"workspace"`
	Image     string            `json:"image,omitempty"`        // the image QEMU booted, relative to the workspace
	ImageHash string            `json:"image_sha256,omitempty"` // to tell captures of different builds apart
	Mode      int               `json:"mode"`                   // 16, 32 or 64
	PC        uint64            `json:"pc"`                     // linear address of the next instruction
	Reason    string            `json:"reason,omitempty"`       // why the machine stopped
	Registers map[string]uint64 `json:"registers"`              // every register GDB reported, by GDB's name
	Memory    []Memory          `json:"memory"`
	Listing   []string          `json:"listing,omitempty"` // the code at the program counter, disassembled
}

// HashFile returns a file's SHA-256 in hex, or "" if it cannot be read.
func HashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Store keeps a workspace's snapshots as JSON files in a folder of their own.
type Store struct {
	Dir string
}

// DefaultStore is the store for a workspace under the user configuration folder:
// PyxForge/snapshots/<hash of the workspace path>.
func DefaultStore(workspace string) (Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	key := filepath.Clean(workspace)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	sum := sha256.Sum256([]byte(key))
	return Store{Dir: filepath.Join(base, "PyxForge", "snapshots", hex.EncodeToString(sum[:8]))}, nil
}

var unsafeChars = regexp.MustCompile(`[^a-z0-9]+`)

// Save writes a snapshot, giving it an ID from its time and name, and removes the oldest
// snapshots beyond MaxKept. It returns how many it removed.
func (st Store) Save(s *Snapshot) (removed int, err error) {
	if s.Name = strings.TrimSpace(s.Name); s.Name == "" {
		return 0, errors.New("a snapshot needs a name")
	}
	if s.Taken.IsZero() {
		s.Taken = time.Now()
	}
	s.Version = Version
	slug := strings.Trim(unsafeChars.ReplaceAllString(strings.ToLower(s.Name), "-"), "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	s.ID = s.Taken.UTC().Format("20060102T150405.000Z") + "-" + slug
	s.ID = strings.ReplaceAll(s.ID, ".", "")
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		return 0, err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(st.Dir, ".snap-*")
	if err != nil {
		return 0, err
	}
	_, werr := tmp.Write(data)
	if err := errors.Join(werr, tmp.Close()); err != nil {
		os.Remove(tmp.Name())
		return 0, err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(st.Dir, s.ID+".json")); err != nil {
		os.Remove(tmp.Name())
		return 0, err
	}
	list, err := st.List()
	if err != nil {
		return 0, nil
	}
	for i := MaxKept; i < len(list); i++ { // List is newest first
		if st.Delete(list[i].ID) == nil {
			removed++
		}
	}
	return removed, nil
}

// List returns the snapshots, newest first. Files that cannot be read are skipped.
func (st Store) List() ([]Snapshot, error) {
	entries, err := os.ReadDir(st.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Snapshot
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		s, err := st.Load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Taken.After(out[j].Taken) })
	return out, nil
}

var idRE = regexp.MustCompile(`^[0-9TZ]+-[a-z0-9-]*$`)

// Load reads one snapshot.
func (st Store) Load(id string) (*Snapshot, error) {
	if !idRE.MatchString(id) {
		return nil, fmt.Errorf("%q is not a snapshot", id)
	}
	data, err := os.ReadFile(filepath.Join(st.Dir, id+".json"))
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("snapshot %s is damaged: %w", id, err)
	}
	s.ID = id
	return &s, nil
}

// Delete removes one snapshot.
func (st Store) Delete(id string) error {
	if !idRE.MatchString(id) {
		return fmt.Errorf("%q is not a snapshot", id)
	}
	return os.Remove(filepath.Join(st.Dir, id+".json"))
}

// RegisterChange is a register whose value differs.
type RegisterChange struct {
	Name     string
	Old, New uint64
}

// ByteChange is a run of changed bytes in a memory range.
type ByteChange struct {
	Addr     uint64
	Old, New []byte
}

// MemoryChange is a memory range present in both snapshots, with its changed runs.
type MemoryChange struct {
	Label   string
	Addr    uint64
	Changes []ByteChange
}

// Diff is what changed from one snapshot to another.
type Diff struct {
	Registers []RegisterChange
	Memory    []MemoryChange
	// Notes say what could not be compared: ranges in only one snapshot, different images.
	Notes []string
}

// Compare reports the registers and memory that differ from a to b. Memory ranges are matched
// by label and address; only bytes both snapshots read are compared.
func Compare(a, b *Snapshot) Diff {
	var d Diff
	if a.Version != Version || b.Version != Version {
		d.Notes = append(d.Notes, fmt.Sprintf("format version %d against %d: not compared", a.Version, b.Version))
		return d
	}
	if a.ImageHash != "" && b.ImageHash != "" && a.ImageHash != b.ImageHash {
		d.Notes = append(d.Notes, "the snapshots were taken of different builds of the image")
	}
	if a.Mode != b.Mode {
		d.Notes = append(d.Notes, fmt.Sprintf("the machine changed mode: %d-bit to %d-bit", a.Mode, b.Mode))
	}
	names := make([]string, 0, len(a.Registers))
	for n := range a.Registers {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		if nv, ok := b.Registers[n]; ok && nv != a.Registers[n] {
			d.Registers = append(d.Registers, RegisterChange{Name: n, Old: a.Registers[n], New: nv})
		}
	}
	for _, ma := range a.Memory {
		i := slices.IndexFunc(b.Memory, func(m Memory) bool { return m.Label == ma.Label && m.Addr == ma.Addr })
		if i < 0 {
			d.Notes = append(d.Notes, fmt.Sprintf("%s at 0x%x is only in the first snapshot", ma.Label, ma.Addr))
			continue
		}
		mc := MemoryChange{Label: ma.Label, Addr: ma.Addr, Changes: byteRuns(ma.Addr, ma.Bytes, b.Memory[i].Bytes)}
		if len(mc.Changes) > 0 {
			d.Memory = append(d.Memory, mc)
		}
	}
	for _, mb := range b.Memory {
		if !slices.ContainsFunc(a.Memory, func(m Memory) bool { return m.Label == mb.Label && m.Addr == mb.Addr }) {
			d.Notes = append(d.Notes, fmt.Sprintf("%s at 0x%x is only in the second snapshot", mb.Label, mb.Addr))
		}
	}
	return d
}

// byteRuns finds the runs of differing bytes over the length both slices have.
func byteRuns(base uint64, a, b []byte) []ByteChange {
	var out []ByteChange
	n := min(len(a), len(b))
	for i := 0; i < n; {
		if a[i] == b[i] {
			i++
			continue
		}
		j := i
		for j < n && a[j] != b[j] {
			j++
		}
		out = append(out, ByteChange{Addr: base + uint64(i), Old: slices.Clone(a[i:j]), New: slices.Clone(b[i:j])})
		i = j
	}
	return out
}

// Same reports whether a diff found nothing that changed.
func (d Diff) Same() bool { return len(d.Registers) == 0 && len(d.Memory) == 0 }
