package lsp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestURIRoundTrip(t *testing.T) {
	p := filepath.Join(Canonical(t.TempDir()), "a dir", "é.c")
	if got := Path(URI(p)); got != p {
		t.Errorf("Path(URI(%q)) = %q (%s)", p, got, URI(p))
	}
}

func TestColumnsInUTF16(t *testing.T) {
	c := &Client{Encoding: "utf-16"}
	line := []rune("a😀b") // the emoji is two UTF-16 units
	if c.Character(line, 2) != 3 || c.Column(line, 3) != 2 {
		t.Errorf("character %d, column %d", c.Character(line, 2), c.Column(line, 3))
	}
}

// TestClangd drives a real clangd through every request the editor makes. It skips where
// clangd is not installed.
func TestClangd(t *testing.T) {
	bin, err := exec.LookPath("clangd")
	if err != nil {
		t.Skip("clangd is not installed")
	}
	dir := t.TempDir()
	src := "static int add(int a, int b) { return a + b; }\nint main(void) {\n\tint total = add(1, 2);\n\treturn tota;\n}\n"
	path := filepath.Join(dir, "main.c")
	os.WriteFile(path, []byte(src), 0o644)
	var mu sync.Mutex
	diags := map[string][]Diagnostic{}
	got := make(chan struct{}, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	c, err := Start(ctx, dir, []string{bin, "--log=error"}, func(uri string, d []Diagnostic) {
		mu.Lock()
		diags[uri] = d
		mu.Unlock()
		got <- struct{}{}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer scancel()
		c.Shutdown(sctx)
	}()
	t.Logf("%s %s, positions in %s", c.Name, c.Version, c.Encoding)
	uri := URI(path)
	if err := c.DidOpen(uri, "c", 1, src); err != nil {
		t.Fatal(err)
	}
	// The misspelt "tota" is an error.
	deadline := time.After(90 * time.Second)
	for {
		mu.Lock()
		d := diags[uri]
		mu.Unlock()
		if len(d) > 0 {
			if d[0].Severity != 1 || d[0].Range.Start.Line != 3 {
				t.Errorf("diagnostic %+v", d[0])
			}
			break
		}
		select {
		case <-got:
		case <-deadline:
			t.Fatal("no diagnostics from clangd")
		}
	}
	at := Position{Line: 2, Character: 14} // on "add" in the call
	if h, err := c.Hover(ctx, uri, at); err != nil || !strings.Contains(h, "add") {
		t.Errorf("hover %q, %v", h, err)
	}
	if locs, err := c.Definition(ctx, uri, at); err != nil || len(locs) != 1 || locs[0].Range.Start.Line != 0 {
		t.Errorf("definition %+v, %v", locs, err)
	}
	if refs, err := c.References(ctx, uri, at); err != nil || len(refs) != 2 {
		t.Errorf("references %+v, %v", refs, err)
	}
	// The file's symbols: add and main at the top, main's local total inside it, if the server
	// reports locals.
	syms, err := c.DocumentSymbols(ctx, uri)
	var names []string
	for _, s := range syms {
		if s.Depth == 0 {
			names = append(names, s.Name+":"+KindName(s.Kind))
		}
	}
	if err != nil || strings.Join(names, " ") != "add:function main:function" || syms[0].Selection.Start.Line != 0 {
		t.Errorf("document symbols %+v, %v", syms, err)
	}
	if w, err := c.Rename(ctx, uri, at, "sum"); err != nil || len(w.Files()[uri]) != 2 {
		t.Errorf("rename %+v, %v", w, err)
	}
	items, err := c.Completion(ctx, uri, Position{Line: 3, Character: 11})
	found := false
	for _, it := range items {
		found = found || strings.Contains(it.Label, "total")
	}
	if err != nil || !found {
		t.Errorf("completion has no 'total' among %d items: %v", len(items), err)
	}
	if _, err := c.Format(ctx, uri, 4, true); err != nil {
		t.Errorf("format: %v", err)
	}
	// Fix the typo: the error goes.
	fixed := strings.Replace(src, "return tota;", "return total;", 1)
	c.DidChange(uri, 2, fixed)
	for {
		mu.Lock()
		d := diags[uri]
		mu.Unlock()
		if len(d) == 0 {
			break
		}
		select {
		case <-got:
		case <-deadline:
			t.Fatalf("the error stayed after the fix: %+v", d)
		}
	}
}

// TestServerThatNeverAnswers: a program that does not speak LSP is given up on when the
// start-up deadline passes, and its process is ended.
func TestServerThatNeverAnswers(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	t.Setenv("PYXFORGE_LSP_SILENT", "1")
	start := time.Now()
	_, err = Start(ctx, t.TempDir(), []string{exe, "-test.run=TestSilentHelper"}, nil)
	if err == nil {
		t.Fatal("a silent server started")
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("gave up after %v", time.Since(start))
	}
}

func TestSilentHelper(t *testing.T) {
	if os.Getenv("PYXFORGE_LSP_SILENT") != "1" {
		t.Skip("helper process")
	}
	time.Sleep(time.Minute)
}

func TestServerTable(t *testing.T) {
	s, id := ForFile("/x/kernel/main.c")
	if s == nil || s.Name != "clangd" || id != "c" {
		t.Fatalf("main.c: %+v %q", s, id)
	}
	if s, id := ForFile("boot.ASM"); s == nil || s.Name != "asm-lsp" || id != "nasm" {
		t.Errorf("boot.ASM: %+v %q", s, id)
	}
	if s, _ := ForFile("notes.txt"); s != nil {
		t.Errorf("a server for .txt: %s", s.Name)
	}
	py, _ := ForFile("a.py")
	found := func(name string) (string, error) {
		if name == "basedpyright-langserver" {
			return "/bin/" + name, nil
		}
		return "", os.ErrNotExist
	}
	if cmd := py.Command(found); len(cmd) != 2 || cmd[0] != "/bin/basedpyright-langserver" {
		t.Errorf("the alternative was not chosen: %v", cmd)
	}
	if got := section(py.Settings, "python.analysis"); got == nil {
		t.Error("no python.analysis section")
	}
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(root, "a", "compile_commands.json"), nil, 0o644)
	if got := s.Root(filepath.Join(sub, "x.c"), root); got != filepath.Join(root, "a") {
		t.Errorf("root %s", got)
	}
}

// TestSymbolForms: both answers a server may give to documentSymbol are read, in document
// order with nested symbols after their parent.
func TestSymbolForms(t *testing.T) {
	tree := `[{"name":"Point","kind":23,"range":{"start":{"line":0,"character":0},"end":{"line":3,"character":1}},
		"selectionRange":{"start":{"line":0,"character":7},"end":{"line":0,"character":12}},
		"children":[{"name":"x","kind":8,"range":{"start":{"line":1,"character":1},"end":{"line":1,"character":7}},
		"selectionRange":{"start":{"line":1,"character":5},"end":{"line":1,"character":6}}}]},
		{"name":"main","kind":12,"range":{"start":{"line":5,"character":0},"end":{"line":7,"character":1}},
		"selectionRange":{"start":{"line":5,"character":4},"end":{"line":5,"character":8}}}]`
	got := symbols([]byte(tree))
	if len(got) != 3 || got[0].Name != "Point" || got[1].Name != "x" || got[1].Depth != 1 || got[1].Container != "Point" ||
		got[0].Selection.Start.Character != 7 || got[2].Name != "main" || KindName(got[2].Kind) != "function" {
		t.Errorf("tree: %+v", got)
	}
	flat := `[{"name":"start","kind":12,"location":{"uri":"file:///b.asm","range":{"start":{"line":6,"character":0},"end":{"line":6,"character":6}}},"containerName":"text"}]`
	got = symbols([]byte(flat))
	if len(got) != 1 || got[0].Name != "start" || got[0].Selection.Start.Line != 6 || got[0].Container != "text" {
		t.Errorf("flat: %+v", got)
	}
	if symbols([]byte("null")) != nil || KindName(99) != "symbol" {
		t.Error("an empty answer or an unknown kind")
	}
}
