// Command editorspike is the feasibility harness for PyxForge's native editor (decision D16).
// It opens a real window with two documents in tabs, drives the editor through the paths a
// user's keys take, talks to a real clangd, and reports what worked and how fast:
//
//	go run ./tools/editorspike [-out frame.png] [-keep]
//
// Checks: open a 10,000-line file, first frame, paging, typing, undo, saving to disk, a second
// document in its own tab, clangd's diagnostics, completion, hover and go to definition, and
// that clangd ends when the window closes. It exits with status 1 if a check fails. -keep
// leaves the window open after the checks, for a person to try.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/obstinix/PyxForge/internal/lsp"
	"github.com/obstinix/PyxForge/internal/textbuf"
	"github.com/obstinix/PyxForge/internal/ui/codeedit"
	"github.com/obstinix/PyxForge/internal/ui/theme"
)

type result struct {
	name string
	ok   bool
	note string
}

var results []result

func check(name string, ok bool, format string, args ...any) {
	results = append(results, result{name, ok, fmt.Sprintf(format, args...)})
}

func ui(f func()) {
	done := make(chan struct{})
	fyne.Do(func() { f(); close(done) })
	<-done
}

func waitFor(timeout time.Duration, ok func() bool) bool {
	deadline := time.Now().Add(timeout)
	for !ok() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
	return true
}

func main() {
	out := flag.String("out", "", "write a PNG of the window")
	keep := flag.Bool("keep", false, "leave the window open after the checks")
	flag.Parse()
	dir, err := os.MkdirTemp("", "pyxforge-editorspike-")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	var big strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&big, "static int f%d(int x) {\n\t/* step %d */\n\treturn x * %d + 1;\n}\n\n", i, i, i)
	}
	bigPath := filepath.Join(dir, "big.c")
	os.WriteFile(bigPath, []byte(big.String()), 0o644)
	mainPath := filepath.Join(dir, "main.c")
	mainSrc := "#include <stdio.h>\n\nstatic int add(int a, int b) { return a + b; }\n\nint main(void) {\n\tint total = add(1, 2);\n\tprintf(\"%d\\n\", tota);\n\treturn 0;\n}\n"
	os.WriteFile(mainPath, []byte(mainSrc), 0o644)

	a := app.NewWithID("io.github.obstinix.pyxforge.editorspike")
	a.Settings().SetTheme(theme.NewFyne(theme.Default))
	w := a.NewWindow("PyxForge native editor spike")
	tabs := container.NewDocTabs()
	w.SetContent(tabs)
	w.Resize(fyne.NewSize(1280, 800))

	exit := 0
	var clangd *lsp.Client
	go func() {
		defer func() {
			if !*keep {
				fyne.Do(a.Quit)
			}
		}()
		run(w, tabs, bigPath, mainPath, *out, &clangd)
		fmt.Printf("\nPyxForge native editor spike on %s/%s\n\n", runtime.GOOS, runtime.GOARCH)
		for _, r := range results {
			mark := "ok  "
			if !r.ok {
				mark = "FAIL"
				exit = 1
			}
			fmt.Printf("%s %-34s %s\n", mark, r.name, r.note)
		}
	}()
	w.ShowAndRun()
	if clangd != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		clangd.Shutdown(ctx)
		cancel()
		select {
		case <-clangd.Done:
			fmt.Println("ok   clangd ended with the window")
		default:
			fmt.Println("FAIL clangd still running after the window closed")
			exit = 1
		}
	}
	os.Exit(exit)
}

func load(path string) *codeedit.Editor {
	b, err := textbuf.Load(path)
	if err != nil {
		panic(err)
	}
	return codeedit.New(b, path)
}

func run(w fyne.Window, tabs *container.DocTabs, bigPath, mainPath, out string, clangd **lsp.Client) {
	time.Sleep(500 * time.Millisecond) // the window is up
	var big *codeedit.Editor
	start := time.Now()
	ui(func() {
		big = load(bigPath)
		tabs.Append(container.NewTabItem("big.c", big))
		w.Canvas().Focus(big)
	})
	firstFrame := time.Since(start)
	check("open 10,000 lines", big.Buf.LineCount() == 10001, "%v to the first frame, %d lines", firstFrame, big.Buf.LineCount())

	// Page down through the whole file.
	start = time.Now()
	pages := 0
	for big.Cur.Line < big.Buf.LineCount()-1 && pages < 1000 {
		ui(func() { big.TypedKey(&fyne.KeyEvent{Name: fyne.KeyPageDown}) })
		pages++
	}
	d := time.Since(start)
	check("page through it", big.Cur.Line == big.Buf.LineCount()-1, "%d pages in %v, %v a page (key and frame)", pages, d, d/time.Duration(max(pages, 1)))

	// Type, undo, save.
	ui(func() {
		big.Reveal(codeedit.Pos{Line: 1, Col: 0})
		big.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
	})
	start = time.Now()
	for _, r := range " // typed by the spike" {
		ui(func() { big.TypedRune(r) })
	}
	typing := time.Since(start) / 22
	check("type", strings.HasSuffix(string(big.Buf.Line(1)), "// typed by the spike */") || strings.Contains(string(big.Buf.Line(1)), "typed by the spike"), "%v a key, line now %q", typing, string(big.Buf.Line(1)))
	ui(func() { big.TypedShortcut(&fyne.ShortcutUndo{}) })
	check("undo", !strings.Contains(string(big.Buf.Line(1)), "spike"), "line %q", string(big.Buf.Line(1)))
	ui(func() { big.TypedShortcut(&fyne.ShortcutRedo{}) })
	err := big.Buf.Save(bigPath)
	disk, _ := os.ReadFile(bigPath)
	check("save", err == nil && strings.Contains(string(disk), "typed by the spike") && !big.Buf.Modified(), "err %v", err)

	// A second document in its own tab, with clangd.
	var m *codeedit.Editor
	ui(func() {
		m = load(mainPath)
		tabs.Append(container.NewTabItem("main.c", m))
		tabs.SelectIndex(1)
		w.Canvas().Focus(m)
	})
	check("second document", len(tabs.Items) == 2 && big.Buf.LineCount() == 10001, "two tabs, each with its own buffer")

	bin, err := exec.LookPath("clangd")
	if err != nil {
		check("clangd", false, "not installed")
		capture(w, out)
		return
	}
	var mu sync.Mutex
	var diags []lsp.Diagnostic
	uri := lsp.URI(mainPath)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	start = time.Now()
	c, err := lsp.Start(ctx, filepath.Dir(mainPath), []string{bin}, func(u string, d []lsp.Diagnostic) {
		if u == uri {
			mu.Lock()
			diags = d
			mu.Unlock()
		}
	})
	if err != nil {
		check("clangd", false, "%v", err)
		return
	}
	*clangd = c
	c.DidOpen(uri, "c", 1, m.Buf.Text())
	got := waitFor(60*time.Second, func() bool { mu.Lock(); defer mu.Unlock(); return len(diags) > 0 })
	mu.Lock()
	var shown []codeedit.Diagnostic
	for _, d := range diags {
		shown = append(shown, codeedit.Diagnostic{Severity: d.Severity, Message: d.Message,
			Range: codeedit.Range{Start: codeedit.Pos{Line: d.Range.Start.Line, Col: d.Range.Start.Character}, End: codeedit.Pos{Line: d.Range.End.Line, Col: d.Range.End.Character}}})
	}
	mu.Unlock()
	ui(func() { m.SetDiagnostics(shown) })
	check("clangd diagnostics", got && len(shown) > 0 && shown[0].Range.Start.Line == 6, "%v after start; %q", time.Since(start), first(shown))

	// Completion at the end of "tota": column 20 of line 7.
	ui(func() { m.SetCursor(codeedit.Pos{Line: 6, Col: 20}, false) })
	items, err := c.Completion(ctx, uri, lsp.Position{Line: 6, Character: 20})
	var list []codeedit.Completion
	for _, it := range items {
		list = append(list, codeedit.Completion{Label: strings.TrimSpace(it.Label), Detail: it.Detail, Insert: it.FilterText})
	}
	m.ShowCompletions(list)
	hasTotal := false
	for _, it := range list {
		hasTotal = hasTotal || strings.Contains(it.Label, "total")
	}
	check("completion", err == nil && hasTotal, "%d items", len(list))
	ui(func() { m.ShowCompletions(nil) })

	// Hover and go to definition on "add".
	hover, err := c.Hover(ctx, uri, lsp.Position{Line: 5, Character: 14})
	check("hover", err == nil && strings.Contains(hover, "add"), "%q", firstLine(hover))
	locs, err := c.Definition(ctx, uri, lsp.Position{Line: 5, Character: 14})
	if err == nil && len(locs) == 1 {
		ui(func() { m.Reveal(codeedit.Pos{Line: locs[0].Range.Start.Line, Col: locs[0].Range.Start.Character}) })
	}
	check("go to definition", err == nil && len(locs) == 1 && m.Cur.Line == 2, "cursor at line %d", m.Cur.Line+1)
	ui(func() {
		m.Reveal(codeedit.Pos{Line: 5, Col: 15})
		m.ShowInfo(firstLine(hover))
	})
	time.Sleep(300 * time.Millisecond)
	ui(func() { fmt.Printf("cursor before capture: %+v\n", m.Cur) })
	capture(w, out)
}

func first(d []codeedit.Diagnostic) string {
	if len(d) == 0 {
		return ""
	}
	return d[0].Message
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

func capture(w fyne.Window, out string) {
	if out == "" {
		return
	}
	var err error
	ui(func() {
		img := w.Canvas().Capture()
		f, ferr := os.Create(out)
		if ferr != nil {
			err = ferr
			return
		}
		defer f.Close()
		err = png.Encode(f, img)
	})
	check("capture", err == nil, "%s", out)
}
