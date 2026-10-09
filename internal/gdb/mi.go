// Package gdb drives GDB through its machine interface (GDB/MI): attach to QEMU's GDB stub,
// read registers, memory, the stack and disassembly, set breakpoints and step.
package gdb

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Record is one line of GDB/MI output.
type Record struct {
	Token   int    // the command's token, or -1
	Kind    byte   // '^' result, '*' exec, '+' status, '=' notify, '~' console, '@' target, '&' log
	Class   string // done, running, connected, error, exit; stopped, running; breakpoint-created…
	Results map[string]any
	Text    string // for stream records: the decoded text
}

// Values in Results are string, map[string]any (a tuple) or []any (a list).

// ParseRecord parses one line of GDB/MI output. "(gdb)" prompts are not records.
func ParseRecord(line string) (Record, error) {
	line = strings.TrimRight(line, "\r\n")
	r := Record{Token: -1}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 {
		r.Token, _ = strconv.Atoi(line[:i])
	}
	if i >= len(line) {
		return r, errors.New("empty GDB/MI record")
	}
	r.Kind = line[i]
	rest := line[i+1:]
	switch r.Kind {
	case '~', '@', '&':
		p := &parser{s: rest}
		s, err := p.cstring()
		r.Text = s
		return r, err
	case '^', '*', '+', '=':
		class, results, _ := strings.Cut(rest, ",")
		r.Class = class
		r.Results = map[string]any{}
		if results == "" {
			return r, nil
		}
		p := &parser{s: results}
		for {
			name, v, err := p.result()
			if err != nil {
				return r, fmt.Errorf("GDB/MI %q: %w", line, err)
			}
			r.Results[name] = v
			if !p.eat(',') {
				break
			}
		}
		return r, nil
	}
	return r, fmt.Errorf("unknown GDB/MI record %q", line)
}

type parser struct {
	s string
	i int
}

func (p *parser) eat(c byte) bool {
	if p.i < len(p.s) && p.s[p.i] == c {
		p.i++
		return true
	}
	return false
}

func (p *parser) result() (string, any, error) {
	eq := strings.IndexByte(p.s[p.i:], '=')
	if eq < 0 {
		return "", nil, errors.New("expected name=value")
	}
	name := p.s[p.i : p.i+eq]
	p.i += eq + 1
	v, err := p.value()
	return name, v, err
}

func (p *parser) value() (any, error) {
	if p.i >= len(p.s) {
		return nil, errors.New("value expected")
	}
	switch p.s[p.i] {
	case '"':
		return p.cstring()
	case '{':
		p.i++
		t := map[string]any{}
		if p.eat('}') {
			return t, nil
		}
		for {
			name, v, err := p.result()
			if err != nil {
				return nil, err
			}
			t[name] = v
			if p.eat('}') {
				return t, nil
			}
			if !p.eat(',') {
				return nil, errors.New("expected , or }")
			}
		}
	case '[':
		p.i++
		l := []any{}
		if p.eat(']') {
			return l, nil
		}
		for {
			var v any
			var err error
			if c := p.s[p.i]; c == '"' || c == '{' || c == '[' {
				v, err = p.value()
			} else {
				_, v, err = p.result() // a list of results keeps the values: frame={…}
			}
			if err != nil {
				return nil, err
			}
			l = append(l, v)
			if p.eat(']') {
				return l, nil
			}
			if !p.eat(',') {
				return nil, errors.New("expected , or ]")
			}
		}
	}
	return nil, fmt.Errorf("unexpected %q", p.s[p.i])
}

// cstring reads a C string with GDB's escapes.
func (p *parser) cstring() (string, error) {
	if !p.eat('"') {
		return "", errors.New("string expected")
	}
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		p.i++
		switch c {
		case '"':
			return b.String(), nil
		case '\\':
			if p.i >= len(p.s) {
				return "", errors.New("unfinished escape")
			}
			e := p.s[p.i]
			p.i++
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'e':
				b.WriteByte(0x1b)
			case '0', '1', '2', '3', '4', '5', '6', '7':
				n := int(e - '0')
				for k := 0; k < 2 && p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '7'; k++ {
					n = n*8 + int(p.s[p.i]-'0')
					p.i++
				}
				b.WriteByte(byte(n))
			default:
				b.WriteByte(e)
			}
		default:
			b.WriteByte(c)
		}
	}
	return "", errors.New("unterminated string")
}

// Str reads a string at a path of tuple keys, such as Str(r.Results, "frame", "addr").
func Str(v any, path ...string) string {
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = m[k]
	}
	s, _ := v.(string)
	return s
}

// List reads a list at a path of tuple keys.
func List(v any, path ...string) []any {
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	l, _ := v.([]any)
	return l
}
