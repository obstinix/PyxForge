//go:build windows

package build

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

// cpOEM is CP_OEMCP, the system's OEM code page: the one a new console starts with.
const cpOEM = 1

// toolText makes a line of a tool's output UTF-8. Console programs that don't write UTF-8 print
// in their console's code page, and build tools run in a console of their own
// (proc.NoWindow), which starts with the system's OEM code page. Microsoft's cl, link and ml
// do this: a folder named "src dir ü" comes back as "src dir \x81" under code page 437, a path
// that doesn't exist (#71). A line that is already UTF-8 (GCC, Clang, rustc, NASM, or any tool
// where the OEM code page is UTF-8) is left as it is.
func toolText(line string) string {
	if line == "" || utf8.ValidString(line) {
		return line
	}
	b := []byte(line)
	n, err := windows.MultiByteToWideChar(cpOEM, 0, &b[0], int32(len(b)), nil, 0)
	if err != nil || n <= 0 {
		return strings.ToValidUTF8(line, "�")
	}
	w := make([]uint16, n)
	if n, err = windows.MultiByteToWideChar(cpOEM, 0, &b[0], int32(len(b)), &w[0], n); err != nil || n <= 0 {
		return strings.ToValidUTF8(line, "�")
	}
	return string(utf16.Decode(w[:n]))
}
