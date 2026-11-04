package config

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
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

// continuation reports, for each line, whether it starts inside a value that began on an
// earlier line: a multi-line string (between three double or three single quotes) or a
// multi-line array. Such a line is string text or array items, never a key or a table header,
// whatever it looks like. The extra last element says whether the file ends inside such a
// value.
func continuation(lines []string) []bool {
	cont := make([]bool, len(lines)+1)
	open := "" // the multi-line string delimiter we are inside, or ""
	depth := 0 // unclosed '[' of arrays
	for i, l := range lines {
		cont[i] = open != "" || depth > 0
		header := open == "" && depth == 0 && anyHeader.MatchString(l)
		for j := 0; j < len(l); {
			if open != "" {
				switch {
				case open == `"""` && l[j] == '\\':
					j += 2
				case strings.HasPrefix(l[j:], open):
					j += 3
					for k := 0; k < 2 && j < len(l) && l[j] == open[0]; k++ {
						j++ // up to two quotes before the delimiter belong to the string
					}
					open = ""
				default:
					j++
				}
				continue
			}
			c := l[j]
			switch {
			case c == '#':
				j = len(l)
			case strings.HasPrefix(l[j:], `"""`) || strings.HasPrefix(l[j:], `'''`):
				open = l[j : j+3]
				j += 3
			case c == '"':
				for j++; j < len(l) && l[j] != '"'; j++ {
					if l[j] == '\\' {
						j++
					}
				}
				j++
			case c == '\'':
				for j++; j < len(l) && l[j] != '\''; j++ {
				}
				j++
			case c == '[' && !header:
				depth++
				j++
			case c == ']' && !header && depth > 0:
				depth--
				j++
			default:
				j++
			}
		}
	}
	cont[len(lines)] = open != "" || depth > 0
	return cont
}

// section finds table's lines: the header's index and the index after its last line, or
// header -1 when the file has no such table.
func section(lines []string, cont []bool, table string) (header, end int) {
	header, end = -1, len(lines)
	for i, l := range lines {
		if cont[i] {
			continue
		}
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
// a missing table is added at the end of the file. The result is decoded again and must hold
// the value; when it wouldn't (the key is written in a form these functions don't rewrite, or
// the file isn't TOML), the error is ErrNotEditable and nothing changes.
func SetValue(text, table, key, value string) (string, error) {
	out, err := setValue(text, table, key, value)
	if err != nil {
		return "", err
	}
	if !holds(out, table, key, value) {
		return "", fmt.Errorf("%s.%s is %w", table, key, ErrNotEditable)
	}
	return out, nil
}

func setValue(text, table, key, value string) (string, error) {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	cont := continuation(lines)
	header, end := section(lines, cont, table)
	if header < 0 {
		if dottedOrInline(lines, cont, table) {
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
		if cont[i] {
			continue
		}
		m := keyLine.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		last = valueEnd(cont, i, end)
		if m[2] != key {
			continue
		}
		v := strings.TrimSpace(m[3])
		if strings.HasPrefix(v, "{") || strings.HasPrefix(v, "[") || last != i {
			return "", fmt.Errorf("%s.%s is %w", table, key, ErrNotEditable)
		}
		lines[i] = m[1] + key + " = " + value + trailingComment(m[3])
		return strings.Join(lines, nl), nil
	}
	lines = append(lines[:last+1], append([]string{key + " = " + value}, lines[last+1:]...)...)
	return strings.Join(lines, nl), nil
}

// valueEnd is the last line of the value whose key is on line i: i itself, or the line that
// closes its multi-line string or array.
func valueEnd(cont []bool, i, end int) int {
	for i+1 < end && cont[i+1] {
		i++
	}
	return i
}

// RemoveKey deletes key and its value from table, if it is there. When the result would not be
// TOML without the key, the text is returned as it was.
func RemoveKey(text, table, key string) string {
	nl := "\n"
	if strings.Contains(text, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	cont := continuation(lines)
	header, end := section(lines, cont, table)
	if header < 0 {
		return text
	}
	for i := header + 1; i < end; i++ {
		if cont[i] {
			continue
		}
		if m := keyLine.FindStringSubmatch(lines[i]); m != nil && m[2] == key {
			last := valueEnd(cont, i, end)
			out := strings.Join(append(lines[:i:i], lines[last+1:]...), nl)
			var doc map[string]any
			if _, err := toml.Decode(out, &doc); err != nil || lookupKey(doc, table, key) != nil {
				return text
			}
			return out
		}
	}
	return text
}

// holds reports whether text decodes as TOML with table's key equal to value (TOML).
func holds(text, table, key, value string) bool {
	var want, doc map[string]any
	if _, err := toml.Decode("v = "+value, &want); err != nil {
		return false
	}
	if _, err := toml.Decode(text, &doc); err != nil {
		return false
	}
	return reflect.DeepEqual(lookupKey(doc, table, key), want["v"])
}

// lookupKey returns table's key from a decoded document; table may be dotted ("qemu.debug").
func lookupKey(doc map[string]any, table, key string) any {
	m := doc
	for _, part := range strings.Split(table, ".") {
		next, ok := m[part].(map[string]any)
		if !ok {
			return nil
		}
		m = next
	}
	return m[key]
}

// dottedOrInline reports whether table could be written somewhere other than under its own
// header: as dotted keys, or as an inline table.
func dottedOrInline(lines []string, cont []bool, table string) bool {
	parts := strings.Split(table, ".")
	for i, l := range lines {
		if cont[i] {
			continue
		}
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
