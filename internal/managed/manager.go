package managed

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DataDir is PyxForge's folder for what it installs and keeps per user:
// %LOCALAPPDATA%\PyxForge on Windows, $XDG_DATA_HOME/pyxforge (~/.local/share/pyxforge) elsewhere.
// An installer's upgrade or removal does not touch it.
func DataDir() (string, error) {
	if runtime.GOOS == "windows" {
		base, err := os.UserCacheDir() // %LOCALAPPDATA%
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "PyxForge"), nil
	}
	if d := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(d) {
		return filepath.Join(d, "pyxforge"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "pyxforge"), nil
}

// Receipt records an install: written last, so a package without one is not installed.
type Receipt struct {
	ID        string    `json:"id"`
	Version   string    `json:"version"`
	SHA256    string    `json:"sha256"` // of the archive it came from
	Bin       []string  `json:"bin"`
	Installed time.Time `json:"installed"`
}

// State is where a package stands.
type State int

const (
	NotInstalled State = iota
	Installed
	UpdateAvailable // another version is installed than this PyxForge pins
	Broken          // installed, but files it needs are gone
	External        // an installer package's program is there, installed by someone else
)

func (s State) String() string {
	return [...]string{"Not installed", "Installed", "Update available", "Needs repair", "Installed outside PyxForge"}[s]
}

// Status is a package and where it stands.
type Status struct {
	Package
	State   State
	Receipt Receipt // when installed
	Problem error   // why it is Broken
}

// Progress reports an install as it goes.
type Progress struct {
	Phase string // Downloading, Verifying, Unpacking, Installing
	Done  int64  // bytes or files done
	Total int64  // bytes or files in all; 0 when unknown
}

// Manager installs packages under Root.
type Manager struct {
	Root      string       // installs: Root/<id>, receipts Root/<id>.json
	Downloads string       // archives while they download
	LogFile   string       // the install log; "" for none
	Client    *http.Client // nil means http.DefaultClient
	// Elevate runs an installer with administrator rights and waits for it; nil means the
	// system's (Windows asks the user). Tests replace it.
	Elevate func(ctx context.Context, exe string, args []string) error
	Getenv  func(string) string // nil means os.Getenv; expands InstallDir

	mu   sync.Mutex
	busy map[string]bool
}

// Default is the manager for this user's data folder.
func Default() (*Manager, error) {
	d, err := DataDir()
	if err != nil {
		return nil, err
	}
	return &Manager{Root: filepath.Join(d, "toolchains"), Downloads: filepath.Join(d, "cache", "downloads"),
		LogFile: filepath.Join(d, "logs", "toolchains.log")}, nil
}

// Dir is where an archive package id is unpacked.
func (m *Manager) Dir(id string) string { return filepath.Join(m.Root, id) }

// PackageDir is where p's files are: its folder under Root, or an installer's InstallDir; ""
// when the InstallDir names a variable this system does not set.
func (m *Manager) PackageDir(p Package) string {
	if !p.IsInstaller() {
		return m.Dir(p.ID)
	}
	getenv := m.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	unset := false
	dir := filepath.Clean(os.Expand(p.InstallDir, func(k string) string {
		v := getenv(k)
		unset = unset || v == ""
		return v
	}))
	if unset || !filepath.IsAbs(dir) { // on Linux an unset ${ProgramFiles} would leave /qemu
		return ""
	}
	return dir
}

func (m *Manager) receiptPath(id string) string { return filepath.Join(m.Root, id+".json") }

// Receipt reads id's receipt; ok is false when it is not installed.
func (m *Manager) Receipt(id string) (Receipt, bool) {
	data, err := os.ReadFile(m.receiptPath(id))
	if err != nil {
		return Receipt{}, false
	}
	var r Receipt
	if json.Unmarshal(data, &r) != nil || r.ID != id {
		return Receipt{}, false
	}
	return r, true
}

// BinDirs are the program folders of every installed package of this platform, in manifest
// order: what tool lookups and the PATH of started programs add.
func (m *Manager) BinDirs() []string {
	var out []string
	for _, p := range ForPlatform() {
		r, ok := m.Receipt(p.ID)
		if !ok {
			continue
		}
		dir := m.PackageDir(p)
		if dir == "" {
			continue
		}
		for _, b := range r.Bin {
			out = append(out, filepath.Join(dir, filepath.FromSlash(b)))
		}
	}
	return out
}

// Status reports where p stands.
func (m *Manager) Status(p Package) Status {
	st := Status{Package: p}
	r, ok := m.Receipt(p.ID)
	if !ok {
		if p.IsInstaller() && m.check(p, m.PackageDir(p)) == nil {
			st.State = External // PyxForge uses it, and leaves it alone
		}
		return st
	}
	st.Receipt, st.State = r, Installed
	if err := m.check(p, m.PackageDir(p)); err != nil {
		st.State, st.Problem = Broken, err
	} else if r.Version != p.Version || r.SHA256 != p.SHA256 {
		st.State = UpdateAvailable
	}
	return st
}

// Verify checks that an installed package still has the files it needs.
func (m *Manager) Verify(p Package) error {
	if _, ok := m.Receipt(p.ID); !ok {
		return fmt.Errorf("%s is not installed", p.Name)
	}
	return m.check(p, m.PackageDir(p))
}

func (m *Manager) check(p Package, dir string) error {
	if dir == "" {
		return fmt.Errorf("%s has no install folder on this system", p.Name)
	}
	for _, f := range p.Requires {
		fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil || !fi.Mode().IsRegular() {
			return fmt.Errorf("%s is missing %s; repair it", p.Name, f)
		}
	}
	return nil
}

// ErrBusy means the package is being installed or removed already.
var ErrBusy = errors.New("already being installed or removed")

func (m *Manager) claim(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busy[id] {
		return fmt.Errorf("%s: %w", id, ErrBusy)
	}
	if m.busy == nil {
		m.busy = map[string]bool{}
	}
	m.busy[id] = true
	return nil
}

func (m *Manager) release(id string) {
	m.mu.Lock()
	delete(m.busy, id)
	m.mu.Unlock()
}

// Install downloads p, checks its SHA-256, unpacks it beside the old install and then swaps it
// in, so an install that fails or is cancelled leaves what was there. Installing again repairs
// or updates. progress may be nil; it is called from this goroutine.
func (m *Manager) Install(ctx context.Context, p Package, progress func(Progress)) (err error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	if err := m.claim(p.ID); err != nil {
		return err
	}
	defer m.release(p.ID)
	m.logf("install %s %s from %s", p.ID, p.Version, p.URL)
	defer func() {
		if err != nil {
			m.logf("install %s failed: %v", p.ID, err)
		} else {
			m.logf("installed %s %s in %s", p.ID, p.Version, m.Dir(p.ID))
		}
	}()

	if _, mine := m.Receipt(p.ID); p.IsInstaller() && !mine && m.check(p, m.PackageDir(p)) == nil {
		return fmt.Errorf("%s is already installed in %s, by someone other than PyxForge; PyxForge uses it as it is", p.Name, m.PackageDir(p))
	}
	archive, err := m.fetch(ctx, p, progress)
	if err != nil {
		return err
	}
	if p.IsInstaller() {
		return m.runInstaller(ctx, p, archive, progress)
	}
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(m.Root, p.ID+".staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage) // empty after a swap; a failed unpack's files otherwise
	if err := unzip(ctx, archive, stage, func(done, total int64) {
		progress(Progress{Phase: "Unpacking", Done: done, Total: total})
	}); err != nil {
		return err
	}
	if err := m.check(p, stage); err != nil {
		return fmt.Errorf("the download does not hold what %s should: %w", p.Name, err)
	}
	progress(Progress{Phase: "Installing"})
	dir, old := m.Dir(p.ID), m.Dir(p.ID)+".old"
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := renameRetry(dir, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("cannot replace the installed %s (is one of its programs running?): %w", p.Name, err)
	}
	if err := renameRetry(stage, dir); err != nil {
		_ = renameRetry(old, dir) // put the previous install back
		return err
	}
	if err := writeAtomic(m.receiptPath(p.ID), Receipt{ID: p.ID, Version: p.Version, SHA256: p.SHA256, Bin: p.Bin, Installed: time.Now().UTC()}); err != nil {
		return err
	}
	_ = os.RemoveAll(old)
	_ = os.Remove(archive) // installed: the archive only takes space now; a repair downloads it again
	return nil
}

// Uninstall removes an installed package: its receipt first, so a half-removed folder is never
// taken for an install.
func (m *Manager) Uninstall(p Package) error {
	if err := m.claim(p.ID); err != nil {
		return err
	}
	defer m.release(p.ID)
	if p.IsInstaller() {
		return m.runUninstaller(p)
	}
	if err := os.Remove(m.receiptPath(p.ID)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.RemoveAll(m.Dir(p.ID)); err != nil {
		return fmt.Errorf("could not remove every file of %s (is one of its programs running?): %w", p.Name, err)
	}
	m.logf("uninstalled %s", p.ID)
	return nil
}

// fetch returns p's archive in Downloads, downloading what is missing of it, and checks its
// SHA-256. A partial download from before is continued where the server allows it.
func (m *Manager) fetch(ctx context.Context, p Package, progress func(Progress)) (string, error) {
	if err := os.MkdirAll(m.Downloads, 0o755); err != nil {
		return "", err
	}
	ext := path.Ext(p.URL)
	if ext == "" {
		ext = ".zip"
	}
	final := filepath.Join(m.Downloads, p.SHA256+ext)
	if sum, err := fileSHA256(final); err == nil && sum == p.SHA256 {
		return final, nil
	}
	part := final + ".part"
	have := int64(0)
	if fi, err := os.Stat(part); err == nil {
		have = fi.Size()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "PyxForge")
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
	}
	client := m.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", p.Name, err)
	}
	defer resp.Body.Close()
	flags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		flags |= os.O_TRUNC // the server sends it all: start again
		have = 0
	case http.StatusRequestedRangeNotSatisfiable:
		_ = os.Remove(part) // what was kept is no prefix of the file
		return "", fmt.Errorf("download %s: the partial download did not match; try again", p.Name)
	default:
		return "", fmt.Errorf("download %s: %s", p.Name, resp.Status)
	}
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return "", err
	}
	total := p.Size
	if resp.ContentLength > 0 {
		total = have + resp.ContentLength
	}
	w := &counter{w: f, n: have, report: func(n int64) { progress(Progress{Phase: "Downloading", Done: n, Total: total}) }}
	_, err = io.Copy(w, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("download %s: %w", p.Name, err) // the part is kept, to continue later
	}
	progress(Progress{Phase: "Verifying"})
	sum, err := fileSHA256(part)
	if err != nil {
		return "", err
	}
	if sum != p.SHA256 {
		_ = os.Remove(part)
		return "", fmt.Errorf("download %s: its SHA-256 is %s, not the %s this release expects; nothing was installed", p.Name, sum, p.SHA256)
	}
	if err := os.Rename(part, final); err != nil {
		return "", err
	}
	return final, nil
}

// renameRetry renames a folder, trying again for a while on Windows when it is refused: an
// antivirus scanner reads the files just unpacked, and a folder cannot be renamed while one of
// its files is open (seen with the 12,000 files of the GCC toolchain).
func renameRetry(from, to string) error {
	delay := 100 * time.Millisecond
	deadline := time.Now().Add(30 * time.Second)
	for {
		err := os.Rename(from, to)
		if err == nil || runtime.GOOS != "windows" || !errors.Is(err, fs.ErrPermission) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		delay = min(2*delay, 2*time.Second)
	}
}

// counter counts bytes written and reports them at most every 64 KiB.
type counter struct {
	w      io.Writer
	n      int64
	last   int64
	report func(int64)
}

func (c *counter) Write(b []byte) (int, error) {
	n, err := c.w.Write(b)
	c.n += int64(n)
	if c.n-c.last >= 64<<10 {
		c.last = c.n
		c.report(c.n)
	}
	return n, err
}

func fileSHA256(name string) (string, error) {
	f, err := os.Open(name)
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

// unzip unpacks archive into dir. An entry naming a path outside dir, an absolute path or a
// symbolic link makes it fail: an archive cannot write anywhere else.
func unzip(ctx context.Context, archive, dir string, report func(done, total int64)) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	total := int64(len(zr.File))
	for i, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := path.Clean(strings.ReplaceAll(f.Name, `\`, "/"))
		if name == "." || path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, ":") {
			return fmt.Errorf("the archive has an entry outside its folder: %q", f.Name)
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		mode := f.Mode()
		switch {
		case mode&fs.ModeSymlink != 0:
			return fmt.Errorf("the archive has a symbolic link, which PyxForge does not unpack: %q", f.Name)
		case f.FileInfo().IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := extract(f, target, mode.Perm()|0o600); err != nil {
				return err
			}
		}
		if i%64 == 0 || int64(i+1) == total {
			report(int64(i+1), total)
		}
	}
	return nil
}

func extract(f *zip.File, target string, perm fs.FileMode) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	return err
}

func writeAtomic(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".receipt-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func (m *Manager) logf(format string, args ...any) {
	if m.LogFile == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.LogFile), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(m.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
