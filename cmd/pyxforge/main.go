// Command pyxforge is PyxForge 3.0: the desktop app and its command line.
//
//	pyxforge [folder]            open the desktop app
//	pyxforge <command> [options] run a command; see `pyxforge help`
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/obstinix/PyxForge/internal/cli"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/ui/shell"
)

func main() {
	// Every process PyxForge starts ends with it, even if PyxForge is killed.
	if err := proc.ContainChildren(); err != nil {
		fmt.Fprintln(os.Stderr, "pyxforge: warning: child processes may outlive a forced exit:", err)
	}
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
	// Ctrl+C in the starting terminal or SIGTERM quits like closing the window. Whatever ends
	// the app's loop, the processes it started (QEMU, GDB, Neovim) are stopped before it exits.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fyne.Do(a.Quit)
	}()
	s.Window().ShowAndRun()
	s.Shutdown()
}
