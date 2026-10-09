package build

import (
	"reflect"
	"testing"
)

// The cases port legacy/core/src/diagnostics.rs's tests one for one, then add Windows paths
// and GCC's context lines.

func TestParseGNU(t *testing.T) {
	cases := []struct {
		name, output string
		want         []Diagnostic
	}{
		{"nasm error", "boot.asm:10: error: instruction expected",
			[]Diagnostic{{File: "boot.asm", Line: 10, Severity: "error", Message: "instruction expected"}}},
		{"gcc warning with column", "src/main.c:12:5: warning: implicit declaration of function 'foo'",
			[]Diagnostic{{File: "src/main.c", Line: 12, Column: 5, Severity: "warning", Message: "implicit declaration of function 'foo'"}}},
		{"fatal error", "test.c:1:10: fatal error: nonexistent.h: No such file or directory",
			[]Diagnostic{{File: "test.c", Line: 1, Column: 10, Severity: "error", Message: "nonexistent.h: No such file or directory"}}},
		{"several", "boot.asm:5: error: comma expected\nboot.asm:10: warning: label alone on a line without a colon might be in error\nboot.asm:20: error: instruction expected",
			[]Diagnostic{
				{File: "boot.asm", Line: 5, Severity: "error", Message: "comma expected"},
				{File: "boot.asm", Line: 10, Severity: "warning", Message: "label alone on a line without a colon might be in error"},
				{File: "boot.asm", Line: 20, Severity: "error", Message: "instruction expected"},
			}},
		{"other lines skipped", "Compiling boot.asm...\nnasm -f bin boot.asm -o boot.bin\nboot.asm:10: error: instruction expected\nBuild complete.",
			[]Diagnostic{{File: "boot.asm", Line: 10, Severity: "error", Message: "instruction expected"}}},
		{"empty", "", nil},
		{"windows path", `C:\src\os\boot.asm:3: error: symbol 'x' not defined`,
			[]Diagnostic{{File: `C:\src\os\boot.asm`, Line: 3, Severity: "error", Message: "symbol 'x' not defined"}}},
		{"crlf and gcc context", "In file included from kernel.c:1:\r\nvga.h:4:1: error: unknown type name 'u8'\r\n    4 | u8 *vga;\r\n      | ^~\r\n",
			[]Diagnostic{{File: "vga.h", Line: 4, Column: 1, Severity: "error", Message: "unknown type name 'u8'"}}},
		{"note", "kernel.c:9:3: note: each undeclared identifier is reported only once",
			[]Diagnostic{{File: "kernel.c", Line: 9, Column: 3, Severity: "note", Message: "each undeclared identifier is reported only once"}}},
		{"no message", "boot.asm:4: error:", nil},
	}
	for _, c := range cases {
		if got := ParseGNU(c.output); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}

func TestParseCargoJSON(t *testing.T) {
	errLine := `{"reason":"compiler-message","message":{"message":"cannot find value ` + "`x`" + ` in this scope","code":{"code":"E0425"},"level":"error","spans":[{"file_name":"src/main.rs","byte_start":100,"byte_end":101,"line_start":10,"line_end":10,"column_start":15,"column_end":16,"is_primary":true}]}}`
	got := ParseCargoJSON(errLine)
	want := []Diagnostic{{File: "src/main.rs", Line: 10, Column: 15, EndLine: 10, EndColumn: 16, Severity: "error",
		Message: "cannot find value `x` in this scope"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("error: got %+v", got)
	}

	warn := `{"reason":"compiler-message","message":{"message":"unused variable: ` + "`y`" + `","code":{"code":"W0001"},"level":"warning","spans":[{"file_name":"src/lib.rs","byte_start":50,"byte_end":51,"line_start":5,"line_end":5,"column_start":9,"column_end":10,"is_primary":true}]}}`
	if got := ParseCargoJSON(warn); len(got) != 1 || got[0].Severity != "warning" {
		t.Errorf("warning: got %+v", got)
	}

	mixed := `{"reason":"compiler-artifact","package_id":"foo 0.1.0"}
{"reason":"compiler-message","message":{"message":"unused import","level":"warning","spans":[{"file_name":"src/main.rs","line_start":1,"line_end":1,"column_start":5,"column_end":10,"is_primary":true}]}}
{"reason":"build-finished","success":false}`
	if got := ParseCargoJSON(mixed); len(got) != 1 || got[0].Message != "unused import" {
		t.Errorf("mixed: got %+v", got)
	}

	if got := ParseCargoJSON(""); got != nil {
		t.Errorf("empty: got %+v", got)
	}
}

func TestParseDiagnosticsPrefersCargoJSON(t *testing.T) {
	out := `{"reason":"compiler-message","message":{"message":"test error","level":"error","spans":[{"file_name":"src/main.rs","line_start":1,"line_end":1,"column_start":1,"column_end":2,"is_primary":true}]}}` +
		"\nsrc/main.rs:1:1: error: test error"
	if got := ParseDiagnostics(out); len(got) != 1 || got[0].EndColumn != 2 {
		t.Errorf("got %+v, want only the JSON diagnostic", got)
	}
	if got := ParseDiagnostics("\nboot.asm:10: error: instruction expected"); len(got) != 1 || got[0].File != "boot.asm" {
		t.Errorf("GNU fallback: got %+v", got)
	}
}
