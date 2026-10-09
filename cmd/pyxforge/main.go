// Command pyxforge is PyxForge 3.0: the desktop app and its command line.
//
//	pyxforge [folder]            open the desktop app
//	pyxforge <command> [options] run a command; see `pyxforge help`
package main

import (
	"os"

	"fyne.io/fyne/v2/app"
	"github.com/obstinix/PyxForge/internal/cli"
	"github.com/obstinix/PyxForge/internal/ui/shell"
)

func main() {
	env, stop := cli.DefaultEnv()
	res := cli.Run(os.Args[1:], env)
	stop()
	if !res.OpenGUI {
		os.Exit(res.Exit)
	}
	a := app.NewWithID("io.github.obstinix.pyxforge")
	s := shell.New(a, res.Folder)
	for _, f := range res.Files {
		s.OpenFile(f)
	}
	s.Window().ShowAndRun()
}
