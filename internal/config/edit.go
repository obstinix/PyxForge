package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Editing pyxforge.toml in place (#69). Set Up Machine changes a few keys of [qemu],
// [qemu.debug] and [gdb] in a file the user wrote; these functions touch only the lines of
// those keys, so comments, layout and every other table stay as they were. They edit the
// plain form PyxForge writes and people usually write, a table header with one key per line;
// a key written another way (dotted, or in an inline table) is reported, not rewritten.

var (
	headerLine = regexp.MustCompile(`^\s*\[\s*([A-Za-z0-9_.-]+)\s*\]\s*(#.*)?$`)
	anyHeader  = regexp.MustCompile(`^\s*\[`)
	keyLine    = regexp.MustCompile(`^(\s*)([A-Za-z0-9_-]+)\s*=\s*(.*)$`)
)

// ErrNotEditable means a key is written in a form these functions do not rewrite.
var ErrNotEditable = errors.New("written in a form PyxForge does not edit; change it in the file")

// TOMLString is s as a TOML basic string.
func TOMLString(s string) (string, error) {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("%q holds a control character", s)
		}
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`, nil
}

// section finds table's lines: the header's index and the index after its last line, or
// header -1 when the file has no such table.
func section(lines []string, table string) (header, end int) {
	header, end = -1, len(lines)
	for i, l := range lines {
		if header < 0 {
			if m := headerLine.FindStringSubmatch(l); m != nil && m[1] == table {
				header = i
			}
			continue
		}
		if anyHeader.MatchString(l) {
			return header, i
		}
	}
	return header, end
}

// trailingComment returns the comment after a value, with the spaces before it, or "".
func trailingComment(value string) string {
	inString, quote := false, byte(0)
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case inString && c == '\\' && quote == '"':
			i++
		case inString && c == quote:
			inString = false
		case !inString && (c == '"' || c == '\''):
			inString, quote = true, c
		case !inString && c == '#':
			j := i
			for j > 0 && (value[j-1] == ' ' || value[j-1] == '\t') {
				j--
			}
			return value[j:]
		}
	}
	return ""
}

// SetValue sets key in table to value (TOML, already formatted: TOMLString for strings). An
// existing line keeps its indentation and comment; a new key goes after the table's last key;
// a missing table is added at the end of the file.
func SetValue(text, table, key, value string) (string, error) {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	header, end := section(lines, table)
	if header < 0 {
		if dottedOrInline(lines, table, key) {
			return "", fmt.Errorf("%s.%s is %w", table, key, ErrNotEditable)
		}
		body := strings.TrimRight(strings.Join(lines, "\n"), "\n")
		if body != "" {
			body += "\n\n"
		}
		body += "[" + table + "]\n" + key + " = " + value + "\n"
		return strings.ReplaceAll(body, "\n", nl), nil
	}
	last := header
	for i := header + 1; i < end; i++ {
		m := keyLine.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		last = i
		if m[2] != key {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(m[3]), "{") || strings.HasPrefix(strings.TrimSpace(m[3]), "[") {
			return "", fmt.Errorf("%s.%s is %w", table, key, ErrNotEditable)
		}
		lines[i] = m[1] + key + " = " + value + trailingComment(m[3])
		return strings.Join(lines, nl), nil
	}
	lines = append(lines[:last+1], append([]string{key + " = " + value}, lines[last+1:]...)...)
	return strings.Join(lines, nl), nil
}

// RemoveKey deletes key's line from table, if it is there.
func RemoveKey(text, table, key string) string {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	header, end := section(lines, table)
	if header < 0 {
		return text
	}
	for i := header + 1; i < end; i++ {
		if m := keyLine.FindStringSubmatch(lines[i]); m != nil && m[2] == key {
			return strings.Join(append(lines[:i], lines[i+1:]...), nl)
		}
	}
	return text
}

// dottedOrInline reports whether table's key could be written somewhere other than under its
// own header: as a dotted key, or inside an inline table.
func dottedOrInline(lines []string, table, key string) bool {
	parts := strings.Split(table, ".")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, table+".") || strings.HasPrefix(t, table+" =") || strings.HasPrefix(t, table+"=") {
			return true
		}
		if len(parts) > 1 && strings.HasPrefix(t, parts[len(parts)-1]+" =") && strings.Contains(t, "{") {
			return true
		}
	}
	return false
}
