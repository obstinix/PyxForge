// Command pyxforge is the PyxForge 3.0 desktop application.
//
//	pyxforge [workspace]
//
// The workspace defaults to the current directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2/app"
	"github.com/obstinix/PyxForge/internal/ui/shell"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	root, err := filepath.Abs(root)
	if err == nil {
		var fi os.FileInfo
		if fi, err = os.Stat(root); err == nil && !fi.IsDir() {
			err = fmt.Errorf("not a folder")
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyxforge: %s: %v\n", root, err)
		os.Exit(2)
	}
	a := app.NewWithID("io.github.obstinix.pyxforge")
	shell.New(a, root).Window().ShowAndRun()
}
