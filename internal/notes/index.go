package notes

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Backlink is a note's link to a file, with the line it is on.
type Backlink struct {
	Note    string // the note, slash-separated from the root
	Line    int    // 0-based line of the link in the note
	Context string // that line, trimmed
	Link    Link
	To      Resolved
}

// Index knows every Markdown file's links and which notes link to each file.
type Index struct {
	mu      sync.Mutex
	files   *Files
	links   map[string][]Link     // note -> its links, as parsed
	tags    map[string][]string   // note -> its tags
	text    map[string][]string   // note -> its lines, for context
	back    map[string][]Backlink // target path -> links to it
	issues  map[string][]Backlink // note -> its links that did not resolve
	symbols SymbolFinder
}

// skipped folders, as the graph skips them.
var skip = map[string]bool{".git": true, "node_modules": true, "target": true, "vendor": true, "dist": true,
	"build": true, "out": true, "__pycache__": true, ".venv": true}

// Build walks root, indexes every file and reads every Markdown file's links.
func Build(root, notesDir string, symbols SymbolFinder) (*Index, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (skip[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, err := filepath.Rel(root, p); err == nil && d.Type().IsRegular() {
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	x := &Index{files: NewFiles(root, notesDir, paths), links: map[string][]Link{}, tags: map[string][]string{},
		text: map[string][]string{}, symbols: symbols}
	for _, p := range paths {
		if isNote(p) {
			x.read(p)
		}
	}
	x.resolveAll()
	return x, nil
}

func isNote(p string) bool {
	l := strings.ToLower(p)
	return strings.HasSuffix(l, ".md") || strings.HasSuffix(l, ".markdown")
}

// read parses one note from disk.
func (x *Index) read(p string) {
	data, err := os.ReadFile(filepath.Join(x.files.Root, filepath.FromSlash(p)))
	if err != nil {
		delete(x.links, p)
		delete(x.tags, p)
		delete(x.text, p)
		return
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	links, tags := Parse(text)
	x.links[p], x.tags[p], x.text[p] = links, tags, strings.Split(text, "\n")
}

// resolveAll ties every note's links to their targets again.
func (x *Index) resolveAll() {
	x.back, x.issues = map[string][]Backlink{}, map[string][]Backlink{}
	notes := make([]string, 0, len(x.links))
	for n := range x.links {
		notes = append(notes, n)
	}
	sort.Strings(notes)
	for _, n := range notes {
		for _, l := range x.links[n] {
			r := x.files.Resolve(n, l, x.symbols)
			ctx := ""
			if lines := x.text[n]; l.At.Line < len(lines) {
				ctx = strings.TrimSpace(lines[l.At.Line])
			}
			b := Backlink{Note: n, Line: l.At.Line, Context: ctx, Link: l, To: r}
			if r.Path == "" || r.NoSymbol {
				x.issues[n] = append(x.issues[n], b)
			}
			if r.Path != "" {
				x.back[r.Path] = append(x.back[r.Path], b)
			}
		}
	}
}

// Update takes the files that changed on disk (absolute paths, as the explorer's watcher
// reports them): new and deleted files change what links resolve to, and changed notes are
// read again. A rename on disk is a delete and a create.
func (x *Index) Update(changed []string) bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	touched := false
	for _, p := range changed {
		rel, err := filepath.Rel(x.files.Root, p)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		id := filepath.ToSlash(rel)
		if hidden(id) {
			continue
		}
		fi, err := os.Stat(p)
		switch {
		case err != nil:
			if x.files.Has(id) {
				x.files.Remove(id)
				delete(x.links, id)
				delete(x.tags, id)
				delete(x.text, id)
				touched = true
			}
		case fi.Mode().IsRegular():
			x.files.Add(id)
			if isNote(id) {
				x.read(id)
			}
			touched = true
		}
	}
	if touched {
		x.resolveAll()
	}
	return touched
}

func hidden(id string) bool {
	for _, part := range strings.Split(id, "/") {
		if skip[part] || strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// Backlinks are the links to a file, by note and line.
func (x *Index) Backlinks(target string) []Backlink {
	x.mu.Lock()
	defer x.mu.Unlock()
	out := append([]Backlink(nil), x.back[target]...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Note != out[j].Note {
			return out[i].Note < out[j].Note
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// Issues are a note's links that lead nowhere: missing, ambiguous, or a symbol not found.
func (x *Index) Issues(note string) []Backlink {
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([]Backlink(nil), x.issues[note]...)
}

// Tags are a note's tags.
func (x *Index) Tags(note string) []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.tags[note]
}

// Files is the index's view of the project's files.
func (x *Index) Files() *Files { return x.files }

// Resolve ties a link from a note, with the index's language-server lookup.
func (x *Index) Resolve(from string, l Link) Resolved {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.files.Resolve(from, l, x.symbols)
}

// Mention is a file's base name in a note's text without a link to it.
type Mention struct {
	Name    string // as it appears
	Path    string // the file it names
	Line    int    // 0-based
	Context string
}

var wordish = regexp.MustCompile(`[\w][\w.-]*\.[A-Za-z0-9]+`)

// UnlinkedMentions lists the project files a note names in its text (by base name, when
// exactly one file has it) without linking to them, outside code and outside links.
func (x *Index) UnlinkedMentions(note string) []Mention {
	x.mu.Lock()
	defer x.mu.Unlock()
	linked := map[string]bool{}
	for _, l := range x.links[note] {
		if r := x.files.Resolve(note, l, nil); r.Path != "" {
			linked[r.Path] = true
		}
	}
	var out []Mention
	seen := map[string]bool{}
	inFence := false
	for i, line := range x.text[note] {
		if fence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		clean := wikiLink.ReplaceAllStringFunc(blankInlineCode(line), func(s string) string { return strings.Repeat(" ", len(s)) })
		clean = mdLinkRe.ReplaceAllStringFunc(clean, func(s string) string { return strings.Repeat(" ", len(s)) })
		for _, w := range wordish.FindAllString(clean, -1) {
			ps := x.files.base[strings.ToLower(w)]
			if len(ps) != 1 || ps[0] == note || linked[ps[0]] || seen[ps[0]] {
				continue
			}
			seen[ps[0]] = true
			out = append(out, Mention{Name: w, Path: ps[0], Line: i, Context: strings.TrimSpace(line)})
		}
	}
	return out
}

var mdLinkRe = regexp.MustCompile(`\[[^\]\n]*\]\([^)\n]*\)`)

// LinkTo is the link text for a place in a file, as New Note from Selection and Insert Link
// to Current Location write it: [[path:line]], by base name when that is unique.
func (x *Index) LinkTo(p string, line int) string {
	x.mu.Lock()
	defer x.mu.Unlock()
	target := p
	if ps := x.files.base[strings.ToLower(path.Base(p))]; len(ps) == 1 {
		target = path.Base(p)
	}
	if line > 0 {
		return "[[" + target + ":" + itoa(line) + "]]"
	}
	return "[[" + target + "]]"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
