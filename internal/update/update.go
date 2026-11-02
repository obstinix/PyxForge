// Package update checks GitHub Releases for a newer PyxForge and downloads its installer or
// package, checked against the release's SHA256SUMS. It runs only when asked (Help > Check for
// Updates…, pyxforge update): PyxForge makes no network request at start-up. It never replaces
// the running program; the installer it downloads does that, with the user watching.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Repo is where PyxForge is released.
const Repo = "obstinix/PyxForge"

// API is GitHub's REST API; tests point it at their own server.
var API = "https://api.github.com"

// Release is one published release.
type Release struct {
	Tag        string    `json:"tag_name"`
	Name       string    `json:"name"`
	Notes      string    `json:"body"`
	Page       string    `json:"html_url"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published_at"`
	Assets     []Asset   `json:"assets"`
}

// Asset is one file of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Version is the release's version: its tag without the v.
func (r Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

// Current describes the running build, for choosing what is newer.
type Current struct {
	Version string    // buildinfo.Version
	Built   time.Time // when its commit was made; zero when unknown
}

// Check lists the releases and returns the newest one that is an update for cur, or ok false
// when there is none. A release is an update when its version is higher, and when it was
// published after the running build's commit: development builds are named 3.0.0-dev.<commit>,
// which version order puts below any 3.0.0-rc, however much newer they are. Pre-releases are
// offered to a pre-release only; drafts never.
func Check(ctx context.Context, client *http.Client, cur Current) (Release, bool, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, API+"/repos/"+Repo+"/releases?per_page=30", nil)
	if err != nil {
		return Release{}, false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PyxForge/"+cur.Version)
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, false, fmt.Errorf("GitHub could not be reached: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, false, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var all []Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&all); err != nil {
		return Release{}, false, fmt.Errorf("GitHub's answer was not understood: %w", err)
	}
	return Newest(all, cur)
}

// Newest picks the update for cur from releases, as Check does.
func Newest(releases []Release, cur Current) (Release, bool, error) {
	mine, err := ParseVersion(cur.Version)
	if err != nil {
		return Release{}, false, fmt.Errorf("this build's version %q: %w", cur.Version, err)
	}
	var best Release
	var bestV Semver
	found := false
	for _, r := range releases {
		v, err := ParseVersion(r.Version())
		if err != nil || r.Draft || (r.Prerelease && mine.Pre == "") {
			continue
		}
		if Compare(v, mine) <= 0 || (!cur.Built.IsZero() && !r.Published.After(cur.Built)) {
			continue
		}
		if !found || Compare(v, bestV) > 0 {
			best, bestV, found = r, v, true
		}
	}
	return best, found, nil
}

// Installer is the file of r to install on this system: the Windows installer, or the Debian
// package on Linux; ok is false when the release has none.
func Installer(r Release) (Asset, bool) {
	var want string
	switch runtime.GOOS {
	case "windows":
		want = "PyxForge-Setup-" + r.Version()
	case "linux":
		want = "PyxForge-" + r.Version() + "-linux-x64.deb"
	default:
		return Asset{}, false
	}
	for _, a := range r.Assets {
		if strings.HasPrefix(a.Name, want) {
			return a, true
		}
	}
	return Asset{}, false
}

// sumsFile names a release's checksum list for an asset.
func sumsFile(r Release, a Asset) (Asset, bool) {
	names := []string{"SHA256SUMS.txt"}
	if strings.HasSuffix(a.Name, ".deb") || strings.HasSuffix(a.Name, ".tar.gz") {
		names = append([]string{"SHA256SUMS-linux.txt"}, names...)
	}
	for _, n := range names {
		for _, s := range r.Assets {
			if s.Name == n {
				return s, true
			}
		}
	}
	return Asset{}, false
}

// ErrNoChecksum means the release lists no SHA-256 for the file, so it is not used.
var ErrNoChecksum = errors.New("the release lists no SHA-256 for it, so PyxForge does not use it")

// Download fetches a of r into dir and checks it against the release's SHA256SUMS before
// keeping it: a file that does not match is deleted. It returns the file's path.
func Download(ctx context.Context, client *http.Client, r Release, a Asset, dir string, progress func(done, total int64)) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	sums, ok := sumsFile(r, a)
	if !ok {
		return "", ErrNoChecksum
	}
	listed, err := fetchSums(ctx, client, sums)
	if err != nil {
		return "", err
	}
	want, ok := listed[a.Name]
	if !ok {
		return "", ErrNoChecksum
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, filepath.Base(a.Name))
	tmp := dst + ".part"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", a.Name, resp.Status)
	}
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	var done int64
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				return "", werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, a.Size)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("download %s: %w", a.Name, rerr)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		os.Remove(tmp)
		return "", fmt.Errorf("%s does not match the release's SHA-256 (%s, expected %s); it was deleted", a.Name, got, want)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return dst, nil
}

// fetchSums reads a sha256sum-style list: "<hex>  <name>" per line.
func fetchSums(ctx context.Context, client *http.Client, a Asset) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", a.Name, resp.Status)
	}
	out := map[string]string{}
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for sc.Scan() {
		sum, name, ok := strings.Cut(strings.TrimSpace(sc.Text()), " ")
		if !ok || len(sum) != 64 {
			continue
		}
		out[strings.TrimPrefix(strings.TrimSpace(name), "*")] = strings.ToLower(sum)
	}
	return out, sc.Err()
}
