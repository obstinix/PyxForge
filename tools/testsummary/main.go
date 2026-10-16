// Command testsummary reads `go test -json` and prints the tests' output as go test would,
// then how many tests passed, failed and were skipped, with each skipped test and its reason.
// The integration tests skip when a tool (QEMU, GDB, Neovim, a compiler) is missing, and a
// package whose tests all skipped still prints "ok"; this makes the difference visible. Under
// GitHub Actions the skips also become a notice annotation. It exits 1 when anything failed.
//
//	go test -json ./... | go run ./tools/testsummary
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type event struct {
	Action  string
	Package string
	Test    string
	Output  string
}

// result is what a run came to.
type result struct {
	passed, failed int
	skipped        []string // "package Test: reason"
	broken         []string // packages that failed outside any test (build errors, a crash)
}

func main() {
	r, err := summarize(os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testsummary:", err)
		os.Exit(1)
	}
	report(os.Stdout, r, os.Getenv("GITHUB_ACTIONS") == "true")
	if r.failed > 0 || len(r.broken) > 0 {
		os.Exit(1)
	}
}

func summarize(in io.Reader, out io.Writer) (result, error) {
	var r result
	reasons := map[string][]string{} // a test's own output lines, for its skip reason
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var e event
		if json.Unmarshal(line, &e) != nil || e.Action == "" {
			fmt.Fprintln(out, string(line)) // not JSON: pass it through
			continue
		}
		key := e.Package + " " + e.Test
		switch e.Action {
		case "output", "build-output":
			fmt.Fprint(out, e.Output)
			if e.Test != "" {
				if t := strings.TrimSpace(e.Output); t != "" && !strings.HasPrefix(t, "=== ") && !strings.HasPrefix(t, "--- ") {
					reasons[key] = append(reasons[key], t)
				}
			}
		case "pass":
			if e.Test != "" {
				r.passed++
			}
		case "fail", "build-fail":
			if e.Test != "" {
				r.failed++
			} else {
				r.broken = append(r.broken, e.Package)
			}
		case "skip":
			if e.Test != "" {
				why := strings.Join(reasons[key], " ")
				if i := strings.Index(why, ": "); i >= 0 && strings.Contains(why[:i], "_test.go:") {
					why = why[i+2:] // drop the file:line prefix
				}
				r.skipped = append(r.skipped, fmt.Sprintf("%s %s: %s", e.Package, e.Test, why))
			}
		}
	}
	sort.Strings(r.skipped)
	return r, sc.Err()
}

func report(w io.Writer, r result, actions bool) {
	fmt.Fprintf(w, "\n%d passed, %d failed, %d skipped\n", r.passed, r.failed, len(r.skipped))
	for _, s := range r.skipped {
		fmt.Fprintln(w, "  skipped "+s)
	}
	for _, p := range r.broken {
		fmt.Fprintln(w, "  FAILED  "+p)
	}
	if actions && len(r.skipped) > 0 {
		msg := strings.Join(r.skipped, "\n")
		msg = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(msg)
		fmt.Fprintf(w, "::notice title=%d tests skipped::%s\n", len(r.skipped), msg)
	}
}
