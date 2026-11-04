// Command coverreport summarises a Go cover profile per package, as a Markdown table sorted
// from the least to the most covered package, after the total. CI appends it to the job
// summary so untested packages are visible in every run (#44).
//
//	go test -coverprofile=coverage.out -coverpkg=./... ./...
//	go run ./tools/coverreport -module github.com/obstinix/PyxForge coverage.out
//
// With -coverpkg, every test binary reports every package's blocks, so the same block appears
// once per binary; a block counts as covered when any binary ran it.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type block struct {
	stmts   int
	covered bool
}

type pkgCover struct {
	name           string
	stmts, covered int
}

func (p pkgCover) percent() float64 {
	if p.stmts == 0 {
		return 0
	}
	return 100 * float64(p.covered) / float64(p.stmts)
}

func main() {
	module := flag.String("module", "", "module path to trim from package names")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: coverreport [-module path] coverage.out")
		os.Exit(2)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverreport:", err)
		os.Exit(1)
	}
	defer f.Close()
	pkgs, err := parse(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "coverreport:", err)
		os.Exit(1)
	}
	write(os.Stdout, pkgs, *module)
}

// parse reads a cover profile and returns per-package statement counts.
func parse(r io.Reader) ([]pkgCover, error) {
	blocks := map[string]*block{} // "file:range" -> block
	files := map[string]string{}  // "file:range" -> package
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if first {
			first = false
			if !strings.HasPrefix(line, "mode:") {
				return nil, errors.New("not a cover profile: the first line is not a mode line")
			}
			continue
		}
		if strings.HasPrefix(line, "mode:") {
			continue // profiles concatenated by hand repeat the mode line
		}
		// file.go:12.34,15.2 3 1
		colon := strings.LastIndex(line, ":")
		if colon < 0 {
			return nil, fmt.Errorf("malformed line %q", line)
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed line %q", line)
		}
		stmts, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("malformed counts in %q", line)
		}
		key := line[:colon] + ":" + fields[0]
		b := blocks[key]
		if b == nil {
			b = &block{stmts: stmts}
			blocks[key] = b
			files[key] = path.Dir(line[:colon])
		}
		if count > 0 {
			b.covered = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	byPkg := map[string]*pkgCover{}
	for key, b := range blocks {
		name := files[key]
		p := byPkg[name]
		if p == nil {
			p = &pkgCover{name: name}
			byPkg[name] = p
		}
		p.stmts += b.stmts
		if b.covered {
			p.covered += b.stmts
		}
	}
	out := make([]pkgCover, 0, len(byPkg))
	for _, p := range byPkg {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		if pi, pj := out[i].percent(), out[j].percent(); pi != pj {
			return pi < pj
		}
		return out[i].name < out[j].name
	})
	return out, nil
}

func write(w io.Writer, pkgs []pkgCover, module string) {
	var total pkgCover
	for _, p := range pkgs {
		total.stmts += p.stmts
		total.covered += p.covered
	}
	fmt.Fprintf(w, "**Statement coverage: %.1f%%** (%d of %d statements, %d packages)\n\n",
		total.percent(), total.covered, total.stmts, len(pkgs))
	fmt.Fprintln(w, "| Package | Coverage | Statements |")
	fmt.Fprintln(w, "|---|---:|---:|")
	for _, p := range pkgs {
		name := p.name
		if module != "" {
			name = strings.TrimPrefix(strings.TrimPrefix(name, module), "/")
			if name == "" {
				name = "."
			}
		}
		fmt.Fprintf(w, "| %s | %.1f%% | %d / %d |\n", name, p.percent(), p.covered, p.stmts)
	}
}
