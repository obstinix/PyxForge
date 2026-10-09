// Command nvimspike is the Phase 3 feasibility harness for the embedded Neovim editor (D4).
// It opens a real window with the editor view, drives it through the same input paths a user
// does, and reports what worked and how fast:
//
//	go run ./tools/nvimspike [-out frame.png]
//
// Checks: first redraw, a 2,000-line file, paging through it, editing, the modified flag,
// saving to disk, diagnostics, a :terminal job, a shell chord kept from Neovim, resize, and
// process exit. It exits with status 1 if any check fails.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/obstinix/PyxForge/internal/neovim"
	"github.com/obstinix/PyxForge/internal/ui/editor"
	"github.com/obstinix/PyxForge/internal/ui/shell"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

type result struct {
	name string
	ok   bool
	note string
}

type harness struct {
	win     fyne.Window
	view    *editor.View
	sess    *neovim.Session
	mu      sync.Mutex
	events  []neovim.Event
	results []result
	chord   bool
}

func (h *harness) check(name string, ok bool, format string, args ...any) {
	h.results = append(h.results, result{name, ok, fmt.Sprintf(format, args...)})
}

// ui runs f on the UI thread and waits for it.
func ui(f func()) {
	done := make(chan struct{})
	fyne.Do(func() { f(); close(done) })
	<-done
}

// waitText reports whether some row shows want, checking at least once.
func (h *harness) waitText(want string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		g := h.sess.Grid()
		for r := range g.Rows {
			if strings.Contains(g.Line(r), want) {
				return true
			}
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitFlush polls finely, so round trips are measured to a fraction of a millisecond.
func (h *harness) waitFlush(after uint64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for h.sess.Grid().Version <= after {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Microsecond)
	}
	return true
}

func (h *harness) waitEvent(kind string, ok func(neovim.Event) bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		for _, e := range h.events {
			if e.Kind == kind && ok(e) {
				h.mu.Unlock()
				return true
			}
		}
		h.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func (h *harness) typeRunes(s string) {
	ui(func() {
		for _, r := range s {
			h.view.TypedRune(r)
		}
	})
}

func (h *harness) key(k fyne.KeyName) { ui(func() { h.view.TypedKey(&fyne.KeyEvent{Name: k}) }) }

func (h *harness) shortcut(m fyne.KeyModifier, k fyne.KeyName) {
	ui(func() { h.view.TypedShortcut(&desktop.CustomShortcut{KeyName: k, Modifier: m}) })
}

func main() {
	out := flag.String("out", "", "write a PNG of the window after the paging test")
	flag.Parse()
	a := app.NewWithID("io.github.obstinix.pyxforge.nvimspike")
	a.Settings().SetTheme(theme.NewFyne(theme.Default))
	h := &harness{win: a.NewWindow("PyxForge editor spike"), view: editor.NewView()}
	h.view.IsShellChord = shell.IsShellChord
	h.view.OnShortcut = func(fyne.Shortcut) bool { h.chord = true; return true }
	h.win.SetContent(h.view)
	h.win.Resize(fyne.NewSize(1200, 760))

	exit := 0
	go func() {
		defer func() { fyne.Do(a.Quit) }()
		if err := h.run(*out); err != nil {
			fmt.Println("spike failed:", err)
			exit = 1
			return
		}
		fmt.Printf("\nPyxForge editor spike on %s/%s\n\n", runtime.GOOS, runtime.GOARCH)
		for _, r := range h.results {
			mark := "ok  "
			if !r.ok {
				mark, exit = "FAIL", 1
			}
			fmt.Printf("  %s  %-28s %s\n", mark, r.name, r.note)
		}
	}()
	h.win.ShowAndRun()
	os.Exit(exit)
}

func (h *harness) run(out string) error {
	dir, err := os.MkdirTemp("", "pyxforge-spike-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	var lines []string
	for i := 1; i <= 2000; i++ {
		lines = append(lines, fmt.Sprintf("line %04d:    mov ax, 0x%04x    ; sector %d", i, i, i/32))
	}
	file := filepath.Join(dir, "big.asm")
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return err
	}

	time.Sleep(300 * time.Millisecond) // let the window lay out so the first size is real
	var cell fyne.Size
	var size fyne.Size
	ui(func() { cell, size = h.view.CellSize(), h.view.Size() })
	cols, rows := int(size.Width/cell.Width), int(size.Height/cell.Height)

	start := time.Now()
	h.sess, err = neovim.Start(context.Background(), neovim.Options{
		Dir: dir, Width: cols, Height: rows,
		OnFlush: h.view.FlushHook,
		OnEvent: func(e neovim.Event) { h.mu.Lock(); h.events = append(h.events, e); h.mu.Unlock() },
	})
	if err != nil {
		return err
	}
	_, first := h.sess.WaitFlush(0, 5*time.Second)
	h.check("first redraw", first, "%d ms after launch, grid %dx%d", time.Since(start).Milliseconds(), cols, rows)
	ui(func() { h.view.Attach(h.sess); h.win.Canvas().Focus(h.view) })

	t0 := time.Now()
	if err := h.sess.Open(file); err != nil {
		return err
	}
	h.check("open 2,000-line file", h.waitText("line 0001", 5*time.Second), "%d ms to first screen", time.Since(t0).Milliseconds())

	// Page through the whole file with Ctrl+F, one key at a time, through the view, timing
	// each key from the shortcut to the redraw that answers it.
	before, _, _ := h.view.Stats().Snapshot()
	t0 = time.Now()
	pages := 0
	var worst, total time.Duration
	for !h.waitText("line 2000", 0) && pages < 200 {
		v := h.sess.Grid().Version
		k := time.Now()
		h.shortcut(fyne.KeyModifierControl, fyne.KeyF)
		h.waitFlush(v, 2*time.Second)
		d := time.Since(k)
		total, worst = total+d, max(worst, d)
		pages++
	}
	elapsed := time.Since(t0)
	frames, avgRender, maxRender := h.view.Stats().Snapshot()
	h.check("page to line 2000", h.waitText("line 2000", time.Second),
		"%d pages in %d ms; key->redraw avg %.2f ms max %.2f ms; frame build avg %.2f ms max %.2f ms (%d frames)",
		pages, elapsed.Milliseconds(), ms(total/time.Duration(max(pages, 1))), ms(worst), ms(avgRender), ms(maxRender), frames-before)
	if out != "" {
		time.Sleep(100 * time.Millisecond)
		ui(func() {
			if f, err := os.Create(out); err == nil {
				_ = png.Encode(f, h.win.Canvas().Capture())
				f.Close()
			}
		})
	}

	// Edit through the view: gg, then O to open a line, type, Escape.
	h.typeRunes("gg")
	h.typeRunes("O")
	h.typeRunes("; edited <here>")
	h.key(fyne.KeyEscape)
	h.check("insert text", h.waitText("; edited <here>", 3*time.Second), "typed through the view, '<' escaped")
	h.check("modified flag", h.waitEvent("BufModifiedSet", func(e neovim.Event) bool { return e.Modified }, 3*time.Second), "BufModifiedSet reported")

	h.typeRunes(":w")
	h.key(fyne.KeyReturn)
	saved := h.waitEvent("BufWritePost", func(e neovim.Event) bool { return !e.Modified }, 3*time.Second)
	data, _ := os.ReadFile(file)
	h.check("save to disk", saved && strings.HasPrefix(string(data), "; edited <here>\nline 0001"), "%d bytes written", len(data))

	err = h.sess.ExecLua(`local ns = vim.api.nvim_create_namespace("spike")
vim.diagnostic.set(ns, 0, {{ lnum = 4, col = 0, severity = 1, message = "spike error", source = "nasm" }})`, nil)
	h.check("diagnostics", err == nil && h.waitEvent("DiagnosticChanged", func(e neovim.Event) bool {
		return len(e.Diagnostics) == 1 && e.Diagnostics[0].Line == 4
	}, 3*time.Second), "vim.diagnostic -> PyxForge event with position")

	h.chord = false
	sent := h.sess.Inputs()
	h.shortcut(fyne.KeyModifierShortcutDefault|fyne.KeyModifierShift, fyne.KeyP)
	h.shortcut(fyne.KeyModifierControl, fyne.KeyW) // a Neovim chord, for contrast
	h.check("shell chord", h.chord && h.sess.Inputs() == sent+1, "Ctrl+Shift+P went to the shell; Ctrl+W went to Neovim")
	h.key(fyne.KeyEscape)

	term := `:terminal echo PYXTERM-OK`
	if runtime.GOOS == "windows" {
		term = `:terminal cmd.exe /c echo PYXTERM-OK`
	}
	h.typeRunes(term)
	h.key(fyne.KeyReturn)
	h.check(":terminal job", h.waitText("PYXTERM-OK", 5*time.Second), "pseudo-console output drawn in the grid")

	ui(func() { h.win.Resize(fyne.NewSize(800, 500)) })
	time.Sleep(300 * time.Millisecond)
	ui(func() { cell, size = h.view.CellSize(), h.view.Size() })
	wantCols, wantRows := int(size.Width/cell.Width), int(size.Height/cell.Height)
	deadline := time.Now().Add(3 * time.Second)
	gotRows, gotCols := h.sess.Grid().Size()
	for (gotRows != wantRows || gotCols != wantCols) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		gotRows, gotCols = h.sess.Grid().Size()
	}
	h.check("resize", gotRows == wantRows && gotCols == wantCols, "grid %dx%d for a %dx%d-cell view", gotCols, gotRows, wantCols, wantRows)

	_ = h.sess.Close()
	h.check("process exit", h.sess.Exited(), "Neovim ended with the session")
	return nil
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
