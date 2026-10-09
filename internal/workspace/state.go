package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// State is what PyxForge remembers about a workspace between sessions: the layout and the
// files that were open. It lives in the user's configuration folder, never in the project.
type State struct {
	Root      string   `json:"root"`                // the workspace it belongs to
	Files     []string `json:"files,omitempty"`     // open files, absolute, in tab order
	Active    string   `json:"active,omitempty"`    // the selected file
	Expanded  []string `json:"expanded,omitempty"`  // open explorer folders, relative, slash-separated
	Explorer  *bool    `json:"explorer,omitempty"`  // panel visibility; nil means the default
	Panel     *bool    `json:"panel,omitempty"`     //
	Inspector *bool    `json:"inspector,omitempty"` //
	PanelTab  string   `json:"panelTab,omitempty"`  // the bottom panel's selected tab
	Build     string   `json:"build,omitempty"`     // the chosen build profile
	Width     float32  `json:"width,omitempty"`     // window size
	Height    float32  `json:"height,omitempty"`    //
}

// StateStore reads and writes workspace states as JSON files in Dir, one per workspace.
type StateStore struct{ Dir string }

// DefaultStateStore keeps states under the user configuration folder.
func DefaultStateStore() (StateStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return StateStore{}, err
	}
	return StateStore{Dir: filepath.Join(base, "PyxForge", "workspaces")}, nil
}

// path names a workspace's file by a hash of its folder, so any path is a safe file name.
func (st StateStore) path(root string) string {
	key := filepath.Clean(root)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key) // the same folder however it was typed
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(st.Dir, hex.EncodeToString(sum[:8])+".json")
}

// Load returns root's saved state; a workspace never saved, or a file that cannot be read,
// gives an empty state, so a damaged file costs the layout and nothing else.
func (st StateStore) Load(root string) State {
	data, err := os.ReadFile(st.path(root))
	if err != nil {
		return State{Root: root}
	}
	var s State
	if json.Unmarshal(data, &s) != nil || !strings.EqualFold(filepath.Clean(s.Root), filepath.Clean(root)) {
		return State{Root: root}
	}
	// Forget files that are gone.
	kept := s.Files[:0]
	for _, f := range s.Files {
		if fi, err := os.Stat(f); err == nil && fi.Mode().IsRegular() {
			kept = append(kept, f)
		}
	}
	s.Files = kept
	return s
}

// Save writes root's state, replacing the previous file atomically.
func (st StateStore) Save(s State) error {
	if s.Root == "" {
		return errors.New("workspace state without a root")
	}
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dst := st.path(s.Root)
	tmp, err := os.CreateTemp(st.Dir, ".state-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Forget deletes root's saved state.
func (st StateStore) Forget(root string) error {
	err := os.Remove(st.path(root))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
