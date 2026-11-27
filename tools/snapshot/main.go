// Command snapshot renders the PyxForge shell with Fyne's software renderer, for the design
// reviews in refactor-v2.md Section 16.2. It writes one PNG per scenario.
//
//	go run ./tools/snapshot -out docs/design/phase1-screens
//
// The renders use the real shell; review-only steps (opening a dialog, posting a notification)
// are listed in the scenario table below.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/shell"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

type scenario struct {
	name  string
	sel   theme.Selection
	size  fyne.Size
	setup func(s *shell.Shell, root string)
}

func main() {
	out := flag.String("out", "docs/design/phase1-screens", "folder for the PNG files")
	rootFlag := flag.String("root", ".", "workspace shown in the explorer")
	emptyWorkspace := flag.Bool("empty-workspace", false, "render without selecting a workspace")
	only := flag.String("only", "", "render only the scenarios whose names contain this")
	flag.Parse()
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fail(err)
	}
	if *emptyWorkspace {
		root = ""
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}

	wide, mid, narrow := fyne.NewSize(1440, 900), fyne.NewSize(1280, 800), fyne.NewSize(1024, 640)
	openReadme := func(s *shell.Shell, root string) { s.OpenFile(filepath.Join(root, "README.md")) }
	openBoot := func(s *shell.Shell, root string) {
		s.OpenFile(filepath.Join(root, "examples", "boot-sector", "boot.asm"))
	}
	sel := func(p theme.Palette, a theme.Accent) theme.Selection {
		return theme.Selection{PaletteID: p.ID, AccentID: a.ID}
	}

	var scenarios []scenario
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			scenarios = append(scenarios, scenario{
				fmt.Sprintf("workspace-%s-%s-1440", p.ID, a.ID), sel(p, a), wide, openBoot})
		}
	}
	sk, ip, vf := theme.SmokedKraft, theme.InkPaper, theme.VerdigrisForge
	scenarios = append(scenarios,
		scenario{"workspace-smoked-kraft-crimson-1280", sel(sk, theme.Crimson), mid, openReadme},
		scenario{"workspace-smoked-kraft-crimson-1024", sel(sk, theme.Crimson), narrow, openReadme},
		scenario{"workspace-ink-paper-crimson-1024", sel(ip, theme.Crimson), narrow, openReadme},
		scenario{"inspector-floating-verdigris-forge-crimson-1024", sel(vf, theme.Crimson), narrow,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.ToggleInspector() // reopened below the breakpoint: it floats
			}},
		scenario{"empty-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide, nil},
		scenario{"empty-smoked-kraft-crimson-1024", sel(sk, theme.Crimson), narrow, nil},
		scenario{"empty-smoked-kraft-crimson-800", sel(sk, theme.Crimson), fyne.NewSize(800, 640), nil},
	)
	for _, p := range []theme.Palette{sk, ip, vf} {
		scenarios = append(scenarios,
			scenario{"palette-" + p.ID + "-crimson-1440", sel(p, theme.Crimson), wide,
				func(s *shell.Shell, root string) {
					openReadme(s, root)
					s.ShowCommands()
					test.Type(s.Window().Canvas().Focused(), "theme")
				}},
			scenario{"settings-" + p.ID + "-crimson-1440", sel(p, theme.Crimson), wide,
				func(s *shell.Shell, _ string) { s.OpenSettings() }},
			// The whole Settings page, every section, in a window tall enough to hold it.
			scenario{"settings-full-" + p.ID + "-crimson-1440", sel(p, theme.Crimson), fyne.NewSize(1440, 2000),
				func(s *shell.Shell, _ string) { s.OpenSettings() }},
		)
	}
	scenarios = append(scenarios,
		scenario{"editor-native-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide, openBoot},
		scenario{"editor-native-ink-paper-amber-1440", sel(ip, theme.Amber), wide, openBoot},
		// A pinned tab: boot.asm opened second, pinned, so it moves to the front with its pin.
		scenario{"tabs-pinned-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.OpenFile(filepath.Join(root, "docs", "ROADMAP.md"))
				openBoot(s, root)
				s.Commands().Run("view.pinTab")
			}},
	)
	run := func(ids ...string) func(s *shell.Shell, root string) {
		return func(s *shell.Shell, root string) {
			openReadme(s, root)
			for _, id := range ids {
				s.Commands().Run(id)
			}
		}
	}
	scenarios = append(scenarios,
		// Keyboard focus and the active region (decisions A1 and R1).
		scenario{"focus-settings-card-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			run("prefs.settings")},
		scenario{"focus-explorer-ink-paper-crimson-1440", sel(ip, theme.Crimson), wide,
			run("view.focusExplorer")},
		scenario{"focus-panel-log-monochrome-amber-1440", sel(theme.Monochrome, theme.Amber), wide,
			run("panel.log")},
	)
	scenarios = append(scenarios,
		// Review-only: the notification texts are the ones the shell really posts.
		scenario{"notifications-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.Notify(notifications.Error, "Cannot read folder", "open build: Access is denied.")
				s.Notify(notifications.Success, "Appearance reset", "System (Smoked Kraft) · Crimson")
			}},
		scenario{"setup-wizard-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) { s.Commands().Run("help.setup") }},
		scenario{"setup-wizard-ink-paper-amber-1440", sel(ip, theme.Amber), wide,
			func(s *shell.Shell, root string) { s.Commands().Run("help.setup") }},
		scenario{"about-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.Commands().Run("help.about")
			}},
	)

	scenarios = append(scenarios,
		// The Build tab after building examples/boot-sector (pass -root examples/boot-sector).
		scenario{"build-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				s.OpenFile(filepath.Join(root, "boot.asm"))
				s.Commands().Run("build.run")
				waitIdle(s)
			}},
		// A debug session paused at the boot sector's first instruction (-root examples/boot-sector).
		scenario{"debug-verdigris-forge-crimson-1440", sel(theme.VerdigrisForge, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				s.OpenFile(filepath.Join(root, "boot.asm"))
				s.Commands().Run("run.debug")
				waitIdle(s)
				s.Commands().Run("debug.stepInstruction")
				for range 20 { // let the step's stop arrive and its views be read
					time.Sleep(20 * time.Millisecond)
					waitIdle(s)
				}
			}},
		scenario{"debug-disasm-ink-paper-amber-1440", sel(ip, theme.Amber), wide,
			func(s *shell.Shell, root string) {
				s.Commands().Run("run.debug")
				waitIdle(s)
				s.Commands().Run("inspector.disasm")
				s.Commands().Run("panel.gdb")
			}},
		// The boot-sector map of examples/boot-sector (pass -root examples/boot-sector).
		scenario{"map-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				s.Commands().Run("inspector.map")
			}},
		// The side-by-side diff of boot.asm in a repository where it has changed (pass -root
		// such a repository).
		scenario{"git-diff-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				s.OpenFile(filepath.Join(root, "boot.asm"))
				waitIdle(s)
				s.Commands().Run("git.diffCurrent")
				for range 50 { // the diff is read in the background
					time.Sleep(20 * time.Millisecond)
					waitIdle(s)
				}
			}},
		scenario{"git-diff-ink-paper-amber-1440", sel(ip, theme.Amber), wide,
			func(s *shell.Shell, root string) {
				s.OpenFile(filepath.Join(root, "boot.asm"))
				waitIdle(s)
				s.Commands().Run("git.diffCurrent")
				for range 50 { // the diff is read in the background
					time.Sleep(20 * time.Millisecond)
					waitIdle(s)
				}
			}},
		// The Git tab on this repository (pass -root .).
		scenario{"git-ink-paper-crimson-1440", sel(ip, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.Commands().Run("panel.git")
				waitIdle(s)
			}},
	)

	for _, sc := range scenarios {
		if !strings.Contains(sc.name, *only) {
			continue
		}
		p := filepath.Join(*out, sc.name+".png")
		if err := render(p, root, sc); err != nil {
			fail(fmt.Errorf("%s: %w", sc.name, err))
		}
		fmt.Println(p)
	}
}

func render(path, root string, sc scenario) error {
	a := test.NewApp()
	defer a.Quit()
	// No toolchain probe, so renders stay deterministic. Background work (a build, QEMU, GDB)
	// posts UI updates to uiQueue, which waitIdle drains on this goroutine: Fyne's test driver
	// would otherwise run them on the posting goroutines, at the same time as each other.
	opts := shell.Options{Editor: true, Dispatch: func(f func()) { uiQueue <- f }} // files open in the editor (D16)
	s := shell.NewWithOptions(a, root, opts)
	defer s.StopAll()
	s.SetSelection(sc.sel)
	s.Window().Resize(sc.size)
	if sc.setup != nil {
		sc.setup(s, root)
	}
	drain() // what the app would run on the next pass of its loop: a relayout, a refresh
	img := s.Window().Canvas().Capture()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

var uiQueue = make(chan func(), 4096)

// drain runs the UI work already queued, and what that queues in turn, until nothing more
// arrives for a moment.
func drain() {
	for {
		select {
		case f := <-uiQueue:
			f()
		case <-time.After(50 * time.Millisecond):
			return
		}
	}
}

// waitIdle runs queued UI work until background work a scenario started (a build, a Git
// refresh, a debug session reaching its first stop) has settled.
func waitIdle(s *shell.Shell) {
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		select {
		case f := <-uiQueue:
			f()
			continue
		case <-time.After(20 * time.Millisecond):
		}
		if s.Idle() {
			return
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "snapshot:", err)
	os.Exit(1)
}
