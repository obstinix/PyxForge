package textfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func utf16le(s string, bom bool) []byte {
	var b []byte
	if bom {
		b = []byte{0xFF, 0xFE}
	}
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}

func TestDetect(t *testing.T) {
	binary := bytes.Repeat([]byte{0, 1, 2, 0xFF, 'A'}, 300)
	for _, tc := range []struct {
		name   string
		data   []byte
		binary bool
		bom    Encoding
	}{
		{"empty", nil, false, ""},
		{"UTF-8 with LF", []byte("print('héllo')\n"), false, ""},
		{"CRLF", []byte("line one\r\nline two\r\n"), false, ""},
		{"no final newline", []byte("last line"), false, ""},
		{"UTF-8 with BOM", append([]byte{0xEF, 0xBB, 0xBF}, "café\n"...), false, "UTF-8"},
		{"UTF-16LE with BOM", utf16le("UTF-16 LE: नमस्ते\r\n", true), false, "UTF-16LE"},
		{"UTF-16BE with BOM", []byte{0xFE, 0xFF, 0, 'h', 0, 'i'}, false, "UTF-16BE"},
		{"UTF-32LE with BOM", []byte{0xFF, 0xFE, 0, 0, 'h', 0, 0, 0}, false, "UTF-32LE"},
		{"Latin-1", []byte("caf\xe9 cr\xe8me\n"), false, ""},
		{"control characters but no NUL", []byte("\x1b[31mred\x1b[0m\x07\n"), false, ""},
		{"binary", binary, true, ""},
		{"UTF-16 without a BOM", utf16le("plain", false), true, ""},
		{"boot sector", append(bytes.Repeat([]byte{0x90}, 510), 0x55, 0xAA), false, ""},
		{"boot sector with zeros", append(append([]byte{0xEB, 0x3C}, make([]byte, 508)...), 0x55, 0xAA), true, ""},
		{"NUL after the sniffed bytes", append(bytes.Repeat([]byte("a"), Sniff), 0), false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "f")
			if err := os.WriteFile(p, tc.data, 0o644); err != nil {
				t.Fatal(err)
			}
			info, err := Detect(p)
			if err != nil {
				t.Fatal(err)
			}
			if info.Binary != tc.binary || info.BOM != tc.bom || info.Size != int64(len(tc.data)) {
				t.Errorf("Detect = binary %v, BOM %q, size %d; want %v, %q, %d", info.Binary, info.BOM, info.Size, tc.binary, tc.bom, len(tc.data))
			}
			if want := tc.data[:min(len(tc.data), Sniff)]; !bytes.Equal(info.Head, want) {
				t.Errorf("Head holds %d bytes, want the first %d", len(info.Head), len(want))
			}
		})
	}
}

func TestDetectFailures(t *testing.T) {
	if _, err := Detect(t.TempDir()); err == nil {
		t.Error("a folder was detected as a file")
	}
	if _, err := Detect(filepath.Join(t.TempDir(), "missing")); !os.IsNotExist(err) {
		t.Errorf("missing file: %v", err)
	}
}
