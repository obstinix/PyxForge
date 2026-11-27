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
	"github.com/obstinix/PyxForge/internal/ui/graphview"
	"github.com/obstinix/PyxForge/internal/ui/kit"
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
		)
	}
	scenarios = append(scenarios,
		scenario{"editor-native-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide, openBoot},
		scenario{"editor-native-ink-paper-amber-1440", sel(ip, theme.Amber), wide, openBoot},
		// Two editor groups: boot.asm split to the right of README.md and ROADMAP.md.
		scenario{"editor-groups-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				openReadme(s, root)
				s.OpenFile(filepath.Join(root, "docs", "ROADMAP.md"))
				openBoot(s, root)
				s.Commands().Run("view.splitRight")
			}},
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

	// Settings as a modal (D22): General, Appearance, a search across pages and Hotkeys, over a
	// file, in every theme and accent.
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			for _, v := range []struct {
				name string
				run  func(s *shell.Shell)
			}{
				{"general", func(s *shell.Shell) { s.OpenSettingsAt("general") }},
				{"appearance", func(s *shell.Shell) { s.OpenSettingsAt("appearance") }},
				{"search", func(s *shell.Shell) { s.OpenSettingsAt("general"); s.SearchSettings("size") }},
				{"hotkeys", func(s *shell.Shell) { s.OpenSettingsAt("hotkeys") }},
			} {
				scenarios = append(scenarios, scenario{fmt.Sprintf("settings-%s-%s-%s-1440", v.name, p.ID, a.ID), sel(p, a), wide,
					func(s *shell.Shell, root string) { openBoot(s, root); v.run(s) }})
			}
		}
	}

	// Liquid Glass (D21): the palette and Settings over a busy editor, in every theme and
	// accent, so the blur, the tint, the rim and the plates can be judged.
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			scenarios = append(scenarios,
				scenario{fmt.Sprintf("palette-glass-%s-%s-1440", p.ID, a.ID), sel(p, a), wide,
					func(s *shell.Shell, root string) {
						s.SetMaterial(true, 1)
						openBoot(s, root)
						s.ShowCommands()
						test.Type(s.Window().Canvas().Focused(), "go to")
					}},
				scenario{fmt.Sprintf("settings-glass-%s-%s-1440", p.ID, a.ID), sel(p, a), wide,
					func(s *shell.Shell, root string) {
						s.SetMaterial(true, 1)
						openBoot(s, root)
						s.ShowSetting("appearance", "appearance.reduceMotion")
					}},
			)
		}
	}
	scenarios = append(scenarios,
		scenario{"notifications-glass-smoked-kraft-crimson-1440", sel(sk, theme.Crimson), wide,
			func(s *shell.Shell, root string) {
				s.SetMaterial(true, 1)
				openBoot(s, root)
				s.Notify(notifications.Success, "Build succeeded", "boot.img (512 bytes) in 0.4 s.")
				s.Notify(notifications.Error, "Build failed", "boot.asm:12: error: label or instruction expected at start of line")
			}},
	)

	// The Terminal (identity prompt Phase 4): command blocks (one success, one failure with
	// its header on hover, one collapsed), the rail at its narrow width, and two panes.
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			scenarios = append(scenarios,
				scenario{fmt.Sprintf("terminal-blocks-%s-%s-1440", p.ID, a.ID), sel(p, a), wide,
					func(s *shell.Shell, root string) { openBoot(s, root); s.ShowTerminalDemo(false) }},
				scenario{fmt.Sprintf("terminal-rail-narrow-%s-%s-960", p.ID, a.ID), sel(p, a), fyne.NewSize(960, 640),
					func(s *shell.Shell, root string) { openBoot(s, root); s.ShowTerminalDemo(false) }},
				scenario{fmt.Sprintf("terminal-split-%s-%s-1440", p.ID, a.ID), sel(p, a), wide,
					func(s *shell.Shell, root string) { openBoot(s, root); s.ShowTerminalDemo(true) }},
			)
		}
	}

	// The graph view (identity prompt Phase 5): this repository's graph, the Local Graph of
	// boot.asm, and the controls panel open, in every theme and each colour scheme.
	graphReady := func(s *shell.Shell, scheme graphview.Scheme) *graphview.View {
		s.Commands().Run("view.graph")
		until(func() bool { v, _ := s.GraphView(); return v != nil && v.Graph() != nil })
		v, _ := s.GraphView()
		gs := v.Settings()
		gs.Scheme = scheme
		v.SetSettings(gs)
		_, c := s.GraphView()
		c.Rebuild() // the panel shows the scheme set here
		v.Settle(1500)
		v.Fit()
		return v
	}
	for _, p := range theme.Palettes {
		for si, scheme := range graphview.SchemeNames {
			name := strings.ToLower(scheme)
			sch := graphview.Scheme(si)
			scenarios = append(scenarios,
				scenario{fmt.Sprintf("graph-pyxforge-repo-%s-%s-1440", p.ID, name), sel(p, theme.Crimson), wide,
					func(s *shell.Shell, root string) {
						v := graphReady(s, sch)
						_, c := s.GraphView()
						c.Toggle() // the panel collapsed: the graph whole
						v.Fit()
					}},
				scenario{fmt.Sprintf("graph-controls-%s-%s-1440", p.ID, name), sel(p, theme.Crimson), wide,
					func(s *shell.Shell, root string) {
						graphReady(s, sch)
						_, c := s.GraphView()
						for _, sec := range []string{"Groups", "Forces"} {
							c.OpenSection(sec)
						}
					}},
				scenario{fmt.Sprintf("graph-local-%s-%s-1440", p.ID, name), sel(p, theme.Crimson), wide,
					func(s *shell.Shell, root string) {
						openReadme(s, root) // README.md links to the docs
						s.ShowLocalGraph()
						until(func() bool { return s.LocalGraph().Graph() != nil })
						l := s.LocalGraph()
						gs := l.Settings()
						gs.Scheme = sch
						l.SetSettings(gs)
						l.Settle(1500)
						l.Fit()
					}},
			)
		}
	}

	// Every kit component in every state (identity prompt, Phase 1), in every theme and accent.
	for _, p := range theme.Palettes {
		for _, a := range theme.Accents {
			scenarios = append(scenarios, scenario{fmt.Sprintf("kit-gallery-%s-%s-1440", p.ID, a.ID), sel(p, a), wide,
				func(s *shell.Shell, _ string) { s.Window().SetContent(kit.NewGallery()) }})
		}
	}

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

// until runs queued UI work until cond holds, at most 30 s.
func until(cond func() bool) {
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline) && !cond(); {
		select {
		case f := <-uiQueue:
			f()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

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
