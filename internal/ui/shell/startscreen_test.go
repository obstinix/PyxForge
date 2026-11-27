package shell

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

func TestBroadFolder(t *testing.T) {
	wd, _ := os.Getwd()
	root := filepath.VolumeName(wd) + string(filepath.Separator)
	if _, broad := broadFolder(root); !broad {
		t.Errorf("%s is not broad", root)
	}
	if home, err := os.UserHomeDir(); err == nil {
		if what, broad := broadFolder(home); !broad || what != "your whole home folder" {
			t.Errorf("home: %q %v", what, broad)
		}
	}
	if _, broad := broadFolder(t.TempDir()); broad {
		t.Error("a project folder is broad")
	}
}

// TestBroadFolderAsksAndEnterCancels: opening the home folder asks first, and Enter, the
// default, cancels (owner review F1).
func TestBroadFolderAsksAndEnterCancels(t *testing.T) {
	s, _, q := newEditorShell(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	s.lastDialog = nil
	s.openFolderAt(home)
	q.pumpUntil(t, "the question", func() bool { return s.lastDialog != nil })
	if findButton(s.win.Canvas().Overlays().Top(), "Open Anyway") == nil {
		t.Fatal("no Open Anyway button")
	}
	s.modalKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if len(s.modals) != 0 || s.switching {
		t.Fatalf("Enter did not cancel: %d dialogs, switching %v", len(s.modals), s.switching)
	}
}

func TestReopenFolder(t *testing.T) {
	s, root := newTestShell(t)
	p := s.app.Preferences()
	gone := filepath.Join(root, "gone")
	p.SetStringList(prefRecentFolders, []string{root, gone})
	if got := ReopenFolder(s.app); got != "" {
		t.Fatalf("off, reopened %q", got)
	}
	p.SetBool(prefReopenLast, true)
	if got := ReopenFolder(s.app); got != root {
		t.Fatalf("reopened %q, want %q", got, root)
	}
	p.SetStringList(prefRecentFolders, []string{gone, root})
	if got := ReopenFolder(s.app); got != "" {
		t.Fatalf("the last folder is gone, but %q was reopened", got)
	}
}

// TestWelcomePinsAndLocates: a pinned folder comes first; a folder that is gone offers Locate….
func TestWelcomePinsAndLocates(t *testing.T) {
	s, root := newTestShell(t)
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	os.Mkdir(a, 0o755)
	s.app.Preferences().SetStringList(prefRecentFolders, []string{a, b})
	s.togglePin(b)
	if got := s.welcomeFolders(); len(got) != 2 || got[0] != b {
		t.Fatalf("folders %q, want %s first", got, b)
	}
	if findButton(s.recentRow(b), "Locate…") == nil {
		t.Error("a missing folder offers no Locate…")
	}
	if findButton(s.recentRow(a), "Locate…") != nil {
		t.Error("a folder that is there offers Locate…")
	}
	s.togglePin(b)
	if s.isPinned(b) {
		t.Error("unpin failed")
	}
}

// TestDropOpensFiles: a file dropped on the window opens in the editor.
func TestDropOpensFiles(t *testing.T) {
	s, root, _ := newEditorShell(t)
	f := filepath.Join(root, "notes.txt")
	os.WriteFile(f, []byte("hi\n"), 0o644)
	s.dropped(fyne.Position{}, []fyne.URI{storage.NewFileURI(f)})
	if d := s.activeDoc(); d == nil || !sameFile(d.path, f) {
		t.Fatalf("the dropped file did not open")
	}
}

// TestFilesSettings: the hidden folders set in Settings apply to the Explorer.
func TestFilesSettings(t *testing.T) {
	s, root := newTestShell(t)
	os.MkdirAll(filepath.Join(root, "out"), 0o755)
	defer func() { s.app.Preferences().SetString(prefHiddenFolders, ""); s.applyFilePrefs() }()
	s.app.Preferences().SetString(prefHiddenFolders, "out,.git")
	s.applyFilePrefs()
	s.explorer.Reload()
	files, _ := s.explorer.Files(10)
	for _, f := range files {
		if filepath.Dir(f) == "out" {
			t.Fatalf("a hidden folder was listed: %q", files)
		}
	}
	s.OpenSettingsAt("files")
	if s.settingsUI == nil || s.settingsUI.page("files") == nil {
		t.Fatal("no Files & Explorer page")
	}
}
