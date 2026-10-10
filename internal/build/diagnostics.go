package build

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Diagnostic is one problem a build tool reported. Line and Column are 1-based as tools print
// them; 0 means absent. File is empty for messages about no particular source file, such as a
// linker that cannot find a library; Tool then says which tool reported it.
type Diagnostic struct {
	File      string `json:"file"`
	Tool      string `json:"tool,omitempty"`
	Line      int    `json:"line"`
	Column    int    `json:"column,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	Severity  string `json:"severity"` // "error", "warning", "note" or "help"
	Message   string `json:"message"`
}

// ParseDiagnostics reads a build's output. Cargo's JSON messages are authoritative when there
// are any (2.x parity: diagnostics.rs). Otherwise structured compiler output is preferred,
// SARIF (GCC's -fdiagnostics-format=sarif-stderr, Clang's -fdiagnostics-format=sarif) or GCC's
// JSON, joined by the text lines it does not cover, such as the linker's. Text is read in the
// GNU form (NASM, GCC, Clang, ld). Anything else is skipped, so parsing never fails.
func ParseDiagnostics(output string) []Diagnostic {
	if d := ParseCargoJSON(output); len(d) > 0 {
		return d
	}
	structured := append(ParseSARIF(output), ParseGCCJSON(output)...)
	text := ParseGNU(output)
	if len(structured) == 0 {
		return text
	}
	key := func(d Diagnostic) string {
		return strings.ToLower(filepath.Base(d.File)) + "\x00" + strconv.Itoa(d.Line) + "\x00" + d.Message
	}
	seen := map[string]bool{}
	for _, d := range structured {
		seen[key(d)] = true
	}
	for _, d := range text {
		if !seen[key(d)] {
			structured = append(structured, d)
		}
	}
	return structured
}

// ParseGNU reads `file:line[:col]: severity: message` lines, the linker's
// `file:(section+offset): message`, and messages that name a tool instead of a source
// position, such as `nasm: fatal: unable to open input file` or `ld: cannot find -lfoo`.
func ParseGNU(output string) []Diagnostic {
	var out []Diagnostic
	for line := range strings.SplitSeq(output, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[{") {
			continue // structured output is read by its own parser
		}
		if d, ok := parseGNULine(line); ok {
			out = append(out, d)
		} else if d, ok := parseUnlocated(line); ok {
			out = append(out, d)
		}
	}
	return out
}

func parseGNULine(raw string) (Diagnostic, bool) {
	line := strings.TrimSpace(raw)
	// The file ends at the first ":<digits>", after a Windows drive letter if there is one.
	start := 0
	if len(line) >= 3 && isLetter(line[0]) && line[1] == ':' && (line[2] == '\\' || line[2] == '/') {
		start = 3
	}
	colon := -1
	for i := start; i < len(line)-1; i++ {
		if line[i] == ':' && isDigit(line[i+1]) {
			colon = i
			break
		}
	}
	if colon <= 0 {
		return Diagnostic{}, false
	}
	d := Diagnostic{File: line[:colon]}
	rest := line[colon+1:]
	d.Line, rest = leadingNumber(rest)
	if len(rest) > 1 && rest[0] == ':' && isDigit(rest[1]) {
		d.Column, rest = leadingNumber(rest[1:])
	}
	rest = strings.TrimSpace(strings.TrimLeft(rest, ":"))
	lower := strings.ToLower(rest)
	for _, k := range []struct{ prefix, severity string }{
		{"fatal error:", "error"}, {"error:", "error"}, {"error ", "error"},
		{"warning:", "warning"}, {"warning ", "warning"}, {"note:", "note"},
	} {
		if strings.HasPrefix(lower, k.prefix) {
			d.Severity = k.severity
			d.Message = strings.TrimSpace(rest[len(k.prefix):])
			return d, d.Message != ""
		}
	}
	return Diagnostic{}, false
}

var (
	// collect2: error: ld returned 1 exit status · nasm: fatal: unable to open input file
	toolSeverityRE = regexp.MustCompile(`^([A-Za-z0-9_.+-]+): (fatal error|fatal|error|warning): (.+)$`)
	// /usr/bin/ld: cannot find -lfoo · x86_64-elf-ld: warning: … · ld.lld: error: undefined symbol: f
	linkerRE = regexp.MustCompile(`^(?:.*[/\\])?((?:[A-Za-z0-9_]+-)*(?:ld|ld\.bfd|ld\.gold|ld\.lld|lld|lld-link|collect2)(?:\.exe)?): (.+)$`)
	// y.c:(.text+0x9): undefined reference to `f'
	sectionRefRE = regexp.MustCompile(`^(.+?):\(([^)]*)\): (.+)$`)
)

// parseUnlocated reads messages that name a tool or an object file's section, not a line.
func parseUnlocated(raw string) (Diagnostic, bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasSuffix(line, "':") { // "in function `main':" introduces the next line
		return Diagnostic{}, false
	}
	if m := sectionRefRE.FindStringSubmatch(line); m != nil {
		return Diagnostic{File: m[1], Tool: "ld", Severity: "error", Message: m[3] + " (" + m[2] + ")"}, true
	}
	if m := toolSeverityRE.FindStringSubmatch(line); m != nil {
		sev := "error"
		if m[2] == "warning" {
			sev = "warning"
		}
		return Diagnostic{Tool: strings.TrimSuffix(m[1], ".exe"), Severity: sev, Message: m[3]}, true
	}
	if m := linkerRE.FindStringSubmatch(line); m != nil {
		msg, sev := m[2], "error"
		switch {
		case strings.HasPrefix(msg, "warning: "):
			msg, sev = strings.TrimPrefix(msg, "warning: "), "warning"
		case strings.HasPrefix(msg, "error: "):
			msg = strings.TrimPrefix(msg, "error: ")
		}
		return Diagnostic{Tool: strings.TrimSuffix(m[1], ".exe"), Severity: sev, Message: msg}, true
	}
	return Diagnostic{}, false
}

func leadingNumber(s string) (int, string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	n, _ := strconv.Atoi(s[:i])
	return n, s[i:]
}

func isDigit(b byte) bool  { return b >= '0' && b <= '9' }
func isLetter(b byte) bool { return b|0x20 >= 'a' && b|0x20 <= 'z' }

// ParseCargoJSON reads `cargo --message-format=json` output: compiler messages with their
// primary span.
func ParseCargoJSON(output string) []Diagnostic {
	type span struct {
		FileName    string `json:"file_name"`
		LineStart   int    `json:"line_start"`
		LineEnd     int    `json:"line_end"`
		ColumnStart int    `json:"column_start"`
		ColumnEnd   int    `json:"column_end"`
		IsPrimary   bool   `json:"is_primary"`
	}
	type message struct {
		Reason  string `json:"reason"`
		Message *struct {
			Message string `json:"message"`
			Level   string `json:"level"`
			Spans   []span `json:"spans"`
		} `json:"message"`
	}
	var out []Diagnostic
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var m message
		if json.Unmarshal([]byte(line), &m) != nil || m.Reason != "compiler-message" || m.Message == nil || len(m.Message.Spans) == 0 {
			continue
		}
		sp := m.Message.Spans[0]
		for _, s := range m.Message.Spans {
			if s.IsPrimary {
				sp = s
				break
			}
		}
		if sp.FileName == "" {
			continue
		}
		sev := "error"
		switch m.Message.Level {
		case "warning", "note", "help":
			sev = m.Message.Level
		}
		msg := m.Message.Message
		if msg == "" {
			msg = "Compiler error"
		}
		out = append(out, Diagnostic{File: sp.FileName, Line: max(sp.LineStart, 1), Column: sp.ColumnStart,
			EndLine: sp.LineEnd, EndColumn: sp.ColumnEnd, Severity: sev, Message: msg})
	}
	return out
}

// ParseSARIF reads SARIF 2.1.0 logs, one JSON document per line, as GCC (sarif-stderr) and
// Clang (-fdiagnostics-format=sarif) print them.
func ParseSARIF(output string) []Diagnostic {
	type location struct {
		PhysicalLocation struct {
			ArtifactLocation struct {
				URI       string `json:"uri"`
				URIBaseID string `json:"uriBaseId"`
			} `json:"artifactLocation"`
			Region struct {
				StartLine   int `json:"startLine"`
				StartColumn int `json:"startColumn"`
				EndLine     int `json:"endLine"`
				EndColumn   int `json:"endColumn"`
			} `json:"region"`
		} `json:"physicalLocation"`
	}
	type log struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Name string `json:"name"`
				} `json:"driver"`
			} `json:"tool"`
			OriginalURIBaseIDs map[string]struct {
				URI string `json:"uri"`
			} `json:"originalUriBaseIds"`
			Results []struct {
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []location `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	var out []Diagnostic
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") || !strings.Contains(line, `"runs"`) {
			continue
		}
		var l log
		if json.Unmarshal([]byte(line), &l) != nil {
			continue
		}
		for _, run := range l.Runs {
			for _, r := range run.Results {
				d := Diagnostic{Severity: sarifLevel(r.Level), Message: r.Message.Text}
				if len(r.Locations) > 0 {
					pl := r.Locations[0].PhysicalLocation
					base := ""
					if b, ok := run.OriginalURIBaseIDs[pl.ArtifactLocation.URIBaseID]; ok {
						base = b.URI
					}
					d.File = uriPath(base, pl.ArtifactLocation.URI)
					d.Line, d.Column = pl.Region.StartLine, pl.Region.StartColumn
					d.EndLine, d.EndColumn = pl.Region.EndLine, pl.Region.EndColumn
				}
				if d.File == "" {
					d.Tool = run.Tool.Driver.Name
				}
				out = append(out, d)
			}
		}
	}
	return out
}

func sarifLevel(level string) string {
	switch level {
	case "warning", "note":
		return level
	case "none":
		return "help"
	}
	return "error"
}

// uriPath turns a SARIF artifact URI into a file path: a relative URI joins its base, and a
// file: URI becomes a local path. Clang on Windows writes "file:///C:/%2F/Users/…", which
// decodes to a doubled slash that Clean removes.
func uriPath(base, uri string) string {
	if uri == "" {
		return ""
	}
	if !strings.Contains(uri, ":") { // relative
		if base == "" {
			return filepath.FromSlash(uri)
		}
		uri = strings.TrimSuffix(base, "/") + "/" + uri
	}
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	p := u.Path
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' { // /C:/… on Windows
		p = p[1:]
	}
	return filepath.Clean(filepath.FromSlash(p))
}

// ParseGCCJSON reads GCC's -fdiagnostics-format=json: a JSON array of diagnostics per run.
func ParseGCCJSON(output string) []Diagnostic {
	type caret struct {
		File   string `json:"file"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	type diag struct {
		Kind      string `json:"kind"`
		Message   string `json:"message"`
		Locations []struct {
			Caret  caret `json:"caret"`
			Finish caret `json:"finish"`
		} `json:"locations"`
	}
	var out []Diagnostic
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[{") || !strings.Contains(line, `"kind"`) {
			continue
		}
		var list []diag
		if json.Unmarshal([]byte(line), &list) != nil {
			continue
		}
		for _, g := range list {
			d := Diagnostic{Severity: "error", Message: g.Message}
			switch g.Kind {
			case "warning", "note":
				d.Severity = g.Kind
			}
			if len(g.Locations) > 0 {
				c := g.Locations[0]
				d.File, d.Line, d.Column = c.Caret.File, c.Caret.Line, c.Caret.Column
				d.EndLine, d.EndColumn = c.Finish.Line, c.Finish.Column
			}
			out = append(out, d)
		}
	}
	return out
}
