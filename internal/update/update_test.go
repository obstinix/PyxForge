package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestCompare checks SemVer 2.0 precedence, including the example ordering of its section 11.
func TestCompare(t *testing.T) {
	order := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2",
		"1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0"}
	for i := range order {
		for j := range order {
			a, _ := ParseVersion(order[i])
			b, _ := ParseVersion(order[j])
			want := sign(i - j)
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", order[i], order[j], got, want)
			}
		}
	}
	if v, err := ParseVersion("v3.0.0-dev.9237c75+local"); err != nil || v.Pre != "dev.9237c75" || v.Major != 3 {
		t.Errorf("ParseVersion: %+v %v", v, err)
	}
	for _, bad := range []string{"", "3.0", "3.0.0.1", "03.0.0", "3.0.x", "3.0.0-"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func day(n int) time.Time { return time.Date(2026, 10, n, 0, 0, 0, 0, time.UTC) }

// TestNewest: the update is the highest version published after this build's commit; a
// development build is not offered an older release candidate that version order puts above
// it; a release build is not offered pre-releases; drafts never.
func TestNewest(t *testing.T) {
	rels := []Release{
		{Tag: "v3.0.0-rc.1", Prerelease: true, Published: day(20)},
		{Tag: "v3.0.0-rc.2", Prerelease: true, Published: day(20)},
		{Tag: "v3.0.0-dev.9237c75", Prerelease: true, Published: day(25)},
		{Tag: "v3.0.0-rc.3", Prerelease: true, Published: day(30), Draft: true},
	}
	// The dev build of the 21st: rc.2 is higher by version but older; nothing to offer.
	if r, ok, err := Newest(rels, Current{Version: "3.0.0-dev.9237c75", Built: day(21)}); err != nil || ok {
		t.Errorf("dev build: offered %s (%v)", r.Tag, err)
	}
	// rc.1, built on the 19th: rc.2 is an update (the draft rc.3 is not).
	if r, ok, _ := Newest(rels, Current{Version: "3.0.0-rc.1", Built: day(19)}); !ok || r.Tag != "v3.0.0-rc.2" {
		t.Errorf("rc.1: offered %q %v", r.Tag, ok)
	}
	// A release build is not offered pre-releases; 3.0.1 is an update.
	rels = append(rels, Release{Tag: "v3.0.1", Published: day(28)})
	if r, ok, _ := Newest(rels, Current{Version: "3.0.0", Built: day(22)}); !ok || r.Tag != "v3.0.1" {
		t.Errorf("3.0.0: offered %q %v", r.Tag, ok)
	}
	if _, ok, _ := Newest(rels, Current{Version: "3.0.1"}); ok {
		t.Error("the newest version was offered itself")
	}
	if _, _, err := Newest(rels, Current{Version: "not a version"}); err == nil {
		t.Error("a bad current version was accepted")
	}
}

// TestCheckAndDownload runs the check against a GitHub-shaped server and downloads the
// installer, checked against the release's SHA256SUMS; a file that does not match is deleted.
func TestCheckAndDownload(t *testing.T) {
	setup := []byte("installer bytes")
	sum := sha256.Sum256(setup)
	name := map[string]string{"windows": "PyxForge-Setup-3.1.0.exe", "linux": "PyxForge-3.1.0-linux-x64.deb"}
	sumsName := map[string]string{"windows": "SHA256SUMS.txt", "linux": "SHA256SUMS-linux.txt"}
	var srv *httptest.Server
	tamper := false
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"+Repo+"/releases"):
			rel := Release{Tag: "v3.1.0", Name: "PyxForge 3.1.0", Notes: "Faster builds.", Published: day(30), Page: srv.URL + "/page"}
			for _, os := range []string{"windows", "linux"} {
				rel.Assets = append(rel.Assets, Asset{Name: name[os], URL: srv.URL + "/dl/" + name[os], Size: int64(len(setup))},
					Asset{Name: sumsName[os], URL: srv.URL + "/dl/" + sumsName[os]})
			}
			json.NewEncoder(w).Encode([]Release{rel})
		case strings.HasPrefix(r.URL.Path, "/dl/SHA256SUMS"):
			for _, n := range name {
				w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + n + "\n"))
			}
		case strings.HasPrefix(r.URL.Path, "/dl/"):
			if tamper {
				w.Write([]byte("something else"))
				return
			}
			w.Write(setup)
		}
	}))
	defer srv.Close()
	old := API
	API = srv.URL
	t.Cleanup(func() { API = old })

	rel, ok, err := Check(context.Background(), nil, Current{Version: "3.0.0", Built: day(1)})
	if err != nil || !ok || rel.Version() != "3.1.0" || rel.Notes != "Faster builds." {
		t.Fatalf("Check: %+v %v %v", rel, ok, err)
	}
	a, ok := Installer(rel)
	if !ok {
		t.Skip("no installer for this system")
	}
	dir := t.TempDir()
	var seen int64
	p, err := Download(context.Background(), nil, rel, a, dir, func(done, total int64) { seen = done })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != string(setup) || seen != int64(len(setup)) {
		t.Errorf("downloaded %q, progress %d", b, seen)
	}
	tamper = true
	dir2 := t.TempDir()
	if _, err := Download(context.Background(), nil, rel, a, dir2, nil); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Errorf("a tampered download: %v", err)
	}
	if left, _ := os.ReadDir(dir2); len(left) != 0 {
		t.Errorf("the tampered file was kept: %v", left)
	}
	noSums := rel
	noSums.Assets = []Asset{a}
	if _, err := Download(context.Background(), nil, noSums, a, t.TempDir(), nil); err != ErrNoChecksum {
		t.Errorf("without SHA256SUMS: %v", err)
	}

}
