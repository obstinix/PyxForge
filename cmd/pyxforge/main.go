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
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/obstinix/PyxForge/internal/askpass"
	"github.com/obstinix/PyxForge/internal/cli"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/brand"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/shell"
	"github.com/obstinix/PyxForge/internal/ui/splash"
	"github.com/obstinix/PyxForge/nvim"
)

func main() {
	// Started by git or ssh as the askpass helper of a running PyxForge: pass the question on
	// to its window and print the answer, nothing else.
	if askpass.Requested() {
		os.Exit(askpass.Client(os.Args[1:], os.Stdout))
	}
	// Every process PyxForge starts ends with it, even if PyxForge is killed.
	if err := proc.ContainChildren(); err != nil {
		fmt.Fprintln(os.Stderr, "pyxforge: warning: child processes may outlive a forced exit:", err)
	}
	// Tool paths set in Doctor apply to the command line and the desktop app alike.
	if _, err := toolchain.LoadOverrides(); err != nil {
		fmt.Fprintln(os.Stderr, "pyxforge: warning: tool paths set in Doctor were not read:", err)
	}
	// What PyxForge starts finds the tools bundled with it and the toolchains it installed,
	// after the user's own on PATH.
	toolchain.ExtendPath()
	env, stop := cli.DefaultEnv()
	res := cli.Run(os.Args[1:], env)
	stop()
	if !res.OpenGUI {
		os.Exit(res.Exit)
	}
	releaseOwnConsole() // started from Explorer: no console window beside the app
	a := app.NewWithID("io.github.obstinix.pyxforge")
	a.SetIcon(brand.Icon)
	tokens := shell.ApplyAppearance(a)
	reduce := shell.ReduceMotion(a) || splash.SystemReducesMotion()
	minimum := splash.MinDuration // the entrance and a moment to read it, no longer
	if reduce {
		minimum = splash.MinStillDuration // drawn still, but not a flash on a fast start
	}
	// Start-up work runs behind the start-up screen, off the UI thread; the main window opens
	// when it is done, and anything that failed is said there.
	var s *shell.Shell
	steps := []splash.Step{
		{Label: "Preparing the editor", Run: func() error {
			base, err := nvim.DefaultBase()
			if err == nil {
				_, err = nvim.Install(base) // unpacks PyxForge's Neovim configuration once per version
			}
			if err == nil {
				_, _ = nvim.Prune(base, 24*time.Hour) // older versions' copies; a failure only costs disk
			}
			if err != nil {
				return fmt.Errorf("the editor's configuration could not be prepared: %w", err)
			}
			return nil
		}},
		{Label: "Locating toolchains", Run: func() error {
			// Where each tool is, without running any: Doctor and the status bar report versions.
			for _, name := range []string{"nvim", "git", "gcc", "nasm", "gdb", "qemu-system-x86_64"} {
				_, _ = toolchain.LookPath(name)
			}
			return nil
		}},
	}
	splash.Run(a, tokens, steps, splash.Options{ReduceMotion: reduce, MinDuration: minimum}, func(errs []error) {
		s = shell.New(a, res.Folder)
		for _, f := range res.Files {
			s.OpenFile(f)
		}
		for _, err := range errs {
			s.Notify(notifications.Warning, "Start-up", err.Error())
		}
		s.Window().Show()
	})
	// Ctrl+C in the starting terminal or SIGTERM quits like closing the window. Whatever ends
	// the app's loop, the processes it started (QEMU, GDB, Neovim) are stopped before it exits.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fyne.Do(a.Quit)
	}()
	a.Run()
	if s != nil {
		s.Shutdown()
	}
}
