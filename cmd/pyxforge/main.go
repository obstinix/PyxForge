// Command pyxforge is PyxForge 3.0: the desktop app and its command line.
//
//	pyxforge [folder]            open the desktop app
//	pyxforge <command> [options] run a command; see `pyxforge help`
package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/obstinix/PyxForge/internal/askpass"
	"github.com/obstinix/PyxForge/internal/cli"
	"github.com/obstinix/PyxForge/internal/crash"
	"github.com/obstinix/PyxForge/internal/proc"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/brand"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"github.com/obstinix/PyxForge/internal/ui/shell"
	"github.com/obstinix/PyxForge/internal/ui/splash"
)

func main() {
	began := time.Now()
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
	// A panic on the UI thread cannot be contained: the event loop is gone. It is still logged,
	// so the next start can be traced (owner review F1, Phase 7.5).
	defer func() {
		if r := recover(); r != nil {
			p, _ := crash.Write("the UI thread", r, debug.Stack())
			fmt.Fprintf(os.Stderr, "pyxforge: crashed: %v\nlog: %s\n", r, p)
			os.Exit(2)
		}
	}()
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
	// A panic in a worker is contained: logged, and said in the window (crash.Go).
	crash.Report = func(what, msg, log string) {
		fyne.Do(func() {
			if s != nil {
				s.ReportCrash(what, msg, log)
			}
		})
	}
	steps := []splash.Step{
		{Label: "Locating toolchains", Run: func() error {
			// Where each tool is, without running any: Doctor and the status bar report versions.
			for _, name := range []string{"git", "gcc", "nasm", "gdb", "qemu-system-x86_64"} {
				_, _ = toolchain.LookPath(name)
			}
			return nil
		}},
	}
	splash.Run(a, tokens, steps, splash.Options{ReduceMotion: reduce, MinDuration: minimum}, func(errs []error) {
		folder := res.Folder
		if folder == "" && len(res.Files) == 0 {
			folder = shell.ReopenFolder(a) // Settings > Files & Explorer > Reopen Last Workspace
		}
		if shell.IsBroadFolder(folder) {
			// A drive's root or the home folder is asked about first, so the window opens empty
			// and the question comes over it (owner review F1).
			s = shell.New(a, "")
			s.OpenFolder(folder)
		} else {
			s = shell.New(a, folder)
		}
		for _, f := range res.Files {
			s.OpenFile(f)
		}
		for _, err := range errs {
			s.Notify(notifications.Warning, "Start-up", err.Error())
		}
		s.Window().Show()
		// How long the start took, in the Log tab: owner review gate 7 asks under 1.5 s.
		s.LogStartup(time.Since(began), minimum)
		measure := os.Getenv("PYXFORGE_STARTUP_TIME") != "" // for measuring from outside
		s.WatchStalls(func(d time.Duration) {
			if measure {
				fmt.Fprintf(os.Stderr, "pyxforge: the window did not answer for %d ms\n", d.Milliseconds())
			}
		})
		if measure {
			fmt.Fprintf(os.Stderr, "pyxforge: started in %d ms\n", time.Since(began).Milliseconds())
		}
	})
	// Ctrl+C in the starting terminal or SIGTERM quits like closing the window. Whatever ends
	// the app's loop, the processes it started (QEMU, GDB, language servers, terminals) are stopped before it exits.
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
