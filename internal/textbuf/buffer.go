// Package textbuf is the document model of PyxForge's native editor: the text of one file as
// lines of runes, edits that report what changed (for syntax highlighting and language
// servers), grouped undo and redo, and loading and saving that keep the file's line endings
// and encoding (UTF-8, with or without a byte-order mark, or UTF-16 with one). It knows nothing about drawing or input (ARCHITECTURE_DECISIONS.md, D16).
package textbuf

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Pos is a place in the text: a line and a column, both from 0. Col counts runes.
type Pos struct{ Line, Col int }

// Less reports whether p comes before q.
func (p Pos) Less(q Pos) bool { return p.Line < q.Line || p.Line == q.Line && p.Col < q.Col }

// Range is the text from Start up to, not including, End. Start is never after End.
type Range struct{ Start, End Pos }

// Ordered returns the range between two positions given in either order.
func Ordered(a, b Pos) Range {
	if b.Less(a) {
		a, b = b, a
	}
	return Range{a, b}
}

// Empty reports whether the range holds no text.
func (r Range) Empty() bool { return r.Start == r.End }

// Change is one edit as a language server wants it: the range it replaced, in positions from
// before the edit, and the text put there.
type Change struct {
	Range Range
	Text  string
}

// edit is one change as undo needs it.
type edit struct {
	at       Pos
	removed  string
	inserted string
	group    int
}

// Buffer is the text of one document. It is not safe for use from several goroutines.
type Buffer struct {
	lines   [][]rune
	crlf    bool // the file used CRLF line endings
	bom     bool // the file started with a UTF-8 byte-order mark
	enc     encoding
	undo    []edit
	redo    []edit
	group   int
	version int // counts edits, undo and redo included
	saved   int // the undo history's length at the last load or save; -1 once unreachable

	// OnChange, when set, is called after every edit, undo and redo included.
	OnChange func(Change)
	// ReadOnly refuses every edit, undo and save: a binary file shown as text (LoadBinary).
	ReadOnly bool
}

// encoding is how a file's text is stored.
type encoding uint8

const (
	utf8Text encoding = iota
	utf16LE           // with the byte-order mark FF FE
	utf16BE           // with FE FF
)

// ErrReadOnly is returned by Save for a read-only buffer.
var ErrReadOnly = errors.New("the document is read-only")

// New returns a buffer holding text.
func New(text string) *Buffer {
	b := &Buffer{}
	b.setText(text)
	return b
}

func (b *Buffer) setText(text string) {
	if strings.Contains(text, "\r\n") {
		b.crlf = true
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	parts := strings.Split(text, "\n")
	b.lines = make([][]rune, len(parts))
	for i, p := range parts {
		b.lines[i] = []rune(p)
	}
}

// ErrNotText is returned for a file the editor cannot edit as text.
var ErrNotText = errors.New("not UTF-8 or UTF-16 text")

// Load reads a file: UTF-8, or UTF-16 that starts with its byte-order mark. Anything else (a
// binary file, a broken encoding) is refused with ErrNotText, so saving can never rewrite
// bytes the editor could not show.
func Load(path string) (*Buffer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, e := range []encoding{utf16LE, utf16BE} {
		if bytes.HasPrefix(data, e.bom()) {
			text, ok := decodeUTF16(data[2:], e)
			if !ok {
				return nil, fmt.Errorf("%s: %w", filepath.Base(path), ErrNotText)
			}
			b := New(text)
			b.enc = e
			return b, nil
		}
	}
	bom := bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if bom {
		data = data[3:]
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), ErrNotText)
	}
	b := New(string(data))
	b.bom = bom
	return b, nil
}

// LoadBinary reads any file as read-only text, each byte one character (Latin-1), for looking
// at a binary file's strings.
func LoadBinary(path string) (*Buffer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r := make([]rune, len(data))
	for i, c := range data {
		r[i] = rune(c)
	}
	b := &Buffer{ReadOnly: true}
	parts := strings.Split(string(r), "\n")
	b.lines = make([][]rune, len(parts))
	for i, p := range parts {
		b.lines[i] = []rune(p)
	}
	return b, nil
}

func (e encoding) bom() []byte {
	switch e {
	case utf16LE:
		return []byte{0xFF, 0xFE}
	case utf16BE:
		return []byte{0xFE, 0xFF}
	}
	return nil
}

func decodeUTF16(data []byte, e encoding) (string, bool) {
	if len(data)%2 != 0 {
		return "", false
	}
	u := make([]uint16, len(data)/2)
	for i := range u {
		if e == utf16LE {
			u[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
		} else {
			u[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
		}
	}
	r := utf16.Decode(u)
	for _, c := range r {
		if c == utf8.RuneError || c == 0 {
			return "", false // a lone surrogate, or a NUL that only a binary file has
		}
	}
	return string(r), true
}

func encodeUTF16(text string, e encoding) []byte {
	u := utf16.Encode([]rune(text))
	out := make([]byte, 0, 2+2*len(u))
	out = append(out, e.bom()...)
	for _, c := range u {
		if e == utf16LE {
			out = append(out, byte(c), byte(c>>8))
		} else {
			out = append(out, byte(c>>8), byte(c))
		}
	}
	return out
}

// Save writes the text to path through a temporary file in the same folder, renamed over the
// old one, so a failure part way leaves the old file whole. Line endings and the byte-order
// mark and the encoding are the ones the file was loaded with.
func (b *Buffer) Save(path string) error {
	if b.ReadOnly {
		return ErrReadOnly
	}
	var out bytes.Buffer
	if b.bom {
		out.Write([]byte{0xEF, 0xBB, 0xBF})
	}
	nl := "\n"
	if b.crlf {
		nl = "\r\n"
	}
	for i, l := range b.lines {
		if i > 0 {
			out.WriteString(nl)
		}
		out.WriteString(string(l))
	}
	data := out.Bytes()
	if b.enc != utf8Text {
		data = encodeUTF16(out.String(), b.enc)
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".pyxforge-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	b.saved = len(b.undo)
	return nil
}

// MarkSaved records the text as what is on disk, as after a reload.
func (b *Buffer) MarkSaved() { b.saved = len(b.undo) }

// Modified reports whether the text differs from the last load or save. Undoing back to the
// saved text makes it unmodified again.
func (b *Buffer) Modified() bool { return len(b.undo) != b.saved }

// Version counts edits; it changes on every edit, undo and redo.
func (b *Buffer) Version() int { return b.version }

// LineCount is the number of lines; an empty buffer has one.
func (b *Buffer) LineCount() int { return len(b.lines) }

// Line returns one line without its line ending. The slice must not be changed.
func (b *Buffer) Line(i int) []rune {
	if i < 0 || i >= len(b.lines) {
		return nil
	}
	return b.lines[i]
}

// Text returns the whole text with "\n" line endings.
func (b *Buffer) Text() string {
	var s strings.Builder
	for i, l := range b.lines {
		if i > 0 {
			s.WriteByte('\n')
		}
		s.WriteString(string(l))
	}
	return s.String()
}

// End is the position after the last character.
func (b *Buffer) End() Pos {
	n := len(b.lines) - 1
	return Pos{n, len(b.lines[n])}
}

// Clamp moves p to the nearest position inside the text.
func (b *Buffer) Clamp(p Pos) Pos {
	if p.Line < 0 {
		return Pos{}
	}
	if p.Line >= len(b.lines) {
		return b.End()
	}
	return Pos{p.Line, min(max(p.Col, 0), len(b.lines[p.Line]))}
}

// Slice returns the text in r.
func (b *Buffer) Slice(r Range) string {
	r = Range{b.Clamp(r.Start), b.Clamp(r.End)}
	if r.Start.Line == r.End.Line {
		return string(b.lines[r.Start.Line][r.Start.Col:r.End.Col])
	}
	var s strings.Builder
	s.WriteString(string(b.lines[r.Start.Line][r.Start.Col:]))
	for l := r.Start.Line + 1; l < r.End.Line; l++ {
		s.WriteByte('\n')
		s.WriteString(string(b.lines[l]))
	}
	s.WriteByte('\n')
	s.WriteString(string(b.lines[r.End.Line][:r.End.Col]))
	return s.String()
}

// Seal ends the current undo group: the next edit starts a new one. The editor seals when the
// cursor moves other than by typing, so one undo takes back one run of typing.
func (b *Buffer) Seal() { b.group++ }

// Replace puts text in place of r and returns the position after the inserted text.
func (b *Buffer) Replace(r Range, text string) Pos {
	if b.ReadOnly {
		return b.Clamp(r.Start)
	}
	r = Range{b.Clamp(r.Start), b.Clamp(r.End)}
	if r.End.Less(r.Start) {
		r.Start, r.End = r.End, r.Start
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	removed := b.Slice(r)
	if removed == "" && text == "" {
		return r.Start
	}
	if len(b.undo) < b.saved {
		b.saved = -1 // the saved text was undone past and is now beyond reach
	}
	end := b.apply(r, text)
	b.undo = append(b.undo, edit{at: r.Start, removed: removed, inserted: text, group: b.group})
	b.redo = nil
	return end
}

// Insert puts text at p and returns the position after it.
func (b *Buffer) Insert(p Pos, text string) Pos { return b.Replace(Range{p, p}, text) }

// Delete removes the text in r.
func (b *Buffer) Delete(r Range) { b.Replace(r, "") }

// apply changes the lines and reports the change; it does not touch the undo history.
func (b *Buffer) apply(r Range, text string) Pos {
	head := b.lines[r.Start.Line][:r.Start.Col]
	tail := b.lines[r.End.Line][r.End.Col:]
	parts := strings.Split(text, "\n")
	repl := make([][]rune, len(parts))
	for i, p := range parts {
		repl[i] = []rune(p)
	}
	end := Pos{r.Start.Line + len(repl) - 1, len(repl[len(repl)-1])}
	if len(repl) == 1 {
		end.Col += len(head)
	}
	first := append(append([]rune{}, head...), repl[0]...)
	repl[0] = first
	last := len(repl) - 1
	repl[last] = append(append([]rune{}, repl[last]...), tail...)
	lines := make([][]rune, 0, len(b.lines)-(r.End.Line-r.Start.Line)+last)
	lines = append(lines, b.lines[:r.Start.Line]...)
	lines = append(lines, repl...)
	lines = append(lines, b.lines[r.End.Line+1:]...)
	b.lines = lines
	b.version++
	if b.OnChange != nil {
		b.OnChange(Change{Range: r, Text: text})
	}
	return end
}

// endOf is where text inserted at p ends.
func endOf(p Pos, text string) Pos {
	n := strings.Count(text, "\n")
	if n == 0 {
		return Pos{p.Line, p.Col + utf8.RuneCountInString(text)}
	}
	return Pos{p.Line + n, utf8.RuneCountInString(text[strings.LastIndexByte(text, '\n')+1:])}
}

// Undo takes back the last group of edits and returns where the cursor belongs, or false
// when there is nothing to undo.
func (b *Buffer) Undo() (Pos, bool) {
	if len(b.undo) == 0 || b.ReadOnly {
		return Pos{}, false
	}
	g := b.undo[len(b.undo)-1].group
	var at Pos
	for len(b.undo) > 0 && b.undo[len(b.undo)-1].group == g {
		e := b.undo[len(b.undo)-1]
		b.undo = b.undo[:len(b.undo)-1]
		at = b.apply(Range{e.at, endOf(e.at, e.inserted)}, e.removed)
		b.redo = append(b.redo, e)
	}
	b.group++
	return at, true
}

// Redo applies the last undone group again.
func (b *Buffer) Redo() (Pos, bool) {
	if len(b.redo) == 0 || b.ReadOnly {
		return Pos{}, false
	}
	g := b.redo[len(b.redo)-1].group
	var at Pos
	for len(b.redo) > 0 && b.redo[len(b.redo)-1].group == g {
		e := b.redo[len(b.redo)-1]
		b.redo = b.redo[:len(b.redo)-1]
		at = b.apply(Range{e.at, endOf(e.at, e.removed)}, e.inserted)
		b.undo = append(b.undo, e)
	}
	b.group++
	return at, true
}
