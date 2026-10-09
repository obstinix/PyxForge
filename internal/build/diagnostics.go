package build

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Diagnostic is one problem a build tool reported. Line and Column are 1-based as tools print
// them; 0 means absent.
type Diagnostic struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	Severity  string `json:"severity"` // "error", "warning", "note" or "help"
	Message   string `json:"message"`
}

// ParseDiagnostics reads a build's output. Cargo's JSON messages are authoritative when there
// are any; otherwise lines in the GNU form (NASM, GCC, Clang, ld) are used. Lines that are
// neither are skipped, so parsing never fails (2.x parity: diagnostics.rs).
func ParseDiagnostics(output string) []Diagnostic {
	if d := ParseCargoJSON(output); len(d) > 0 {
		return d
	}
	return ParseGNU(output)
}

// ParseGNU reads `file:line[:col]: severity: message` lines.
func ParseGNU(output string) []Diagnostic {
	var out []Diagnostic
	for line := range strings.SplitSeq(output, "\n") {
		if d, ok := parseGNULine(line); ok {
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
