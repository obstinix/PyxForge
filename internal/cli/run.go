package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/obstinix/PyxForge/internal/build"
	"github.com/obstinix/PyxForge/internal/config"
	"github.com/obstinix/PyxForge/internal/inspect"
	"github.com/obstinix/PyxForge/internal/qemu"
	"github.com/obstinix/PyxForge/internal/workspace"
)

// project loads the pyxforge.toml around folder (the working folder when empty).
func project(env Env, cmd, folder string) (*config.Config, string, bool) {
	if folder == "" {
		wd, err := env.Getwd()
		if err != nil {
			fmt.Fprintf(env.Stderr, "pyxforge %s: %v\n", cmd, err)
			return nil, "", false
		}
		folder = wd
	}
	w, err := workspace.Discover(folder)
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge %s: %v\n", cmd, err)
		return nil, "", false
	}
	if w.ProjectFile == "" {
		fmt.Fprintf(env.Stderr, "pyxforge %s: no %s in %s or its parents\n", cmd, config.FileName, w.Dir)
		return nil, "", false
	}
	c, err := config.Load(w.Root)
	if err == nil {
		err = c.Validate()
	}
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge %s: %s: %v\n", cmd, w.ProjectFile, err)
		return nil, "", false
	}
	return c, w.Root, true
}

// runRun builds the project and boots it in QEMU, printing QEMU's output (the serial port,
// with -serial stdio) until QEMU exits or Ctrl+C stops it.
func runRun(env Env, args []string) Result {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	debug := fs.Bool("debug", false, "start paused with QEMU's GDB stub, for a debugger to attach")
	noBuild := fs.Bool("no-build", false, "boot the image as it is, without building first")
	dir := fs.String("C", "", "run in this folder instead of the working folder")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		return Result{Exit: ExitUsage}
	}
	c, root, ok := project(env, "run", *dir)
	if !ok {
		return Result{Exit: ExitFailure}
	}
	if c.Qemu == nil {
		fmt.Fprintf(env.Stderr, "pyxforge run: %s has no [qemu] table saying what to boot\n", config.FileName)
		return Result{Exit: ExitFailure}
	}
	if !*noBuild && len(c.Profiles) > 0 {
		res, err := build.Run(env.Ctx, c, nil, build.Options{Root: root,
			OnLine: func(_, line string) { fmt.Fprintln(env.Stdout, line) }})
		if err != nil || !res.OK() {
			msg := "the build failed"
			if err != nil {
				msg = err.Error()
			} else if st := res.Steps[len(res.Steps)-1]; st.Err != "" {
				msg = st.Err
			}
			fmt.Fprintf(env.Stderr, "pyxforge run: %s; not booting\n", msg)
			return Result{Exit: ExitFailure}
		}
	}
	inst, err := qemu.Launch(env.Ctx, c.Qemu, qemu.Options{Root: root, Debug: *debug,
		OnOutput: func(line string) { fmt.Fprintln(env.Stdout, line) }})
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge run: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	fmt.Fprintf(env.Stderr, "QEMU %d: %s\n", inst.Pid(), strings.Join(inst.Args, " "))
	if *debug && c.Qemu.Debug.Enabled {
		g := c.GdbFor("")
		fmt.Fprintf(env.Stderr, "Paused with the GDB stub on port %d. Attach with:\n  %s -ex \"target remote localhost:%d\" -ex \"set architecture %s\"\n",
			c.Qemu.Debug.GdbPort, g.Executable, c.Qemu.Debug.GdbPort, g.Architecture)
	}
	fmt.Fprintln(env.Stderr, "Ctrl+C stops QEMU.")
	select {
	case <-inst.Done():
		if err := inst.Err(); err != nil {
			fmt.Fprintf(env.Stderr, "QEMU exited: %v\n", err)
			return Result{Exit: ExitFailure}
		}
	case <-env.Ctx.Done():
		inst.Stop()
		fmt.Fprintln(env.Stderr, "QEMU stopped.")
	}
	return Result{Exit: ExitOK}
}

// runInspect describes a binary: an ELF file's headers, sections and symbols, or a raw
// image's boot sector and its first bytes, with disassembly on request.
func runInspect(env Env, args []string) Result {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	jsonOut := fs.Bool("json", false, "print machine-readable JSON")
	disasm := fs.Bool("disasm", false, "disassemble the code (a raw image from its start, an ELF file from its entry)")
	arch := fs.String("arch", "i8086", "for raw images: i8086 (real mode), i386 or i386:x86-64")
	base := fs.Uint64("base", 0x7c00, "for raw images: the address the image loads at")
	var files []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return Result{Exit: ExitUsage}
		}
		args = fs.Args()
		if len(args) > 0 {
			files = append(files, args[0])
			args = args[1:]
		}
	}
	if len(files) != 1 {
		fmt.Fprintln(env.Stderr, "pyxforge inspect: name one file, such as build/boot.bin")
		return Result{Exit: ExitUsage}
	}
	f, err := os.Open(files[0])
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge inspect: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	defer f.Close()

	magic := make([]byte, 4)
	n, _ := f.ReadAt(magic, 0)
	if string(magic[:n]) == "\x7fELF" {
		e, err := inspect.ReadELF(f)
		if err != nil {
			fmt.Fprintf(env.Stderr, "pyxforge inspect: %v\n", err)
			return Result{Exit: ExitFailure}
		}
		var code []inspect.Instruction
		if *disasm {
			if bytes, start, err := inspect.CodeAt(f, e.Entry); err == nil && e.Mode != 0 {
				off := e.Entry - start
				code = inspect.Disassemble(bytes[off:min(off+64, uint64(len(bytes)))], e.Entry, e.Mode, true)
			}
		}
		if *jsonOut {
			writeJSON(env.Stdout, struct {
				*inspect.ELF
				Code []inspect.Instruction `json:"code,omitempty"`
			}{e, code})
			return Result{Exit: ExitOK}
		}
		fmt.Fprintf(env.Stdout, "%s %s %s, entry 0x%x\n\nSections\n", e.Class, e.Type, e.Machine, e.Entry)
		for _, s := range e.Sections {
			fmt.Fprintf(env.Stdout, "  %-20s 0x%08x %8d bytes  %s\n", s.Name, s.Addr, s.Size, s.Flags)
		}
		fmt.Fprintln(env.Stdout, "\nLoaded segments")
		for _, s := range e.Segments {
			fmt.Fprintf(env.Stdout, "  0x%08x %8d bytes in memory, %8d in the file  %s\n", s.VAddr, s.MemSize, s.FileSize, s.Flags)
		}
		if len(e.Symbols) > 0 {
			fmt.Fprintln(env.Stdout, "\nSymbols")
			for _, s := range e.Symbols {
				fmt.Fprintf(env.Stdout, "  0x%08x  %-6s %s\n", s.Addr, s.Kind, s.Name)
			}
		}
		printCode(env, code)
		return Result{Exit: ExitOK}
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		fmt.Fprintf(env.Stderr, "pyxforge inspect: %v\n", err)
		return Result{Exit: ExitFailure}
	}
	boot := inspect.BootSector(data)
	var code []inspect.Instruction
	if *disasm {
		code = inspect.Disassemble(data[:min(len(data), max(boot.Used, 16))], *base, inspect.ModeOf(*arch), true)
	}
	if *jsonOut {
		writeJSON(env.Stdout, struct {
			Boot inspect.Boot          `json:"boot"`
			Code []inspect.Instruction `json:"code,omitempty"`
		}{boot, code})
		return Result{Exit: ExitOK}
	}
	switch {
	case boot.Size < 512:
		fmt.Fprintf(env.Stdout, "%d bytes: shorter than a 512-byte sector, so a BIOS will not boot it.\n", boot.Size)
	case boot.Signature:
		fmt.Fprintf(env.Stdout, "Boot sector: %d bytes, signature 55 aa at 510: a BIOS boots it.\n", boot.Size)
	default:
		fmt.Fprintf(env.Stdout, "%d bytes, but no 55 aa signature at offset 510: a BIOS will not boot it.\n", boot.Size)
	}
	if boot.Size >= 512 {
		fmt.Fprintf(env.Stdout, "Code and data use %d of the sector's 510 bytes; %d are free.\n", boot.Used, boot.Free)
	}
	fmt.Fprintln(env.Stdout)
	for _, l := range inspect.Dump(data[:min(len(data), 512)], 0) {
		fmt.Fprintln(env.Stdout, l.String())
	}
	printCode(env, code)
	return Result{Exit: ExitOK}
}

func printCode(env Env, code []inspect.Instruction) {
	if len(code) == 0 {
		return
	}
	fmt.Fprintln(env.Stdout, "\nDisassembly")
	for _, in := range code {
		fmt.Fprintln(env.Stdout, in.String())
	}
}
