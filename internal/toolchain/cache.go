package toolchain

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// VersionCache remembers the version each tool reported, by the executable's path, size and
// modification time, so a start-up check need not run every tool again (#53). A changed or
// replaced executable is asked again; a failed query is never remembered.
type VersionCache struct {
	File string // where it is kept; "" keeps it in memory only

	mu      sync.Mutex
	loaded  bool
	entries map[string]cachedVersion
	changed bool
}

type cachedVersion struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Args    string    `json:"args"`
	Version string    `json:"version"`
}

// versionKey is what a cached version is valid for: the file as it is now and the arguments
// that asked it.
func versionKey(path string, args []string) (cachedVersion, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return cachedVersion{}, false
	}
	return cachedVersion{Size: fi.Size(), ModTime: fi.ModTime().UTC(), Args: strings.Join(args, "\x00")}, true
}

func (c *VersionCache) load() {
	if c.loaded {
		return
	}
	c.loaded = true
	c.entries = map[string]cachedVersion{}
	if c.File == "" {
		return
	}
	if b, err := os.ReadFile(c.File); err == nil {
		_ = json.Unmarshal(b, &c.entries) // a damaged cache is only a slower start
	}
}

// get returns the version cached for path, if the file and arguments are unchanged.
func (c *VersionCache) get(path string, args []string) (string, bool) {
	k, ok := versionKey(path, args)
	if !ok {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	e, ok := c.entries[path]
	if !ok || e.Size != k.Size || !e.ModTime.Equal(k.ModTime) || e.Args != k.Args {
		return "", false
	}
	return e.Version, true
}

// put remembers version for path as the file is now.
func (c *VersionCache) put(path string, args []string, version string) {
	k, ok := versionKey(path, args)
	if !ok {
		return
	}
	k.Version = version
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	if c.entries[path] != k {
		c.entries[path] = k
		c.changed = true
	}
}

// Save writes the cache to File if anything changed.
func (c *VersionCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.changed || c.File == "" {
		return nil
	}
	b, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.File), 0o755); err != nil {
		return err
	}
	tmp := c.File + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.File); err != nil {
		os.Remove(tmp)
		return err
	}
	c.changed = false
	return nil
}

// VersionCacheFile is PyxForge/tool-versions.json in the user's cache folder.
func VersionCacheFile() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "PyxForge", "tool-versions.json")
}

// DetectCached is Detect for start-up: tools whose executables have not changed since they
// were last asked report the version they gave then, without being run.
func DetectCached(ctx context.Context) []Status {
	d := New()
	d.Cache = &VersionCache{File: VersionCacheFile()}
	st := d.Detect(ctx, Tools)
	_ = d.Cache.Save()
	return st
}

// DetectFresh is Detect that asks every tool again, and remembers the answers for the next
// start-up: for Doctor, Check Toolchain and after an install.
func DetectFresh(ctx context.Context) []Status {
	d := New()
	d.Cache = &VersionCache{File: VersionCacheFile()}
	d.Refresh = true
	st := d.Detect(ctx, Tools)
	_ = d.Cache.Save()
	return st
}
