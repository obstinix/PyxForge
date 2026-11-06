package build

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestParseRealMSVCOutput reads output that Microsoft's own tools printed (#71): cl 19.44,
// link 14.44, ml64 and ml 14.44 (Visual Studio Build Tools 2022), captured byte for byte from
// failing builds in testdata/msvc (CRLF, UTF-8 file names). The sources were in
// "P:\src dir ü", a substituted drive, so no machine's own folder is in them. Besides the
// messages, the output holds lines a parser must pass over: cl's name of each source file,
// the source line and caret of /diagnostics:caret, and MASM's " Assembling: file".
func TestParseRealMSVCOutput(t *testing.T) {
	const dir = `P:\src dir ü\`
	cases := []struct {
		fixture string
		want    []Diagnostic
	}{
		{"cl-classic", []Diagnostic{
			{File: "t.c", Line: 5, Severity: "error", Message: "C2065: 'y': undeclared identifier"},
			{File: "t.c", Line: 4, Severity: "warning", Message: "C4047: 'initializing': 'int' differs in levels of indirection from 'char [2]'"},
		}},
		{"cl-column-fullpath", []Diagnostic{
			{File: dir + "t.c", Line: 5, Column: 12, Severity: "error", Message: "C2065: 'y': undeclared identifier"},
			{File: dir + "t.c", Line: 4, Column: 9, Severity: "warning", Message: "C4047: 'initializing': 'int' differs in levels of indirection from 'char [2]'"},
		}},
		{"cl-caret", []Diagnostic{
			{File: "conv.cpp", Line: 2, Column: 11, Severity: "error", Message: "C2440: 'initializing': cannot convert from 'const char [2]' to 'int'"},
			{File: "conv.cpp", Line: 2, Column: 11, Severity: "note", Message: "There is no context in which this conversion is possible"},
		}},
		{"cl-error-note", []Diagnostic{
			{File: "conv.cpp", Line: 2, Severity: "error", Message: "C2440: 'initializing': cannot convert from 'const char [2]' to 'int'"},
			{File: "conv.cpp", Line: 2, Severity: "note", Message: "There is no context in which this conversion is possible"},
		}},
		{"cl-fatal-include", []Diagnostic{
			{File: "inc.c", Line: 1, Severity: "error", Message: "C1083: Cannot open include file: 'nosuch.h': No such file or directory"},
		}},
		{"cl-command-line", []Diagnostic{
			{Tool: "cl", Severity: "warning", Message: "D9002: ignoring unknown option '/foo'"},
		}},
		{"link-unresolved", []Diagnostic{
			{Tool: "LINK", Severity: "error", Message: "LNK2019: link.obj: unresolved external symbol f referenced in function main"},
			{Tool: "LINK", Severity: "error", Message: "LNK1120: prog.exe: 1 unresolved externals"},
		}},
		{"link-missing-input", []Diagnostic{
			{Tool: "LINK", Severity: "error", Message: "LNK1181: cannot open input file 'nosuch.obj'"},
		}},
		{"link-missing-lib", []Diagnostic{
			{Tool: "LINK", Severity: "error", Message: "LNK1181: cannot open input file 'nosuchlib.lib'"},
		}},
		{"ml64", []Diagnostic{
			{File: "boot64.asm", Line: 4, Severity: "error", Message: "A2008: syntax error : rbx"},
			{File: "boot64.asm", Line: 3, Severity: "error", Message: "A2006: undefined symbol : undefined_sym"},
		}},
		{"ml", []Diagnostic{
			{File: "boot32.asm", Line: 6, Severity: "error", Message: "A2008: syntax error : ebx"},
			{File: "boot32.asm", Line: 5, Severity: "error", Message: "A2006: undefined symbol : undefined_sym"},
			{File: "boot32.asm", Line: 8, Severity: "warning", Message: "A4023: with /coff switch, leading underscore required for start address : start"},
		}},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "msvc", c.fixture+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(data, []byte("\r\n")) {
				t.Fatal("the fixture lost its CRLF line ends; see .gitattributes")
			}
			if got := ParseDiagnostics(string(data)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("\n got %+v\nwant %+v", got, c.want)
			}
		})
	}
}
