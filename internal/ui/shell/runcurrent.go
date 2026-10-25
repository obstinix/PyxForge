package shell

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2/storage"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/runfile"
	"github.com/obstinix/PyxForge/internal/toolchain"
	"github.com/obstinix/PyxForge/internal/ui/notifications"
)

// Run Current File (PXF-FEAT-003) runs the file in the editor on its own, as its language runs
// a program (internal/runfile), whether or not the project has a pyxforge.toml. A file to
// compile is compiled in the Build panel first, so its errors reach Problems and the editor;
// the program then runs in a Terminal session of its own, where it can read what is typed and
// Restart runs it again. Compiled programs go to PyxForge's cache, never into the project. An
// HTML file opens in the browser. Debug Current File does the same under the language's
// debugger (runfile.ForDebug), in its Terminal session.

// runOutDir is where programs compiled from files of this workspace go.
func (s *Shell) runOutDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	key := filepath.Clean(s.root)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key) // one workspace however it is spelled; Linux tells case apart (#23)
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(base, "PyxForge", "run", hex.EncodeToString(sum[:8]))
}

// runCurrentFile saves the file shown in the editor and runs it, or, with debug, starts it under
// its debugger.
func (s *Shell) runCurrentFile(debug bool) {
	bt := s.activeBuffer()
	switch {
	case bt == nil:
		s.Notify(notifications.Info, "No file to run", "Open a file in the editor first; Run Current File runs the one shown.")
		return
	case bt.path == "":
		s.Notify(notifications.Info, "Save the file first", "A new file needs a name, and so a language, before it can run.")
		s.saveAsDialogFor(bt)
		return
	}
	path, sess := bt.path, s.ed.sess
	go func() {
		saved, err := sess.Save() // the buffer shown: run what the user sees
		s.dispatch(func() {
			switch {
			case err != nil:
				s.Notify(notifications.Error, "Not saved, so not run", err.Error())
			case !saved:
				s.Notify(notifications.Info, "Not run", filepath.Base(path)+" changed on disk while it had unsaved changes: choose which to keep first.")
			default:
				s.runFile(path, debug)
			}
		})
	}()
}

// runFile runs one file by its language, or debugs it.
func (s *Shell) runFile(path string, debug bool) {
	name := filepath.Base(path)
	out := s.runOutDir()
	plan, err := runfile.For(path, out, toolchain.LookPath)
	session := "Run " + name
	if debug {
		breaks := runfile.Breakpoints{}
		for _, b := range s.currentBreakpoints() {
			breaks[b.File] = b.Lines
		}
		plan, err = runfile.ForDebug(path, out, toolchain.LookPath, breaks)
		session = "Debug " + name
	}
	var missing *runfile.MissingError
	switch {
	case errors.Is(err, runfile.ErrNoDebugger):
		s.Notify(notifications.Info, "Cannot debug "+name, "Debug Current File knows "+runfile.Debuggable+". Run Current File runs it.")
		return
	case errors.Is(err, runfile.ErrUnknown):
		s.Notify(notifications.Info, "Cannot run "+name, "Run Current File knows "+runfile.Known+". A project builds and boots through pyxforge.toml (Run menu).")
		return
	case errors.As(err, &missing):
		s.Notify(notifications.Error, missing.Language+" is not installed", missing.Error()+". Tools: Doctor shows what is found.")
		return
	case err != nil:
		s.Notify(notifications.Error, "Cannot run "+name, err.Error())
		return
	case plan.Open != "":
		s.openURL(storage.NewFileURI(plan.Open).String())
		return
	}
	run := func() {
		if plan.Note != "" {
			s.Notify(notifications.Info, session, plan.Note)
		}
		if err := plan.WriteFiles(); err != nil {
			s.Notify(notifications.Error, "Cannot "+strings.ToLower(strings.Fields(session)[0])+" "+name, err.Error())
			return
		}
		s.term.run(session, append([]string{plan.Run.Tool}, plan.Run.Args...), plan.Run.Dir)
	}
	if len(plan.Build) == 0 {
		run()
		return
	}
	if s.buildp.running {
		s.Notify(notifications.Info, "A build is running", "Run again when it ends.")
		return
	}
	if err := os.MkdirAll(plan.Out, 0o755); err != nil {
		s.Notify(notifications.Error, "Cannot run "+name, err.Error())
		return
	}
	// The compile step as a one-profile build of the file's folder, which keeps its tool
	// inside that folder; its output goes to the cache (-o).
	dir := filepath.Dir(path)
	cfg := &config.Config{Project: config.Project{Name: name}, Profiles: map[string]*config.Profile{}}
	prev := ""
	for i, st := range plan.Build {
		p := &config.Profile{Name: fmt.Sprintf("compile %s (%d)", name, i+1), Tool: st.Tool, Args: st.Args, SourceDir: ".", OutputDir: "."}
		if len(plan.Build) == 1 {
			p.Name = "compile " + name
		}
		if prev != "" {
			p.DependsOn = []string{prev}
		}
		cfg.Profiles[p.Name], prev = p, p.Name
	}
	s.showDockTab(slices.Index(s.dock.Items, s.buildp.tab))
	s.buildp.after = func(ok bool) {
		if ok {
			run()
		}
	}
	s.buildp.runConfig(cfg, nil, dir, true)
	if !s.buildp.running { // it could not start; runConfig said why
		s.buildp.after = nil
	}
}
