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
	only := flag.String("only", "", "render only the scenarios whose names contain this")
	flag.Parse()
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}

	wide, mid, narrow := fyne.NewSize(1440, 900), fyne.NewSize(1280, 800), fyne.NewSize(1024, 640)
	openReadme := func(s *shell.Shell, root string) { s.OpenFile(filepath.Join(root, "README.md")) }
	sel := func(p theme.Palette, a theme.Accent) theme.Selection {
		return theme.Selection{PaletteID: p.ID, AccentID: a.ID}
	}

	var scenarios []scenario
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			scenarios = append(scenarios, scenario{
				fmt.Sprintf("workspace-%s-%s-1440", p.ID, a.ID), sel(p, a), wide, openReadme})
		}
	}
	sk, ip, ig := theme.SmokedKraft, theme.InkPaper, theme.InkGlass
	scenarios = append(scenarios,
		scenario{"workspace-smoked-kraft-crimson-1280", sel(sk, theme.Crimson), mid, openReadme},
		scenario{"workspace-smoked-kraft-crimson-1024", sel(sk, theme.Crimson), narrow, openReadme},
		scenario{"workspace-ink-paper-crimson-1024", sel(ip, theme.Crimson), narrow, openReadme},
		scenario{"inspector-floating-ink-glass-crimson-1024", sel(ig, theme.Crimson), narrow,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.ToggleInspector() // reopened below the breakpoint: floats as glass
			}},
		scenario{"empty-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide, nil},
	)
	for _, p := range []theme.Palette{sk, ip, ig} {
		scenarios = append(scenarios,
			scenario{"palette-" + p.ID + "-crimson-1440", sel(p, theme.Crimson), wide,
				func(s *shell.Shell, root string) {
					openReadme(s, root)
					s.ShowCommands()
					test.Type(s.Window().Canvas().Focused(), "theme")
				}},
			scenario{"settings-" + p.ID + "-crimson-1440", sel(p, theme.Crimson), wide,
				func(s *shell.Shell, _ string) { s.OpenSettings() }},
		)
	}
	glass := func(p theme.Palette) theme.Selection {
		s := sel(p, theme.Crimson)
		s.Glass = true
		return s
	}
	scenarios = append(scenarios,
		// The glass setting: translucent overlays over a backdrop blur.
		scenario{"glass-palette-ink-glass-crimson-1440", glass(ig), wide,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.ShowCommands()
				test.Type(s.Window().Canvas().Focused(), "theme")
			}},
		scenario{"glass-palette-smoked-kraft-crimson-1440", glass(sk), wide,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.ShowCommands()
			}},
		scenario{"glass-inspector-floating-verdigris-forge-crimson-1024", glass(theme.VerdigrisForge), narrow,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.ToggleInspector()
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
	s := shell.NewWithOptions(a, root, shell.Options{Dispatch: func(f func()) { uiQueue <- f }})
	defer s.StopAll()
	s.SetSelection(sc.sel)
	s.Window().Resize(sc.size)
	if sc.setup != nil {
		sc.setup(s, root)
	}
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
