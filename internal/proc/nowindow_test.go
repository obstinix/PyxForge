package proc

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryChildHidesItsConsole: each function in the app that starts a program with os/exec
// also calls NoWindow, so the windowed Windows build opens no console windows (PXF-DEF-028).
// Files that do not build on Windows are skipped; tools/ holds developer programs run from a
// terminal.
func TestEveryChildHidesItsConsole(t *testing.T) {
	ctx := build.Default
	ctx.GOOS, ctx.CgoEnabled = "windows", true
	root := filepath.Join("..", "..")
	checked := 0
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			if ok, err := ctx.MatchFile(filepath.Dir(path), d.Name()); err != nil || !ok {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				starts, hides := false, false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch f := call.Fun.(type) {
					case *ast.SelectorExpr:
						if x, ok := f.X.(*ast.Ident); ok && x.Name == "exec" && strings.HasPrefix(f.Sel.Name, "Command") {
							starts = true
						}
						hides = hides || f.Sel.Name == "NoWindow"
					case *ast.Ident:
						hides = hides || f.Name == "noWindow"
					}
					return true
				})
				if starts {
					checked++
					if !hides {
						t.Errorf("%s: %s starts a program without proc.NoWindow", filepath.ToSlash(path), fn.Name.Name)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked < 10 {
		t.Fatalf("found only %d functions that start programs; the scan is looking in the wrong place", checked)
	}
}
