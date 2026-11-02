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
	// Panel sizes the user dragged to; 0 means the default.
	ExplorerWidth  float32 `json:"explorerWidth,omitempty"`
	DockHeight     float32 `json:"dockHeight,omitempty"`
	InspectorWidth float32 `json:"inspectorWidth,omitempty"`
	// Breakpoints set in the editor, by file.
	Breakpoints []Breakpoint `json:"breakpoints,omitempty"`
}

// Breakpoint is one file's breakpoints.
type Breakpoint struct {
	File  string `json:"file"`  // absolute
	Lines []int  `json:"lines"` // 1-based
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

// ShadaFile is where the workspace's editor keeps Neovim's history (PXF-DEF-001): beside its
// state, so the jumplist and old files of one project never open another's files. Its folder
// is made if need be. Without a workspace (root ""), files opened on their own share a history
// of their own, never Neovim's default one, which every PyxForge window would read.
func (st StateStore) ShadaFile(root string) string {
	p := filepath.Join(st.Dir, "no-workspace.shada")
	if root != "" {
		p = strings.TrimSuffix(st.path(root), ".json") + ".shada"
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	return p
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
	serr := tmp.Sync() // on disk before the rename: a crash then leaves the old file or the new, not one of NULs
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
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

// Prune removes the saved state and editor history of every workspace whose folder no longer
// exists (#54). It returns the files it removed.
func (st StateStore) Prune() ([]string, error) {
	entries, err := os.ReadDir(st.Dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var removed []string
	var errs []error
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		file := filepath.Join(st.Dir, e.Name())
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var s State
		if json.Unmarshal(b, &s) != nil || s.Root == "" {
			continue // not a state this version can read; leave it
		}
		if _, err := os.Stat(s.Root); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		for _, p := range []string{file, strings.TrimSuffix(file, ".json") + ".shada"} {
			switch err := os.Remove(p); {
			case err == nil:
				removed = append(removed, p)
			case !errors.Is(err, fs.ErrNotExist):
				errs = append(errs, err)
			}
		}
	}
	return removed, errors.Join(errs...)
}
