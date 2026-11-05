package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Workspace trust (#31, #30). Opening a project runs nothing from it (D14), but Build, Run and
// Debug in QEMU, Run and Debug Current File and a repository's own Git hooks folder run
// programs the project's files name. PyxForge asks before the first of them in a folder the user
// hasn't trusted, showing what would run, and remembers the answer here: in the user's
// configuration folder, never in a project, so no project can trust itself. A trusted folder
// trusts everything inside it.

// TrustStore keeps the folders the user trusts, in a JSON file.
type TrustStore struct{ Path string }

type trustFile struct {
	Folders []string `json:"folders"`
}

// DefaultTrustStore is the store in the user's configuration folder.
func DefaultTrustStore() (TrustStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return TrustStore{}, err
	}
	return TrustStore{Path: filepath.Join(base, "PyxForge", "trusted-folders.json")}, nil
}

// Canonical is dir as the store compares it: absolute and clean, with symbolic links resolved
// when it exists, and on Windows in one case, as its file system ignores case.
func Canonical(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	dir = filepath.Clean(dir)
	if runtime.GOOS == "windows" {
		dir = strings.ToLower(dir)
	}
	return dir
}

// List returns the trusted folders, as they were trusted.
func (st TrustStore) List() ([]string, error) {
	if st.Path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(st.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f trustFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Folders, nil
}

// Trusted reports whether dir is a trusted folder or inside one. A store that can't be read
// trusts nothing.
func (st TrustStore) Trusted(dir string) bool { return st.TrustedBy(dir) != "" }

// Trust adds dir to the trusted folders.
func (st TrustStore) Trust(dir string) error {
	list, err := st.List()
	if err != nil {
		return err
	}
	d := Canonical(dir)
	if slices.ContainsFunc(list, func(t string) bool { return Canonical(t) == d }) {
		return nil
	}
	shown := dir // kept as spelled, for Settings; compared as Canonical
	if abs, err := filepath.Abs(dir); err == nil {
		shown = filepath.Clean(abs)
	}
	return st.write(append(list, shown))
}

// Revoke removes dir from the trusted folders. A folder trusted only because a folder around
// it is trusted stays trusted until that one is revoked.
func (st TrustStore) Revoke(dir string) error {
	list, err := st.List()
	if err != nil {
		return err
	}
	d := Canonical(dir)
	kept := slices.DeleteFunc(list, func(t string) bool { return Canonical(t) == d })
	return st.write(kept)
}

// TrustedBy returns the trusted folder that makes dir trusted, as it was trusted, or "".
func (st TrustStore) TrustedBy(dir string) string {
	if dir == "" {
		return ""
	}
	list, err := st.List()
	if err != nil {
		return ""
	}
	d := Canonical(dir)
	for _, t := range list {
		c := Canonical(t)
		if d == c || strings.HasPrefix(d, strings.TrimSuffix(c, string(filepath.Separator))+string(filepath.Separator)) {
			return t
		}
	}
	return ""
}

func (st TrustStore) write(list []string) error {
	if list == nil {
		list = []string{}
	}
	data, err := json.MarshalIndent(trustFile{Folders: list}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(st.Path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(st.Path), ".trusted-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), st.Path)
}
