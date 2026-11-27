package shell

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/obstinix/PyxForge/internal/ui/codeedit"
	"github.com/obstinix/PyxForge/internal/ui/explorer"
	"github.com/obstinix/PyxForge/internal/ui/icons"
	"github.com/obstinix/PyxForge/internal/ui/kit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

// Settings > Files & Explorer (owner review F1, F4, F6).
const (
	prefHiddenFolders = "files.hidden"      // comma-separated folder names never listed
	prefLargeMB       = "files.largeMB"     // large-file mode above this many MB
	prefReadOnlyMB    = "files.readOnlyMB"  // read-only above
	prefHexOnlyMB     = "files.hexOnlyMB"   // hex view only above
	prefShowLimit     = "files.folderLimit" // entries a folder lists before Show all
)

// applyFilePrefs puts the Files & Explorer settings in force.
func (s *Shell) applyFilePrefs() {
	p := s.app.Preferences()
	hidden := explorer.DefaultHidden
	if v := p.String(prefHiddenFolders); v != "" {
		hidden = splitNames(v)
	}
	explorer.SetHidden(hidden)
	codeedit.LargeFileBytes = p.IntWithFallback(prefLargeMB, 2) << 20
	readOnlyBytes = int64(p.IntWithFallback(prefReadOnlyMB, 50)) << 20
	hexOnlyBytes = int64(p.IntWithFallback(prefHexOnlyMB, 512)) << 20
	if s.explorer != nil {
		s.explorer.ShowLimit = p.IntWithFallback(prefShowLimit, explorer.DefaultShowLimit)
	}
}

func splitNames(v string) []string {
	var out []string
	for _, n := range strings.Split(v, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func (m *settingsModal) filesPage() *settingsPage {
	s := m.s
	prefs := s.app.Preferences()
	p := &settingsPage{id: "files", title: "Files & Explorer", icon: icons.FolderTree}

	reopen := kit.NewSwitch(prefs.Bool(prefReopenLast), func(on bool) { prefs.SetBool(prefReopenLast, on) })
	reopen.Label = "Reopen last workspace"

	hidden := widget.NewEntry()
	hidden.SetText(strings.Join(splitNamesOr(prefs.String(prefHiddenFolders), explorer.DefaultHidden), ", "))
	applyHidden := func() {
		prefs.SetString(prefHiddenFolders, strings.Join(splitNames(hidden.Text), ","))
		s.applyFilePrefs()
		s.explorer.Reload()
	}
	hidden.OnSubmitted = func(string) { applyHidden() }
	hiddenRow := container.NewBorder(nil, nil, nil, kit.NewButton("Apply", applyHidden), hidden)
	hiddenRow.Resize(fyne.NewSize(320, hiddenRow.MinSize().Height))

	size := func(pref string, def int, options []int, label string) *kit.Dropdown {
		var names []string
		for _, o := range options {
			names = append(names, mb(o))
		}
		d := kit.NewDropdown(names, mb(prefs.IntWithFallback(pref, def)), func(v string) {
			n, _ := strconv.Atoi(strings.Fields(v)[0])
			if strings.HasSuffix(v, "GB") {
				n <<= 10
			}
			prefs.SetInt(pref, n)
			s.applyFilePrefs()
		})
		d.Label = label
		return d
	}
	limits := []string{"200", "500", "1000", "5000"}
	limit := kit.NewDropdown(limits, strconv.Itoa(prefs.IntWithFallback(prefShowLimit, explorer.DefaultShowLimit)), func(v string) {
		n, _ := strconv.Atoi(v)
		prefs.SetInt(prefShowLimit, n)
		s.applyFilePrefs()
		s.explorer.Reload()
	})
	limit.Label = "Entries per folder"

	p.content = container.New(layout.NewCustomPaddedVBoxLayout(theme.Space6),
		kit.NewSettingGroup("Start-up",
			m.row(p, prefReopenLast, "Reopen last workspace",
				"At start, opens the folder you last worked in instead of the Welcome page. A drive's root or the home folder is still asked about first.",
				reopen, "restore", "session", "startup", "recent"),
		),
		kit.NewSettingGroup("Explorer",
			m.row(p, prefHiddenFolders, "Hidden folders",
				"Folders with these names are never listed in the Explorer or searched by Go to File. Separate names with commas.",
				container.New(&fixedWidth{w: 320}, hiddenRow), "ignore", "exclude", ".git", "node_modules", "target"),
			m.row(p, prefShowLimit, "Entries per folder",
				"A folder lists this many entries, then a Show all row, so a folder of thousands opens at once.",
				limit, "limit", "large folder"),
		),
		kit.NewSettingGroup("Large files",
			m.row(p, prefLargeMB, "Large-file mode above",
				"Larger files open without syntax colours or a language server, and say Large file in the status bar.",
				size(prefLargeMB, 2, []int{1, 2, 5, 10, 20}, "Large-file mode"), "performance", "highlight", "lsp"),
			m.row(p, prefReadOnlyMB, "Read-only above",
				"Larger text files open read-only: for reading logs and dumps, not editing them.",
				size(prefReadOnlyMB, 50, []int{20, 50, 100, 200}, "Read-only"), "log", "performance"),
			m.row(p, prefHexOnlyMB, "Hex view only above",
				"Larger files of any kind open in the hex view, which reads only the rows on screen.",
				size(prefHexOnlyMB, 512, []int{256, 512, 1024, 2048}, "Hex only"), "binary", "image", "dump"),
		),
	)
	return p
}

func splitNamesOr(v string, def []string) []string {
	if v == "" {
		return def
	}
	return splitNames(v)
}

func mb(n int) string {
	if n >= 1024 && n%1024 == 0 {
		return fmt.Sprintf("%d GB", n>>10)
	}
	return fmt.Sprintf("%d MB", n)
}
