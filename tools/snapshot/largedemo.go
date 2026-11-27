package main

import (
	"os"
	"path/filepath"
	"strings"
)

// largeDemo is a folder with the Phase 7 review files: a binary disk image (a boot sector and
// zeros) and a JSON file on one line of about 200 KB.
func largeDemo() string {
	dir, err := os.MkdirTemp("", "pyxforge-large-")
	if err != nil {
		fail(err)
	}
	img := make([]byte, 64<<10)
	copy(img, []byte("\xeb\x3c\x90PYXFORGE\x00\x02\x01\x01\x00\x02\xe0\x00\x40\x0b\xf0\x09\x00\x12\x00\x02\x00"))
	copy(img[0x3e:], []byte("\xfa\x31\xc0\x8e\xd8\x8e\xc0\x8e\xd0\xbc\x00\x7c\xfb\xbe\x5d\x7cBooting PyxForge demo...\r\n\x00"))
	img[510], img[511] = 0x55, 0xaa
	if err := os.WriteFile(filepath.Join(dir, "disk.img"), img, 0o644); err != nil {
		fail(err)
	}
	line := "[" + strings.Repeat(`{"id":1,"name":"sector","lba":2048},`, 6000) + "{}]\n"
	if err := os.WriteFile(filepath.Join(dir, "one-line.json"), []byte(line), 0o644); err != nil {
		fail(err)
	}
	return dir
}
