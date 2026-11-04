package build

import "testing"

var severities = map[string]bool{"error": true, "warning": true, "note": true, "help": true}

// FuzzDiagnostics feeds compiler output to every diagnostics parser: a build prints whatever
// its tools print (#43).
func FuzzDiagnostics(f *testing.F) {
	for _, s := range []string{
		"boot.asm:10: error: instruction expected",
		"src/main.c:12:5: warning: implicit declaration of function 'foo'",
		`C:\src\os\boot.asm:3: error: symbol 'x' not defined`,
		"In file included from kernel.c:1:\r\nvga.h:4:1: error: unknown type name 'u8'\r\n    4 | u8 *vga;\r\n      | ^~\r\n",
		`kernel.c(12,5): error C2065: 'x': undeclared identifier`,
		`LINK : fatal error LNK1104: cannot open file 'kernel.exe'`,
		"error[E0425]: cannot find value `x` in this scope\n --> src/main.rs:10:15\n",
		"ld: cannot find -lfoo",
		"kernel.o:(.text+0x1c): undefined reference to `kmain'",
		`{"reason":"compiler-message","message":{"message":"cannot find value","code":{"code":"E0425"},"level":"error","spans":[{"file_name":"src/main.rs","byte_start":100,"byte_end":101,"line_start":10,"line_end":10,"column_start":15,"column_end":16,"is_primary":true}]}}`,
		gccSARIF,
		gccJSON,
		`[{"kind": "error", "locations": [{"caret": {"line": -3}}]}]`,
		`{"version": "2.1.0", "runs": [{"results": [{"level": "note", "message": {"text": "x"}, "locations": [{"physicalLocation": {"artifactLocation": {"uri": "file:///%zz"}}}]}]}]}`,
	} {
		f.Add(s)
	}
	parsers := map[string]func(string) []Diagnostic{
		"ParseDiagnostics": ParseDiagnostics, "ParseText": ParseText, "ParseCargoJSON": ParseCargoJSON,
		"ParseSARIF": ParseSARIF, "ParseGCCJSON": ParseGCCJSON,
	}
	f.Fuzz(func(t *testing.T, output string) {
		for name, parse := range parsers {
			for _, d := range parse(output) {
				if !severities[d.Severity] {
					t.Fatalf("%s: severity %q in %+v", name, d.Severity, d)
				}
				if d.Message == "" {
					t.Fatalf("%s: a diagnostic without a message: %+v", name, d)
				}
				if d.Line < 0 || d.Column < 0 || d.EndLine < 0 || d.EndColumn < 0 {
					t.Fatalf("%s: a negative position: %+v", name, d)
				}
			}
		}
	})
}
