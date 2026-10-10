package build

import (
	"path/filepath"
	"reflect"
	"strings"
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

// The fixtures below are real tool output: GCC 13.3 and Clang 22, NASM 2.16, GNU ld 2.42.

const gccSARIF = `{"$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json", "version": "2.1.0", "runs": [{"tool": {"driver": {"name": "GNU C17", "version": "13.3.0", "rules": []}}, "originalUriBaseIds": {"PWD": {"uri": "file:///tmp/my%20os/"}}, "results": [{"ruleId": "error", "level": "error", "message": {"text": "‘undeclared’ undeclared (first use in this function)"}, "locations": [{"physicalLocation": {"artifactLocation": {"uri": "x.c", "uriBaseId": "PWD"}, "region": {"startLine": 2, "startColumn": 10, "endColumn": 20}}}]}, {"level": "warning", "message": {"text": "unused variable ‘v’"}, "locations": [{"physicalLocation": {"artifactLocation": {"uri": "x.c", "uriBaseId": "PWD"}, "region": {"startLine": 3, "startColumn": 7}}}]}]}]}`

const clangSARIF = `{"$schema":"https://docs.oasis-open.org/sarif/sarif/v2.1.0/cos02/schemas/sarif-schema-2.1.0.json","runs":[{"results":[{"level":"error","locations":[{"physicalLocation":{"artifactLocation":{"index":0,"uri":"file:///C:/%2F/Users/dev/AppData/Local/Temp/x.c"},"region":{"endColumn":33,"endLine":1,"startColumn":23,"startLine":1}}}],"message":{"text":"use of undeclared identifier 'undeclared'"}}],"tool":{"driver":{"name":"clang","version":"22.1.8"}}}],"version":"2.1.0"}`

const gccJSON = `[{"kind": "error", "message": "‘undeclared’ undeclared (first use in this function)", "children": [{"kind": "note", "message": "each undeclared identifier is reported only once"}], "locations": [{"caret": {"file": "x.c", "line": 2, "column": 10}, "finish": {"file": "x.c", "line": 2, "column": 19}}]}]`

func TestParseStructured(t *testing.T) {
	got := ParseSARIF(gccSARIF)
	if len(got) != 2 || got[0].File != filepath.Clean(filepath.FromSlash("/tmp/my os/x.c")) || got[0].Line != 2 || got[0].Column != 10 ||
		got[0].Severity != "error" || got[1].Severity != "warning" || got[1].Line != 3 {
		t.Errorf("GCC SARIF: %+v", got)
	}
	got = ParseSARIF(clangSARIF)
	if len(got) != 1 || got[0].Line != 1 || got[0].Column != 23 || got[0].Message != "use of undeclared identifier 'undeclared'" ||
		!strings.HasSuffix(filepath.ToSlash(got[0].File), "/Users/dev/AppData/Local/Temp/x.c") || strings.Contains(got[0].File, "%") {
		t.Errorf("Clang SARIF: %+v", got)
	}
	got = ParseGCCJSON(gccJSON)
	if len(got) != 1 || got[0].File != "x.c" || got[0].Line != 2 || got[0].Column != 10 || got[0].EndColumn != 19 {
		t.Errorf("GCC JSON: %+v", got)
	}
	// Structured output and the linker's text in one build: both kept, nothing twice.
	out := gccSARIF + "\nx.c:2:10: error: ‘undeclared’ undeclared (first use in this function)\ncollect2: error: ld returned 1 exit status\n"
	all := ParseDiagnostics(out)
	if len(all) != 3 || all[2].Tool != "collect2" || all[2].File != "" {
		t.Errorf("merged: %+v", all)
	}
	if got := ParseSARIF(`{"runs": [ broken`); got != nil {
		t.Errorf("malformed SARIF: %+v", got)
	}
}

func TestParseUnlocated(t *testing.T) {
	out := strings.Join([]string{
		"nasm: fatal: unable to open input file `nope.asm' No such file or directory",
		"ld: cannot find /tmp/none.o: No such file or directory",
		"/usr/bin/ld: /tmp/ccG92hYO.o: in function `main':",
		"y.c:(.text+0x9): undefined reference to `f'",
		"collect2: error: ld returned 1 exit status",
		"ld.lld: warning: cannot find entry symbol _start",
		"my os/boot.asm:3: error: label `x' inconsistently redefined",
		"make: *** [Makefile:3: all] Error 1",
	}, "\n")
	got := ParseGNU(out)
	want := []Diagnostic{
		{Tool: "nasm", Severity: "error", Message: "unable to open input file `nope.asm' No such file or directory"},
		{Tool: "ld", Severity: "error", Message: "cannot find /tmp/none.o: No such file or directory"},
		{File: "y.c", Tool: "ld", Severity: "error", Message: "undefined reference to `f' (.text+0x9)"},
		{Tool: "collect2", Severity: "error", Message: "ld returned 1 exit status"},
		{Tool: "ld.lld", Severity: "warning", Message: "cannot find entry symbol _start"},
		{File: "my os/boot.asm", Line: 3, Severity: "error", Message: "label `x' inconsistently redefined"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unlocated:\n got %+v\nwant %+v", got, want)
	}
}
