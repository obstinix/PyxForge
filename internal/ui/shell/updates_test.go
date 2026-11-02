package shell

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/buildinfo"
	"github.com/obstinix/PyxForge/internal/update"
)

// TestCheckForUpdates drives Help > Check for Updates… against a GitHub-shaped server: with
// nothing newer it says so; with a newer release it shows the notes, downloads the installer
// (checked against the release's SHA256SUMS) and, on Windows, starts it only after asking.
func TestCheckForUpdates(t *testing.T) {
	data := t.TempDir()
	t.Setenv("LOCALAPPDATA", data)
	t.Setenv("XDG_DATA_HOME", data)
	setup := []byte("the new installer")
	sum := sha256.Sum256(setup)
	newer := false
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"):
			var rels []update.Release
			if newer {
				rel := update.Release{Tag: "v99.0.0", Notes: "Much faster builds.", Published: time.Now().Add(time.Hour), Page: srv.URL + "/page"}
				for _, n := range []string{"PyxForge-Setup-99.0.0.exe", "PyxForge-99.0.0-linux-x64.deb"} {
					rel.Assets = append(rel.Assets, update.Asset{Name: n, URL: srv.URL + "/dl/" + n, Size: int64(len(setup))})
				}
				rel.Assets = append(rel.Assets, update.Asset{Name: "SHA256SUMS.txt", URL: srv.URL + "/dl/sums"},
					update.Asset{Name: "SHA256SUMS-linux.txt", URL: srv.URL + "/dl/sums"})
				rels = append(rels, rel)
			}
			json.NewEncoder(w).Encode(rels)
		case r.URL.Path == "/dl/sums":
			for _, n := range []string{"PyxForge-Setup-99.0.0.exe", "PyxForge-99.0.0-linux-x64.deb"} {
				w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + n + "\n"))
			}
		default:
			w.Write(setup)
		}
	}))
	defer srv.Close()
	old := update.API
	update.API = srv.URL
	t.Cleanup(func() { update.API = old })
	oldVersion := buildinfo.Version
	buildinfo.Version = "3.0.0"
	t.Cleanup(func() { buildinfo.Version = oldVersion })

	q := make(queue, 1024)
	s := NewWithOptions(test.NewTempApp(t), t.TempDir(), Options{Dispatch: q.post})
	noToastExpiry(s)
	t.Cleanup(s.Shutdown)
	page := func() string { return strings.Join(texts(s.updates.body), "\n") }

	s.Commands().Run("help.updates")
	q.pumpUntil(t, "the check", func() bool { return strings.Contains(page(), "No newer release") })
	s.updates.d.Hide()

	newer = true
	s.Commands().Run("help.updates")
	q.pumpUntil(t, "the update", func() bool { return strings.Contains(page(), "PyxForge 99.0.0 is available") })
	if !strings.Contains(page(), "Much faster builds.") {
		t.Errorf("the notes are not shown:\n%s", page())
	}
	b := findButton(s.updates.body, "Download (0 MB)")
	if b == nil {
		t.Fatalf("no Download button:\n%s", page())
	}
	test.Tap(b)
	q.pumpUntil(t, "the download", func() bool { return strings.Contains(page(), "checked against the release's SHA-256") })

	if runtime.GOOS != "windows" {
		if !strings.Contains(page(), "sudo apt install") {
			t.Errorf("no install instructions:\n%s", page())
		}
		return
	}
	var started string
	s.startInstaller = func(p string) error { started = p; return nil }
	test.Tap(findButton(s.updates.body, "Install and Restart"))
	if started != "" {
		t.Fatal("the installer started without asking")
	}
	tapInDialog(t, s, "Install")
	if b, err := os.ReadFile(started); err != nil || string(b) != string(setup) {
		t.Errorf("started %q (%v)", started, err)
	}
}
