//go:build ci

package shell

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"fyne.io/fyne/v2/app"
)

// The ci driver is headless but uses Fyne's real file-backed preferences. Separate
// processes prove persistence, rather than just reusing an in-memory test App.
func TestWelcomePreferencesAcrossProcesses(t *testing.T) {
	const id = "io.github.obstinix.pyxforge.welcome-test"
	if mode := os.Getenv("PYXFORGE_PREF_TEST_MODE"); mode != "" {
		a := app.NewWithID(id)
		s := NewWithOptions(a, "", Options{})
		noToastExpiry(s)
		defer s.Shutdown()
		project := os.Getenv("PYXFORGE_PREF_TEST_PROJECT")
		if mode == "read" {
			if s.welcome != nil || !slices.Equal(a.Preferences().StringList(prefRecentFolders), []string{project}) {
				t.Fatal("new process did not restore Welcome preference and recent projects")
			}
			return
		}
		s.addRecent(prefRecentFolders, project)
		a.Preferences().SetBool(prefWelcome, false)
		path := filepath.Join(os.TempDir(), "fyne-test", id, "preferences.json")
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			b, _ := os.ReadFile(path)
			var saved map[string]json.RawMessage
			if json.Unmarshal(b, &saved) == nil && string(saved[prefWelcome]) == "false" && len(saved[prefRecentFolders]) > 2 {
				return
			}
		}
		t.Fatal("preferences were not written to disk")
	}
	root := t.TempDir()
	for _, key := range []string{"TEMP", "TMP", "TMPDIR", "APPDATA", "XDG_CONFIG_HOME"} {
		t.Setenv(key, root)
	}
	t.Setenv("PYXFORGE_PREF_TEST_PROJECT", root)
	for _, mode := range []string{"write", "read"} {
		t.Setenv("PYXFORGE_PREF_TEST_MODE", mode)
		cmd := exec.Command(os.Args[0], "-test.run=^TestWelcomePreferencesAcrossProcesses$")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", mode, err, out)
		}
	}
}
