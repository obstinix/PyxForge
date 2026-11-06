package build

import (
	"testing"

	"golang.org/x/sys/windows"
)

// TestToolTextReadsTheOEMCodePage: a line in the console's code page, as cl prints a folder
// named "src dir ü" (#71), becomes UTF-8; a line that is UTF-8 already is left alone.
func TestToolTextReadsTheOEMCodePage(t *testing.T) {
	oem, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetOEMCP").Call()
	if oem != 437 && oem != 850 && oem != 858 { // where "ü" is 0x81, as on GitHub's runners
		t.Skipf("the OEM code page is %d, which has no ü at 0x81", oem)
	}
	got := toolText("P:\\src dir \x81\\t.c(5,12): error C2065: 'y': undeclared identifier\r\n")
	if want := "P:\\src dir ü\\t.c(5,12): error C2065: 'y': undeclared identifier\r\n"; got != want {
		t.Errorf("OEM: got %q, want %q", got, want)
	}
	for _, s := range []string{"", "P:\\src dir ü\\t.c(5): error C2065: 'y'\r\n", "plain ASCII\n"} {
		if got := toolText(s); got != s {
			t.Errorf("UTF-8 changed: %q became %q", s, got)
		}
	}
}
