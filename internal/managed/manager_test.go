package managed

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// zipOf builds an archive holding files (slash-separated name → body).
func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(files[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// server serves archives by path, honouring Range requests, and counts the requests.
func server(t *testing.T, files map[string][]byte) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, filepath.Base(r.URL.Path), testTime, bytes.NewReader(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newManager(t *testing.T) *Manager {
	t.Helper()
	d := t.TempDir()
	return &Manager{Root: filepath.Join(d, "toolchains"), Downloads: filepath.Join(d, "downloads"), LogFile: filepath.Join(d, "logs", "toolchains.log")}
}

func pkg(url string, archive []byte) Package {
	return Package{ID: "tools", Name: "Test tools", Version: "1.0", GOOS: "windows", GOARCH: "amd64", URL: url,
		SHA256: sum(archive), Size: int64(len(archive)), Bin: []string{"tc/bin"}, Requires: []string{"tc/bin/cc.exe"}}
}

func TestInstallVerifyUninstall(t *testing.T) {
	v1 := zipOf(t, map[string]string{"tc/bin/cc.exe": "one", "tc/README": "hello"})
	srv, _ := server(t, map[string][]byte{"/v1.zip": v1})
	m := newManager(t)
	p := pkg(srv.URL+"/v1.zip", v1)

	if st := m.Status(p); st.State != NotInstalled || len(m.BinDirs()) != 0 {
		t.Fatalf("before install: %v, bins %v", st.State, m.BinDirs())
	}
	var phases []string
	if err := m.Install(context.Background(), p, func(pr Progress) {
		if len(phases) == 0 || phases[len(phases)-1] != pr.Phase {
			phases = append(phases, pr.Phase)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(phases, "Verifying") || !slices.Contains(phases, "Unpacking") || phases[len(phases)-1] != "Installing" {
		t.Errorf("phases %v", phases)
	}
	if got, _ := os.ReadFile(filepath.Join(m.Dir("tools"), "tc", "bin", "cc.exe")); string(got) != "one" {
		t.Errorf("installed file %q", got)
	}
	if st := m.Status(p); st.State != Installed || st.Receipt.Version != "1.0" || st.Receipt.SHA256 != p.SHA256 {
		t.Errorf("after install: %+v", st)
	}
	if err := m.Verify(p); err != nil {
		t.Error(err)
	}
	// The archive is not kept once installed; nothing is left beside the install.
	if left, _ := os.ReadDir(m.Downloads); len(left) != 0 {
		t.Errorf("downloads left: %v", left)
	}
	if entries, _ := os.ReadDir(m.Root); len(entries) != 2 { // tools/ and tools.json
		t.Errorf("root holds %v", entries)
	}
	if log, _ := os.ReadFile(m.LogFile); !strings.Contains(string(log), "installed tools 1.0") {
		t.Errorf("log %q", log)
	}

	// A file the install needs goes missing: it needs repair, and installing again repairs it.
	if err := os.Remove(filepath.Join(m.Dir("tools"), "tc", "bin", "cc.exe")); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(p); st.State != Broken || st.Problem == nil || m.Verify(p) == nil {
		t.Errorf("after deleting a file: %+v", st)
	}
	if err := m.Install(context.Background(), p, nil); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(p); st.State != Installed {
		t.Errorf("after repair: %v %v", st.State, st.Problem)
	}

	// Another version pinned by a newer PyxForge shows as an update.
	newer := p
	newer.Version = "2.0"
	if st := m.Status(newer); st.State != UpdateAvailable {
		t.Errorf("with a newer pin: %v", st.State)
	}

	if err := m.Uninstall(p); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(p); st.State != NotInstalled || len(m.BinDirs()) != 0 {
		t.Errorf("after uninstall: %v", st.State)
	}
	if _, err := os.Stat(m.Dir("tools")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("install folder left: %v", err)
	}
}

func TestChecksumMismatchInstallsNothing(t *testing.T) {
	good := zipOf(t, map[string]string{"tc/bin/cc.exe": "one"})
	evil := zipOf(t, map[string]string{"tc/bin/cc.exe": "tampered"})
	srv, _ := server(t, map[string][]byte{"/a.zip": evil})
	m := newManager(t)
	p := pkg(srv.URL+"/a.zip", good) // pinned to the good archive's checksum
	err := m.Install(context.Background(), p, nil)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("install of a tampered archive: %v", err)
	}
	if st := m.Status(p); st.State != NotInstalled {
		t.Errorf("state %v", st.State)
	}
	if _, err := os.Stat(m.Dir("tools")); !errors.Is(err, os.ErrNotExist) {
		t.Error("something was unpacked")
	}
	if left, _ := os.ReadDir(m.Downloads); len(left) != 0 {
		t.Errorf("the bad download was kept: %v", left)
	}
}

func TestArchiveCannotEscape(t *testing.T) {
	for _, name := range []string{"../escape.txt", "tc/../../escape.txt", "/abs.txt", `..\escape.txt`, "C:/x.txt"} {
		t.Run(name, func(t *testing.T) {
			bad := zipOf(t, map[string]string{"tc/bin/cc.exe": "one", name: "x"})
			srv, _ := server(t, map[string][]byte{"/b.zip": bad})
			m := newManager(t)
			err := m.Install(context.Background(), pkg(srv.URL+"/b.zip", bad), nil)
			if err == nil || !strings.Contains(err.Error(), "outside its folder") {
				t.Fatalf("install: %v", err)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(m.Root), "escape.txt")); err == nil {
				t.Fatal("a file was written outside the install")
			}
			if st := m.Status(pkg(srv.URL, bad)); st.State != NotInstalled {
				t.Errorf("state %v", st.State)
			}
		})
	}
}

func TestDownloadContinuesWherePartLeftOff(t *testing.T) {
	archive := zipOf(t, map[string]string{"tc/bin/cc.exe": strings.Repeat("x", 300_000)})
	var ranges []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		http.ServeContent(w, r, "c.zip", testTime, bytes.NewReader(archive))
	}))
	defer srv.Close()
	m := newManager(t)
	p := pkg(srv.URL+"/c.zip", archive)
	if err := os.MkdirAll(m.Downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	half := len(archive) / 2
	if err := os.WriteFile(filepath.Join(m.Downloads, p.SHA256+".zip.part"), archive[:half], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Install(context.Background(), p, nil); err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 || ranges[0] != "bytes="+strconv.Itoa(half)+"-" {
		t.Errorf("requests %q, want one for the rest", ranges)
	}
	if st := m.Status(p); st.State != Installed {
		t.Errorf("state %v", st.State)
	}
}

func TestCancelledInstallKeepsThePreviousOne(t *testing.T) {
	v1 := zipOf(t, map[string]string{"tc/bin/cc.exe": "one"})
	v2 := zipOf(t, map[string]string{"tc/bin/cc.exe": "two"})
	srv, _ := server(t, map[string][]byte{"/v1.zip": v1, "/v2.zip": v2})
	m := newManager(t)
	p1 := pkg(srv.URL+"/v1.zip", v1)
	if err := m.Install(context.Background(), p1, nil); err != nil {
		t.Fatal(err)
	}
	p2 := pkg(srv.URL+"/v2.zip", v2)
	p2.Version = "2.0"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.Install(ctx, p2, nil); err == nil {
		t.Fatal("a cancelled install succeeded")
	}
	if got, _ := os.ReadFile(filepath.Join(m.Dir("tools"), "tc", "bin", "cc.exe")); string(got) != "one" {
		t.Errorf("the previous install changed: %q", got)
	}
	if r, _ := m.Receipt("tools"); r.Version != "1.0" {
		t.Errorf("receipt %+v", r)
	}
	if err := m.Install(context.Background(), p2, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(m.Dir("tools"), "tc", "bin", "cc.exe")); string(got) != "two" {
		t.Errorf("after the update: %q", got)
	}
	if _, err := os.Stat(m.Dir("tools") + ".old"); !errors.Is(err, os.ErrNotExist) {
		t.Error("the previous install was left beside the new one")
	}
}

func TestMissingFileInArchiveIsRefused(t *testing.T) {
	archive := zipOf(t, map[string]string{"tc/README": "no programs"})
	srv, _ := server(t, map[string][]byte{"/d.zip": archive})
	m := newManager(t)
	err := m.Install(context.Background(), pkg(srv.URL+"/d.zip", archive), nil)
	if err == nil || !strings.Contains(err.Error(), "cc.exe") {
		t.Fatalf("install: %v", err)
	}
	if st := m.Status(pkg(srv.URL, archive)); st.State != NotInstalled {
		t.Errorf("state %v", st.State)
	}
}

func TestOneInstallAtATime(t *testing.T) {
	m := newManager(t)
	if err := m.claim("tools"); err != nil {
		t.Fatal(err)
	}
	if err := m.Install(context.Background(), Package{ID: "tools"}, nil); !errors.Is(err, ErrBusy) {
		t.Errorf("second install: %v", err)
	}
	if err := m.Uninstall(Package{ID: "tools"}); !errors.Is(err, ErrBusy) {
		t.Errorf("uninstall during install: %v", err)
	}
	m.release("tools")
}

// TestManifest: every package is pinned with a checksum and an HTTPS URL, names the programs it
// must hold, and has a unique ID per platform.
func TestManifest(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Packages {
		key := p.GOOS + "/" + p.GOARCH + "/" + p.ID
		if seen[key] {
			t.Errorf("%s listed twice", key)
		}
		seen[key] = true
		if len(p.SHA256) != 64 || !strings.HasPrefix(p.URL, "https://") || p.Size <= 0 || p.Version == "" ||
			len(p.Bin) == 0 || len(p.Requires) == 0 || len(p.Provides) == 0 || p.License == "" || p.Source == "" {
			t.Errorf("%s is not fully pinned: %+v", p.ID, p)
		}
	}
	if got := forPlatform("windows", "amd64"); len(got) == 0 {
		t.Error("nothing for Windows x64")
	}
}
