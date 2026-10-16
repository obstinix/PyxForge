package main

import (
	"bytes"
	"strings"
	"testing"
)

// A shortened real `go test -json` stream: a pass, a skip with its reason, a failure, a
// subtest skip, and a package that did not build.
const stream = `{"Action":"start","Package":"p/qemu"}
{"Action":"run","Package":"p/qemu","Test":"TestArgs"}
{"Action":"output","Package":"p/qemu","Test":"TestArgs","Output":"=== RUN   TestArgs\n"}
{"Action":"output","Package":"p/qemu","Test":"TestArgs","Output":"--- PASS: TestArgs (0.00s)\n"}
{"Action":"pass","Package":"p/qemu","Test":"TestArgs","Elapsed":0}
{"Action":"run","Package":"p/qemu","Test":"TestLaunch"}
{"Action":"output","Package":"p/qemu","Test":"TestLaunch","Output":"=== RUN   TestLaunch\n"}
{"Action":"output","Package":"p/qemu","Test":"TestLaunch","Output":"    qemu_test.go:175: qemu-system-x86_64 is not installed\n"}
{"Action":"output","Package":"p/qemu","Test":"TestLaunch","Output":"--- SKIP: TestLaunch (0.00s)\n"}
{"Action":"skip","Package":"p/qemu","Test":"TestLaunch","Elapsed":0}
{"Action":"run","Package":"p/build","Test":"TestReal/rustc"}
{"Action":"output","Package":"p/build","Test":"TestReal/rustc","Output":"    build_test.go:300: rustc is not installed\n"}
{"Action":"skip","Package":"p/build","Test":"TestReal/rustc","Elapsed":0}
{"Action":"run","Package":"p/build","Test":"TestParse"}
{"Action":"output","Package":"p/build","Test":"TestParse","Output":"    diagnostics_test.go:9: got nothing\n"}
{"Action":"fail","Package":"p/build","Test":"TestParse","Elapsed":0}
{"Action":"fail","Package":"p/build","Elapsed":0.1}
{"Action":"build-fail","Package":"p/broken"}
not json from go itself
`

func TestSummarize(t *testing.T) {
	var out bytes.Buffer
	r, err := summarize(strings.NewReader(stream), &out)
	if err != nil {
		t.Fatal(err)
	}
	if r.passed != 1 || r.failed != 1 {
		t.Errorf("passed %d, failed %d", r.passed, r.failed)
	}
	want := []string{"p/build TestReal/rustc: rustc is not installed", "p/qemu TestLaunch: qemu-system-x86_64 is not installed"}
	if strings.Join(r.skipped, "|") != strings.Join(want, "|") {
		t.Errorf("skipped %q", r.skipped)
	}
	if strings.Join(r.broken, ",") != "p/build,p/broken" {
		t.Errorf("broken %q", r.broken)
	}
	if !strings.Contains(out.String(), "--- SKIP: TestLaunch") || !strings.Contains(out.String(), "not json from go itself") {
		t.Errorf("output not passed through:\n%s", out.String())
	}
	if len(r.failures) != 1 || r.failures[0] != "p/build TestParse\n      diagnostics_test.go:9: got nothing" {
		t.Errorf("failures %q", r.failures)
	}
	var rep bytes.Buffer
	report(&rep, r, true)
	if !strings.Contains(rep.String(), "  failed  p/build TestParse\n      diagnostics_test.go:9: got nothing\n  FAILED  p/build") {
		t.Errorf("the failure is not at the end of the report:\n%s", rep.String())
	}
	if !strings.Contains(rep.String(), "1 passed, 1 failed, 2 skipped") ||
		!strings.Contains(rep.String(), "::notice title=2 tests skipped::p/build TestReal/rustc: rustc is not installed%0Ap/qemu") {
		t.Errorf("report:\n%s", rep.String())
	}
}
