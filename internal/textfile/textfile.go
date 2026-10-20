// Package textfile tells text files from binary ones before PyxForge opens them in the editor.
//
// A file is binary when its first 8000 bytes hold a NUL byte and it does not start with a
// Unicode byte-order mark: UTF-16 and UTF-32 text is full of NUL bytes, while text in any
// single- or multi-byte encoding without a BOM (UTF-8, Latin-1, Windows-1252, Shift-JIS) has
// none. It is Git's rule, and PyxForge's Neovim configuration (nvim/lua/pyxforge/files.lua)
// applies the same one, so a binary file opened any other way is still read byte for byte and
// read-only.
package textfile

import (
	"bytes"
	"errors"
	"io"
	"os"
)

// Sniff is how many bytes from the start of a file decide.
const Sniff = 8000

// Encoding names a byte-order mark: "UTF-8", "UTF-16LE", "UTF-16BE", "UTF-32LE", "UTF-32BE",
// or "" for none.
type Encoding string

// boms in the order they must be tested: UTF-32LE's mark begins with UTF-16LE's.
var boms = []struct {
	mark []byte
	enc  Encoding
}{
	{[]byte{0xFF, 0xFE, 0x00, 0x00}, "UTF-32LE"},
	{[]byte{0x00, 0x00, 0xFE, 0xFF}, "UTF-32BE"},
	{[]byte{0xEF, 0xBB, 0xBF}, "UTF-8"},
	{[]byte{0xFF, 0xFE}, "UTF-16LE"},
	{[]byte{0xFE, 0xFF}, "UTF-16BE"},
}

// BOM returns the byte-order mark data starts with, if any.
func BOM(data []byte) Encoding {
	for _, b := range boms {
		if bytes.HasPrefix(data, b.mark) {
			return b.enc
		}
	}
	return ""
}

// IsBinary reports whether data, the start of a file, is binary.
func IsBinary(data []byte) bool {
	if len(data) > Sniff {
		data = data[:Sniff]
	}
	return BOM(data) == "" && bytes.IndexByte(data, 0) >= 0
}

// Info is what PyxForge knows about a file before opening it.
type Info struct {
	Binary bool
	BOM    Encoding
	Size   int64
	Head   []byte // the first bytes, at most Sniff
}

// Detect reads the start of the file at path. It fails for a folder or a file it cannot read.
func Detect(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{}, err
	}
	if st.IsDir() {
		return Info{}, errors.New(path + " is a folder")
	}
	head := make([]byte, Sniff)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return Info{}, err
	}
	head = head[:n]
	return Info{Binary: IsBinary(head), BOM: BOM(head), Size: st.Size(), Head: head}, nil
}
