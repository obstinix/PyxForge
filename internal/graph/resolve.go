package graph

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/obstinix/PyxForge/internal/config"
)

// env is what resolving needs to know about the project: its files, include folders, Go
// modules, Rust crates and note names.
type env struct {
	files    map[string]bool
	include  []string
	modules  map[string]string // Go module path -> its folder ("" the root)
	crates   []string          // folders holding a Cargo.toml, longest first
	notes    map[string][]string
	byBase   map[string][]string // lower-case base name -> files
	goDocs   map[string]string   // Go package folder -> the file that represents it
	goLister map[string][]string
}

func newEnv(g *Graph, o Options) *env {
	e := &env{include: o.IncludeDirs}
	e.index(g)
	return e
}

// index records the files and what can be learned from their names.
func (e *env) index(g *Graph) {
	e.files = map[string]bool{}
	e.modules = map[string]string{}
	e.notes = map[string][]string{}
	e.byBase = map[string][]string{}
	e.goLister = map[string][]string{}
	e.goDocs = nil
	e.crates = nil
	for _, n := range g.Nodes {
		e.addFile(n.ID)
	}
	for id := range e.files {
		switch path.Base(id) {
		case "go.mod":
			if mod := goModule(filepath.Join(g.Root, filepath.FromSlash(id))); mod != "" {
				e.modules[mod] = dirOf(id)
			}
		case "Cargo.toml":
			e.crates = append(e.crates, dirOf(id))
		}
	}
	sort.Slice(e.crates, func(i, j int) bool { return len(e.crates[i]) > len(e.crates[j]) })
}

func (e *env) addFile(id string) {
	e.files[id] = true
	base := strings.ToLower(path.Base(id))
	e.byBase[base] = append(e.byBase[base], id)
	if strings.HasSuffix(base, ".md") {
		name := strings.TrimSuffix(base, ".md")
		e.notes[name] = append(e.notes[name], id)
	}
	if strings.HasSuffix(base, ".go") && !strings.HasSuffix(base, "_test.go") {
		d := dirOf(id)
		e.goLister[d] = append(e.goLister[d], id)
		e.goDocs = nil
	}
}

// goModule reads the module path from a go.mod.
func goModule(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// clean joins a folder and a relative path into a node ID, false when it leaves the root.
func clean(dir, rel string) (string, bool) {
	p := path.Clean(path.Join(dir, rel))
	if p == ".." || strings.HasPrefix(p, "../") || path.IsAbs(p) {
		return "", false
	}
	return strings.TrimPrefix(p, "./"), true
}

func (e *env) has(id string) bool { return e.files[id] }

// resolve ties a reference made in file from to a file in the project. ok false with a
// reference recorded as unresolved means it looked like the project's and was not found;
// references to the system or to packages are dropped without a record (unresolvable but not
// the project's).
func (e *env) resolve(from string, r ref) (string, bool) {
	dir := dirOf(from)
	switch r.lang {
	case "c", "nasm":
		cands := []string{dir}
		if r.lang == "c" && !r.quoted {
			cands = nil // <…> starts at the include folders
		}
		cands = append(cands, e.include...)
		cands = append(cands, "")
		for _, d := range cands {
			if id, ok := clean(d, r.ref); ok && e.has(id) {
				return id, true
			}
		}
	case "go":
		return e.resolveGo(r.ref)
	case "rust":
		return e.resolveRust(from, r)
	case "python":
		return e.resolvePython(from, r.ref)
	case "js":
		base, ok := clean(dir, r.ref)
		if !ok {
			return "", false
		}
		for _, suf := range []string{"", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts", "/index.ts", "/index.tsx", "/index.js", "/index.jsx"} {
			if e.has(base + suf) {
				return base + suf, true
			}
		}
	case "md":
		if r.kind == "wikilink" {
			// Forge Notes (D25): [[file.c]], [[file.c:42]] and [[path/file.c]] link to files.
			ref := strings.TrimSpace(r.ref)
			if i := strings.LastIndex(ref, ":"); i > 0 && strings.Trim(ref[i+1:], "0123456789") == "" {
				ref = ref[:i]
			}
			if ext := strings.ToLower(path.Ext(ref)); ext != "" && ext != ".md" {
				if strings.Contains(ref, "/") {
					for _, c := range []string{strings.TrimPrefix(path.Clean(ref), "./"), ""} {
						if c == "" {
							if id, ok := clean(dir, ref); ok && e.has(id) {
								return id, true
							}
							continue
						}
						if e.has(c) {
							return c, true
						}
					}
					return "", false
				}
				if ids := e.byBase[strings.ToLower(ref)]; len(ids) == 1 {
					return ids[0], true
				}
				return "", false // missing, or several files of that name: never guessed
			}
			name := strings.ToLower(strings.TrimSuffix(ref, ".md"))
			if strings.Contains(name, "/") { // [[folder/note]]
				for _, id := range e.notes[path.Base(name)] {
					if strings.HasSuffix(strings.ToLower(strings.TrimSuffix(id, ".md")), name) {
						return id, true
					}
				}
				return "", false
			}
			ids := e.notes[name]
			if len(ids) == 0 {
				return "", false
			}
			// The shortest path, as Obsidian picks among notes of one name.
			best := ids[0]
			for _, id := range ids[1:] {
				if len(id) < len(best) || len(id) == len(best) && id < best {
					best = id
				}
			}
			return best, true
		}
		target := r.ref
		if u, err := unescapeURL(target); err == nil {
			target = u
		}
		if strings.HasPrefix(target, "/") { // from the project root
			target = strings.TrimPrefix(target, "/")
			if e.has(target) {
				return target, true
			}
			return "", false
		}
		if id, ok := clean(dir, target); ok && e.has(id) {
			return id, true
		}
	}
	return "", false
}

// unescapeURL decodes %20 and the like in a Markdown link.
func unescapeURL(s string) (string, error) {
	if !strings.Contains(s, "%") {
		return s, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			var v byte
			ok := true
			for _, c := range []byte(s[i+1 : i+3]) {
				v <<= 4
				switch {
				case c >= '0' && c <= '9':
					v |= c - '0'
				case c >= 'a' && c <= 'f':
					v |= c - 'a' + 10
				case c >= 'A' && c <= 'F':
					v |= c - 'A' + 10
				default:
					ok = false
				}
			}
			if ok {
				b.WriteByte(v)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String(), nil
}

// resolveGo ties an import path inside one of the project's modules to the file that
// represents its package: the one with the package comment, else the first by name. One edge
// per imported package keeps the graph readable; a package's files are linked by its folder.
func (e *env) resolveGo(imp string) (string, bool) {
	for mod, root := range e.modules {
		rest, ok := strings.CutPrefix(imp, mod)
		if !ok || (rest != "" && rest[0] != '/') {
			continue
		}
		d := path.Join(root, strings.TrimPrefix(rest, "/"))
		if d == "." {
			d = ""
		}
		if id := e.goDoc(d); id != "" {
			return id, true
		}
		return "", false // inside the module, yet not found: recorded
	}
	return "", false
}

// goDoc is the file standing for the Go package in folder d.
func (e *env) goDoc(d string) string {
	if e.goDocs == nil {
		e.goDocs = map[string]string{}
	}
	if id, ok := e.goDocs[d]; ok {
		return id
	}
	files := append([]string(nil), e.goLister[d]...)
	sort.Strings(files)
	pick := ""
	for _, id := range files {
		if path.Base(id) == "doc.go" {
			pick = id
			break
		}
	}
	if pick == "" && len(files) > 0 {
		pick = files[0]
	}
	e.goDocs[d] = pick
	return pick
}

// resolveRust ties `mod name;` to name.rs or name/mod.rs beside the file (in the file's own
// folder for mod.rs, lib.rs and main.rs; in a folder named after the file otherwise), and
// `use crate::a::b` to src/a.rs, src/a/mod.rs or src/a/b.rs of the file's crate.
func (e *env) resolveRust(from string, r ref) (string, bool) {
	dir := dirOf(from)
	if r.kind == "mod" {
		base := path.Base(from)
		if base != "mod.rs" && base != "lib.rs" && base != "main.rs" {
			dir = path.Join(dir, strings.TrimSuffix(base, ".rs"))
		}
		for _, c := range []string{r.ref + ".rs", r.ref + "/mod.rs"} {
			if id, ok := clean(dir, c); ok && e.has(id) {
				return id, true
			}
		}
		return "", false
	}
	crate := ""
	for _, c := range e.crates {
		if c == "" || strings.HasPrefix(from, c+"/") {
			crate = c
			break
		}
	}
	src := path.Join(crate, "src")
	parts := strings.Split(r.ref, "::")
	for n := len(parts); n >= 1; n-- { // the longest module path that is a file
		p := path.Join(append([]string{src}, parts[:n]...)...)
		for _, c := range []string{p + ".rs", p + "/mod.rs"} {
			if e.has(strings.TrimPrefix(c, "./")) {
				return strings.TrimPrefix(c, "./"), true
			}
		}
	}
	return "", false
}

// resolvePython ties a module name to a file: relative names (".x", "..pkg.y") against the
// file's package, absolute ones against the root and the file's top folders (a src layout).
// "import?" refs (from pkg import name) are tried as submodules and dropped quietly.
func (e *env) resolvePython(from, mod string) (string, bool) {
	try := func(base string) (string, bool) {
		for _, c := range []string{base + ".py", base + ".pyi", base + "/__init__.py"} {
			c = strings.TrimPrefix(c, "/")
			if e.has(c) {
				return c, true
			}
		}
		return "", false
	}
	if strings.HasPrefix(mod, ".") {
		dots := len(mod) - len(strings.TrimLeft(mod, "."))
		dir := dirOf(from)
		for range dots - 1 {
			dir = dirOf(dir)
		}
		rest := strings.ReplaceAll(strings.TrimLeft(mod, "."), ".", "/")
		if rest == "" {
			return try(dir + "/__init__")
		}
		return try(path.Join(dir, rest))
	}
	rel := strings.ReplaceAll(mod, ".", "/")
	for d := dirOf(from); ; d = dirOf(d) {
		if id, ok := try(path.Join(d, rel)); ok {
			return id, true
		}
		if d == "" {
			break
		}
	}
	return "", false
}

// IncludeDirs are the folders a project's build profiles pass with -I or -i (GCC, Clang,
// NASM), slash-separated from the root, read from pyxforge.toml.
func IncludeDirs(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, name := range cfg.ProfileNames() {
		p := cfg.Profiles[name]
		src := filepath.ToSlash(p.SourceDir)
		for i := 0; i < len(p.Args); i++ {
			a := p.Args[i]
			var d string
			switch {
			case a == "-I" || a == "-i":
				if i+1 < len(p.Args) {
					d = p.Args[i+1]
					i++
				}
			case strings.HasPrefix(a, "-I") || strings.HasPrefix(a, "-i") && len(a) > 2 && !strings.HasPrefix(a, "-include") && !strings.HasPrefix(a, "-isystem"):
				d = a[2:]
			case strings.HasPrefix(a, "-isystem"):
				d = strings.TrimPrefix(a, "-isystem")
			}
			if d == "" {
				continue
			}
			d = filepath.ToSlash(d)
			if !path.IsAbs(d) {
				d = path.Clean(path.Join(src, d))
			}
			d = strings.TrimSuffix(strings.TrimPrefix(d, "./"), "/")
			if d == "." {
				d = ""
			}
			if !seen[d] && !strings.HasPrefix(d, "..") && !path.IsAbs(d) {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out
}

// ours reports whether an unresolved reference looked like one to the project's own files, so
// it is listed as unresolved: a quoted C include, a NASM include, an import inside one of the
// project's Go modules, a Rust module, a relative Python or JavaScript import, a Markdown link.
// System headers, the standard library and packages are not the project's.
func (e *env) ours(r ref) bool {
	switch r.lang {
	case "c":
		return r.quoted
	case "go":
		for mod := range e.modules {
			if r.ref == mod || strings.HasPrefix(r.ref, mod+"/") {
				return true
			}
		}
		return false
	case "python":
		return r.kind == "import" && strings.HasPrefix(r.ref, ".")
	}
	return true
}
