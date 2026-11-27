// Package notes is Forge Notes (D25): Markdown notes in a folder of the project, linked to its
// code with [[…]] links, and the index of which notes link to which files (backlinks). It reads
// text only; nothing in a note is run (D14).
//
// Link forms:
//
//	[[path/file.c]]    a file by its path from the project root (or from the note's folder)
//	[[file.c]]         a file by its base name, when exactly one file has it
//	[[file.c:42]]      … at a line
//	[[file.c#symbol]]  … at a symbol: the language server's, or a definition-like line found by
//	                   text search, marked approximate
//	[[other-note]]     a note by its name (no extension)
//	[[target|label]]   any of these, shown as label
//	#tag               a tag
package notes

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Link is one [[…]] link in a note.
type Link struct {
	Raw    string // as written between the brackets
	Target string // the path or name, before :line or #symbol
	Line   int    // the 1-based line after a colon, 0 none
	Symbol string // the name after #, "" none
	Label  string // after |, "" none
	At     Pos    // where the link starts in the note
}

// Pos is a place in a note: 0-based line and rune column.
type Pos struct{ Line, Col int }

var (
	wikiLink = regexp.MustCompile(`\[\[([^\[\]\n]+)\]\]`)
	tagRe    = regexp.MustCompile(`(?:^|[\s(])#([A-Za-z][\w/-]*)`)
	fence    = regexp.MustCompile("^[ \t]*(```|~~~)")
)

// Parse lists a note's links and tags, outside fenced code and inline code.
func Parse(text string) ([]Link, []string) {
	var links []Link
	tags := map[string]bool{}
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		if fence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		clean := blankInlineCode(line)
		for _, m := range wikiLink.FindAllStringSubmatchIndex(clean, -1) {
			raw := clean[m[2]:m[3]]
			l := parseLink(raw)
			l.At = Pos{Line: i, Col: len([]rune(clean[:m[0]]))}
			links = append(links, l)
		}
		trimmed := strings.TrimSpace(clean)
		if strings.HasPrefix(trimmed, "#") && strings.HasPrefix(strings.TrimLeft(trimmed, "#"), " ") {
			continue // a heading
		}
		for _, m := range tagRe.FindAllStringSubmatch(wikiLink.ReplaceAllString(clean, ""), -1) {
			tags[strings.ToLower(m[1])] = true
		}
	}
	out := make([]string, 0, len(tags))
	for t := range tags {
		out = append(out, t)
	}
	sort.Strings(out)
	return links, out
}

// blankInlineCode replaces `code` with spaces, keeping columns.
func blankInlineCode(s string) string {
	if !strings.Contains(s, "`") {
		return s
	}
	r := []rune(s)
	in := false
	for i, c := range r {
		if c == '`' {
			in = !in
			r[i] = ' '
			continue
		}
		if in {
			r[i] = ' '
		}
	}
	return string(r)
}

// ParseLink reads the text between [[ and ]]: target|label, target:line and target#symbol.
func ParseLink(raw string) Link { return parseLink(raw) }

// parseLink splits target|label, target:line and target#symbol.
func parseLink(raw string) Link {
	l := Link{Raw: raw}
	target := raw
	if i := strings.Index(target, "|"); i >= 0 {
		l.Label = strings.TrimSpace(target[i+1:])
		target = target[:i]
	}
	if i := strings.Index(target, "#"); i >= 0 {
		l.Symbol = strings.TrimSpace(target[i+1:])
		target = target[:i]
	}
	if i := strings.LastIndex(target, ":"); i > 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(target[i+1:])); err == nil && n > 0 {
			l.Line = n
			target = target[:i]
		}
	}
	l.Target = strings.TrimSpace(filepath.ToSlash(target))
	return l
}

// Resolved is where a link leads.
type Resolved struct {
	Path        string   // the file, slash-separated from the root; "" when not resolved
	Line        int      // 1-based; 0 for the top
	Note        bool     // the link is to a note
	Approximate bool     // the symbol was found by text search, not a language server
	Ambiguous   []string // several files have the base name: listed, never guessed
	Missing     bool     // nothing has that name
	NoSymbol    bool     // the file exists, the symbol was not found in it
}

// Files is what links resolve against: the project's files, slash-separated from the root.
type Files struct {
	Root     string
	NotesDir string
	paths    map[string]bool
	base     map[string][]string // lower-case base name -> paths
	noteName map[string][]string // lower-case note name (no .md) -> paths
}

// NewFiles indexes the project's files (slash-separated, from the root).
func NewFiles(root, notesDir string, paths []string) *Files {
	f := &Files{Root: root, NotesDir: notesDir, paths: map[string]bool{}, base: map[string][]string{}, noteName: map[string][]string{}}
	for _, p := range paths {
		f.Add(p)
	}
	return f
}

// Add records a file.
func (f *Files) Add(p string) {
	if f.paths[p] {
		return
	}
	f.paths[p] = true
	b := strings.ToLower(path.Base(p))
	f.base[b] = append(f.base[b], p)
	if strings.HasSuffix(b, ".md") {
		n := strings.TrimSuffix(b, ".md")
		f.noteName[n] = append(f.noteName[n], p)
	}
}

// Remove forgets a file.
func (f *Files) Remove(p string) {
	if !f.paths[p] {
		return
	}
	delete(f.paths, p)
	drop := func(m map[string][]string, k string) {
		m[k] = without(m[k], p)
		if len(m[k]) == 0 {
			delete(m, k)
		}
	}
	b := strings.ToLower(path.Base(p))
	drop(f.base, b)
	if strings.HasSuffix(b, ".md") {
		drop(f.noteName, strings.TrimSuffix(b, ".md"))
	}
}

// Has reports whether a file is known.
func (f *Files) Has(p string) bool { return f.paths[p] }

func without(s []string, x string) []string {
	out := s[:0]
	for _, v := range s {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

// SymbolFinder finds a symbol's line in a file through a language server; ok false when it
// cannot (no server, or the server does not know it).
type SymbolFinder func(path, symbol string) (line int, ok bool)

// Resolve ties a link made in note from (a path) to a file or note.
func (f *Files) Resolve(from string, l Link, symbols SymbolFinder) Resolved {
	t := l.Target
	if t == "" {
		return Resolved{Missing: true}
	}
	var r Resolved
	switch {
	case !strings.Contains(path.Base(t), "."): // [[other-note]]
		r = f.note(from, t)
	default:
		r = f.file(from, t)
	}
	if r.Path == "" {
		return r
	}
	r.Line = l.Line
	if l.Symbol != "" {
		if symbols != nil {
			if n, ok := symbols(r.Path, l.Symbol); ok {
				r.Line = n
				return r
			}
		}
		if n, ok := findDefinition(filepath.Join(f.Root, filepath.FromSlash(r.Path)), l.Symbol); ok {
			r.Line, r.Approximate = n, true
		} else {
			r.NoSymbol = true
		}
	}
	return r
}

// file finds a file: a path from the root, from the note's folder, or a unique base name.
func (f *Files) file(from, t string) Resolved {
	if strings.Contains(t, "/") {
		for _, c := range []string{path.Clean(t), path.Clean(path.Join(path.Dir(from), t))} {
			if !strings.HasPrefix(c, "../") && f.paths[c] {
				return Resolved{Path: c, Note: strings.HasSuffix(strings.ToLower(c), ".md")}
			}
		}
		// A path that does not exist may still name a unique file by its tail.
		if ps := f.endingWith(t); len(ps) == 1 {
			return Resolved{Path: ps[0], Note: strings.HasSuffix(strings.ToLower(ps[0]), ".md")}
		} else if len(ps) > 1 {
			return Resolved{Ambiguous: ps}
		}
		return Resolved{Missing: true}
	}
	ps := f.base[strings.ToLower(t)]
	switch len(ps) {
	case 0:
		return Resolved{Missing: true}
	case 1:
		return Resolved{Path: ps[0], Note: strings.HasSuffix(strings.ToLower(ps[0]), ".md")}
	}
	sorted := append([]string(nil), ps...)
	sort.Strings(sorted)
	return Resolved{Ambiguous: sorted}
}

// endingWith lists files whose path ends with /t.
func (f *Files) endingWith(t string) []string {
	var out []string
	suffix := "/" + strings.ToLower(strings.TrimPrefix(t, "./"))
	for p := range f.paths {
		if strings.HasSuffix("/"+strings.ToLower(p), suffix) {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// note finds a note by name: in the notes folder first, then anywhere; several outside the
// notes folder are ambiguous.
func (f *Files) note(from, name string) Resolved {
	if strings.Contains(name, "/") {
		return f.file(from, name+".md")
	}
	ps := f.noteName[strings.ToLower(name)]
	if len(ps) == 0 {
		return Resolved{Missing: true}
	}
	var inNotes []string
	for _, p := range ps {
		if f.NotesDir != "" && strings.HasPrefix(p, f.NotesDir+"/") {
			inNotes = append(inNotes, p)
		}
	}
	switch {
	case len(inNotes) == 1:
		return Resolved{Path: inNotes[0], Note: true}
	case len(ps) == 1:
		return Resolved{Path: ps[0], Note: true}
	}
	sorted := append([]string(nil), ps...)
	sort.Strings(sorted)
	return Resolved{Ambiguous: sorted}
}

// definitionLike are lines that define a name, in the languages a systems project uses. NAME
// is replaced by the quoted symbol.
var definitionLike = []string{
	`^\s*func\s+(\([^)]*\)\s*)?NAME\b`, // Go
	`^\s*(type|var|const)\s+NAME\b`,    // Go
	`^\s*(pub(\([\w:]+\))?\s+)?(fn|struct|enum|trait|type|const|static|mod)\s+NAME\b`, // Rust
	`^\s*(async\s+)?(def|class)\s+NAME\b`,                                             // Python
	`^\s*#\s*define\s+NAME\b`,                                                         // C macros
	`^\s*(typedef\s+)?(struct|union|enum|class)\s+NAME\b`,                             // C, C++
	`^[A-Za-z_][\w\s\*&:<>,]*[\s\*&]NAME\s*\([^;]*$`,                                  // a C function definition
	`^[A-Za-z_][\w\s\*]*[\s\*]NAME\s*(=|;|\[)`,                                        // a C variable at file scope
	`^\s*\.?NAME:`,                         // an assembly label
	`^\s*(global|%macro|%define)\s+NAME\b`, // NASM
	`^\s*(export\s+)?(function|class|const|let|interface|type)\s+NAME\b`, // JS, TS
}

// findDefinition finds the first definition-like line for a symbol by text search.
func findDefinition(file, symbol string) (int, bool) {
	if symbol == "" {
		return 0, false
	}
	var res []*regexp.Regexp
	for _, p := range definitionLike {
		res = append(res, regexp.MustCompile(strings.ReplaceAll(p, "NAME", regexp.QuoteMeta(symbol))))
	}
	fh, err := os.Open(file)
	if err != nil {
		return 0, false
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if !strings.Contains(line, symbol) {
			continue
		}
		for _, re := range res {
			if re.MatchString(line) {
				return n, true
			}
		}
	}
	return 0, false
}
