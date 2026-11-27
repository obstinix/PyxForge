package notes

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// project writes files (slash-separated names) under a temporary root.
func project(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

var demo = map[string]string{
	"boot/boot.asm":  "bits 16\nstart:\n    cli\nserial_put:\n    ret\n",
	"kernel/main.c":  "#include <stdio.h>\n\nstatic int count;\n\nint kmain(void)\n{\n    return 0;\n}\n",
	"kernel/util.c":  "int util(void) { return 1; }\n",
	"drivers/util.c": "int drv(void) { return 2; }\n",
	"tools/build.go": "package tools\n\nfunc Build() error {\n\treturn nil\n}\n",
	"notes/boot-hang.md": "# Boot hang\n\nSee [[boot.asm]] and [[boot.asm#serial_put]], also [[kernel/main.c:5]].\n" +
		"[[util.c]] is ambiguous, [[nowhere.c]] is missing, [[main.c#nope]] has no such symbol.\n" +
		"[[ideas]] and [[ideas|my ideas]]. #boot #debug/serial\n\nThe kernel's main.c and build.go matter; so does kmain.\n" +
		"```\n[[NotALink]] #nottag\n```\n`[[inline]]`\n",
	"notes/ideas.md": "Ideas, linking back: [[boot-hang]] and [[tools/build.go#Build]].\n",
	"README.md":      "Root readme. [[ideas]]\n",
}

func TestParse(t *testing.T) {
	links, tags := Parse(demo["notes/boot-hang.md"])
	var raws []string
	for _, l := range links {
		raws = append(raws, l.Raw)
	}
	want := []string{"boot.asm", "boot.asm#serial_put", "kernel/main.c:5", "util.c", "nowhere.c", "main.c#nope", "ideas", "ideas|my ideas"}
	if !slices.Equal(raws, want) {
		t.Errorf("links %q", raws)
	}
	if !slices.Equal(tags, []string{"boot", "debug/serial"}) {
		t.Errorf("tags %q", tags)
	}
	l := links[2]
	if l.Target != "kernel/main.c" || l.Line != 5 || l.At.Line != 2 {
		t.Errorf("line link %+v", l)
	}
	if l := links[7]; l.Target != "ideas" || l.Label != "my ideas" {
		t.Errorf("label link %+v", l)
	}
	if l := parseLink(`C:\x\file.c:12`); l.Target != "C:/x/file.c" || l.Line != 12 {
		t.Errorf("a Windows path %+v", l)
	}
}

// TestResolveEveryForm: path, base name, line, symbol by server and by search, note, and the
// ambiguous and missing cases.
func TestResolveEveryForm(t *testing.T) {
	root := project(t, demo)
	x, err := Build(root, "notes", nil)
	if err != nil {
		t.Fatal(err)
	}
	f := x.Files()
	r := func(raw string, sym SymbolFinder) Resolved {
		return f.Resolve("notes/boot-hang.md", parseLink(raw), sym)
	}
	cases := []struct {
		raw  string
		want Resolved
	}{
		{"boot.asm", Resolved{Path: "boot/boot.asm"}},
		{"boot/boot.asm", Resolved{Path: "boot/boot.asm"}},
		{"../boot/boot.asm", Resolved{Path: "boot/boot.asm"}}, // from the note's folder
		{"kernel/main.c:5", Resolved{Path: "kernel/main.c", Line: 5}},
		{"boot.asm#serial_put", Resolved{Path: "boot/boot.asm", Line: 4, Approximate: true}},
		{"main.c#kmain", Resolved{Path: "kernel/main.c", Line: 5, Approximate: true}},
		{"main.c#count", Resolved{Path: "kernel/main.c", Line: 3, Approximate: true}},
		{"build.go#Build", Resolved{Path: "tools/build.go", Line: 3, Approximate: true}},
		{"main.c#nope", Resolved{Path: "kernel/main.c", NoSymbol: true}},
		{"util.c", Resolved{Ambiguous: []string{"drivers/util.c", "kernel/util.c"}}},
		{"kernel/util.c", Resolved{Path: "kernel/util.c"}},
		{"nowhere.c", Resolved{Missing: true}},
		{"ideas", Resolved{Path: "notes/ideas.md", Note: true}},
		{"notes/ideas", Resolved{Path: "notes/ideas.md", Note: true}},
		{"no-such-note", Resolved{Missing: true}},
	}
	for _, c := range cases {
		got := r(c.raw, nil)
		if got.Path != c.want.Path || got.Line != c.want.Line || got.Approximate != c.want.Approximate ||
			got.Missing != c.want.Missing || got.NoSymbol != c.want.NoSymbol || got.Note != c.want.Note ||
			!slices.Equal(got.Ambiguous, c.want.Ambiguous) {
			t.Errorf("[[%s]]: %+v, want %+v", c.raw, got, c.want)
		}
	}
	// A language server's answer wins and is not approximate.
	server := func(p, sym string) (int, bool) { return 99, p == "kernel/main.c" && sym == "kmain" }
	if got := r("main.c#kmain", server); got.Line != 99 || got.Approximate {
		t.Errorf("with a server: %+v", got)
	}
}

// TestBacklinks: who links to a file, with each link's line; notes with issues list them.
func TestBacklinks(t *testing.T) {
	root := project(t, demo)
	x, _ := Build(root, "notes", nil)
	bl := x.Backlinks("boot/boot.asm")
	if len(bl) != 2 || bl[0].Note != "notes/boot-hang.md" || bl[0].Line != 2 || !strings.Contains(bl[0].Context, "See [[boot.asm]]") {
		t.Fatalf("boot.asm's backlinks %+v", bl)
	}
	notesTo := func(p string) []string {
		var out []string
		for _, b := range x.Backlinks(p) {
			out = append(out, b.Note)
		}
		return out
	}
	if got := notesTo("notes/ideas.md"); !slices.Equal(got, []string{"README.md", "notes/boot-hang.md", "notes/boot-hang.md"}) {
		t.Errorf("ideas.md's backlinks %v", got)
	}
	var issues []string
	for _, b := range x.Issues("notes/boot-hang.md") {
		issues = append(issues, b.Link.Raw)
	}
	if !slices.Equal(issues, []string{"util.c", "nowhere.c", "main.c#nope"}) {
		t.Errorf("issues %v", issues)
	}
	if !slices.Equal(x.Tags("notes/boot-hang.md"), []string{"boot", "debug/serial"}) {
		t.Errorf("tags %v", x.Tags("notes/boot-hang.md"))
	}
}

// TestIndexFollowsDisk: a renamed file leaves its old-name links missing until the note is
// changed; a deleted note's links go; a new note's links count; a new file can make an
// ambiguous name, and removing one makes it unique again.
func TestIndexFollowsDisk(t *testing.T) {
	root := project(t, demo)
	x, _ := Build(root, "notes", nil)
	abs := func(p string) string { return filepath.Join(root, filepath.FromSlash(p)) }

	// Rename boot.asm to stage1.asm.
	os.Rename(abs("boot/boot.asm"), abs("boot/stage1.asm"))
	x.Update([]string{abs("boot/boot.asm"), abs("boot/stage1.asm")})
	if len(x.Backlinks("boot/boot.asm")) != 0 || len(x.Backlinks("boot/stage1.asm")) != 0 {
		t.Error("links followed the rename by themselves")
	}
	missing := 0
	for _, b := range x.Issues("notes/boot-hang.md") {
		if b.To.Missing && strings.HasPrefix(b.Link.Raw, "boot.asm") {
			missing++
		}
	}
	if missing != 2 {
		t.Errorf("after the rename, %d boot.asm links are missing, want 2", missing)
	}
	// The note is edited to the new name.
	note := abs("notes/boot-hang.md")
	data, _ := os.ReadFile(note)
	os.WriteFile(note, []byte(strings.ReplaceAll(string(data), "boot.asm", "stage1.asm")), 0o644)
	x.Update([]string{note})
	if len(x.Backlinks("boot/stage1.asm")) != 2 {
		t.Errorf("after editing the note: %v", x.Backlinks("boot/stage1.asm"))
	}
	// A deleted note's links go with it.
	os.Remove(abs("README.md"))
	x.Update([]string{abs("README.md")})
	for _, b := range x.Backlinks("notes/ideas.md") {
		if b.Note == "README.md" {
			t.Error("the deleted README.md still links")
		}
	}
	// Removing one of two util.c files makes [[util.c]] resolve.
	os.Remove(abs("drivers/util.c"))
	x.Update([]string{abs("drivers/util.c")})
	if len(x.Backlinks("kernel/util.c")) != 1 {
		t.Errorf("util.c after the other one went: %v", x.Backlinks("kernel/util.c"))
	}
	// A new note links.
	os.WriteFile(abs("notes/new.md"), []byte("about [[main.c]]\n"), 0o644)
	x.Update([]string{abs("notes/new.md")})
	found := false
	for _, b := range x.Backlinks("kernel/main.c") {
		found = found || b.Note == "notes/new.md"
	}
	if !found {
		t.Error("the new note's link is not a backlink")
	}
}

// TestUnlinkedMentions: base names in the text, not linked, not in code, unique.
func TestUnlinkedMentions(t *testing.T) {
	root := project(t, demo)
	x, _ := Build(root, "notes", nil)
	var names []string
	for _, m := range x.UnlinkedMentions("notes/boot-hang.md") {
		names = append(names, m.Name+"@"+m.Path)
	}
	// main.c is linked ([[kernel/main.c:5]]); build.go is not.
	if !slices.Equal(names, []string{"build.go@tools/build.go"}) {
		t.Errorf("mentions %v", names)
	}
}

func TestLinkTo(t *testing.T) {
	root := project(t, demo)
	x, _ := Build(root, "notes", nil)
	if got := x.LinkTo("kernel/main.c", 7); got != "[[main.c:7]]" {
		t.Errorf("unique base name: %s", got)
	}
	if got := x.LinkTo("kernel/util.c", 1); got != "[[kernel/util.c:1]]" {
		t.Errorf("ambiguous base name: %s", got)
	}
}
