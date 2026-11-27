package graph

import (
	"regexp"
	"strings"
)

// ref is a reference a parser found, as written.
type ref struct {
	ref    string
	kind   string // "include", "import", "mod", "use", "link", "wikilink", "tag"
	lang   string // "c", "nasm", "go", "rust", "python", "js", "md"
	quoted bool   // C: #include "…" rather than <…>
}

// parserFor is the parser for a file extension, nil for files that state no references.
func parserFor(ext string) func(string) []ref {
	switch ext {
	case "c", "h", "cc", "cpp", "cxx", "hpp", "hh", "hxx", "c++", "inl":
		return parseC
	case "asm", "nasm", "s", "inc", "mac":
		return parseNASM
	case "go":
		return parseGo
	case "rs":
		return parseRust
	case "py", "pyi":
		return parsePython
	case "js", "jsx", "ts", "tsx", "mjs", "cjs", "mts", "cts":
		return parseJS
	case "md", "markdown":
		return parseMarkdown
	}
	return nil
}

var (
	cInclude    = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*include[ \t]*(?:"([^"\n]+)"|<([^>\n]+)>)`)
	nasmInclude = regexp.MustCompile(`(?mi)^[ \t]*%include[ \t]+(?:"([^"\n]+)"|'([^'\n]+)')`)
)

// parseC reads #include "…" and #include <…>.
func parseC(src string) []ref {
	var out []ref
	for _, m := range cInclude.FindAllStringSubmatch(stripCComments(src), -1) {
		if m[1] != "" {
			out = append(out, ref{ref: m[1], kind: "include", lang: "c", quoted: true})
		} else {
			out = append(out, ref{ref: m[2], kind: "include", lang: "c"})
		}
	}
	return out
}

// stripCComments blanks /* … */ comments, so an include inside one is not read. Strings are
// not tracked: an include directive cannot be inside one.
func stripCComments(s string) string {
	if !strings.Contains(s, "/*") {
		return s
	}
	b := []byte(s)
	for i := 0; i+1 < len(b); i++ {
		if b[i] == '/' && b[i+1] == '*' {
			j := i + 2
			for j+1 < len(b) && !(b[j] == '*' && b[j+1] == '/') {
				j++
			}
			for k := i; k < min(j+2, len(b)); k++ {
				if b[k] != '\n' {
					b[k] = ' '
				}
			}
			i = j + 1
		}
	}
	return string(b)
}

// parseNASM reads %include "…" and %include '…'.
func parseNASM(src string) []ref {
	var out []ref
	for _, m := range nasmInclude.FindAllStringSubmatch(src, -1) {
		r := m[1]
		if r == "" {
			r = m[2]
		}
		out = append(out, ref{ref: r, kind: "include", lang: "nasm"})
	}
	return out
}

var (
	goImportOne   = regexp.MustCompile(`(?m)^import[ \t]+(?:[\w.]+[ \t]+)?"([^"]+)"`)
	goImportBlock = regexp.MustCompile(`(?ms)^import[ \t]*\((.*?)\)`)
	goImportLine  = regexp.MustCompile(`(?m)^[ \t]*(?:[\w.]+[ \t]+)?"([^"]+)"`)
)

// parseGo reads a Go file's imports.
func parseGo(src string) []ref {
	var out []ref
	for _, m := range goImportOne.FindAllStringSubmatch(src, -1) {
		out = append(out, ref{ref: m[1], kind: "import", lang: "go"})
	}
	for _, blk := range goImportBlock.FindAllStringSubmatch(src, -1) {
		for _, m := range goImportLine.FindAllStringSubmatch(blk[1], -1) {
			out = append(out, ref{ref: m[1], kind: "import", lang: "go"})
		}
	}
	return out
}

var (
	rustMod = regexp.MustCompile(`(?m)^[ \t]*(?:pub(?:\([\w: ]+\))?[ \t]+)?mod[ \t]+(\w+)[ \t]*;`)
	rustUse = regexp.MustCompile(`\buse[ \t]+crate::([\w:]+)`)
)

// parseRust reads `mod name;` declarations and `use crate::…` paths.
func parseRust(src string) []ref {
	var out []ref
	for _, m := range rustMod.FindAllStringSubmatch(src, -1) {
		out = append(out, ref{ref: m[1], kind: "mod", lang: "rust"})
	}
	for _, m := range rustUse.FindAllStringSubmatch(src, -1) {
		out = append(out, ref{ref: strings.TrimSuffix(m[1], "::"), kind: "use", lang: "rust"})
	}
	return out
}

var (
	pyFrom   = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+(\.+[\w.]*|[\w.]+)[ \t]+import[ \t]+\(?([\w ,*\t]+)`)
	pyImport = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+([\w.]+(?:[ \t]*,[ \t]*[\w.]+)*)`)
)

// parsePython reads `import a.b` and `from .x import y` statements. For `from pkg import
// name` the names are tried as submodules too ("pkg.name").
func parsePython(src string) []ref {
	var out []ref
	for _, m := range pyFrom.FindAllStringSubmatch(src, -1) {
		mod := m[1]
		out = append(out, ref{ref: mod, kind: "import", lang: "python"})
		for _, name := range strings.Split(m[2], ",") {
			name = strings.TrimSpace(strings.Fields(name + " ")[0])
			if name == "" || name == "*" {
				continue
			}
			sep := "."
			if strings.HasSuffix(mod, ".") {
				sep = ""
			}
			out = append(out, ref{ref: mod + sep + name, kind: "import?", lang: "python"})
		}
	}
	for _, m := range pyImport.FindAllStringSubmatch(src, -1) {
		for _, mod := range strings.Split(m[1], ",") {
			out = append(out, ref{ref: strings.TrimSpace(mod), kind: "import", lang: "python"})
		}
	}
	return out
}

var jsImports = []*regexp.Regexp{
	regexp.MustCompile(`(?m)\b(?:import|export)\b[^'"\n;]*?\bfrom[ \t]*['"]([^'"\n]+)['"]`),
	regexp.MustCompile(`(?m)\bimport[ \t]*['"]([^'"\n]+)['"]`),
	regexp.MustCompile(`\brequire\([ \t]*['"]([^'"\n]+)['"][ \t]*\)`),
	regexp.MustCompile(`\bimport\([ \t]*['"]([^'"\n]+)['"][ \t]*\)`),
}

// parseJS reads imports, re-exports, require() and import() of relative paths; packages are
// outside the project and not references.
func parseJS(src string) []ref {
	var out []ref
	for _, re := range jsImports {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			if strings.HasPrefix(m[1], "./") || strings.HasPrefix(m[1], "../") {
				out = append(out, ref{ref: m[1], kind: "import", lang: "js"})
			}
		}
	}
	return out
}

var (
	mdLink   = regexp.MustCompile(`\[[^\]\n]*\]\(<?([^)\s>]+)>?(?:[ \t]+"[^"]*")?\)`)
	mdWiki   = regexp.MustCompile(`\[\[([^\]|#\n]+)(?:#[^\]|\n]*)?(?:\|[^\]\n]*)?\]\]`)
	mdTag    = regexp.MustCompile(`(?:^|[ \t(])#([A-Za-z][\w/-]*)`)
	mdFence  = regexp.MustCompile("(?ms)^[ \t]*(```|~~~).*?^[ \t]*(```|~~~)[ \t]*$")
	mdInline = regexp.MustCompile("`[^`\n]*`")
)

// parseMarkdown reads links to files, [[wikilinks]] and #tags, outside code.
func parseMarkdown(src string) []ref {
	src = mdFence.ReplaceAllString(src, "")
	src = mdInline.ReplaceAllString(src, "")
	var out []ref
	for _, m := range mdLink.FindAllStringSubmatch(src, -1) {
		target := m[1]
		if i := strings.IndexAny(target, "#?"); i >= 0 {
			target = target[:i]
		}
		if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		out = append(out, ref{ref: target, kind: "link", lang: "md"})
	}
	for _, m := range mdWiki.FindAllStringSubmatch(src, -1) {
		out = append(out, ref{ref: strings.TrimSpace(m[1]), kind: "wikilink", lang: "md"})
	}
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") && strings.HasPrefix(strings.TrimLeft(strings.TrimSpace(line), "#"), " ") {
			continue // a heading, not a tag
		}
		for _, m := range mdTag.FindAllStringSubmatch(line, -1) {
			out = append(out, ref{ref: strings.ToLower(m[1]), kind: "tag", lang: "md"})
		}
	}
	return out
}
