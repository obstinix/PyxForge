// Command crashdump runs a program under the Windows debugging API and, if the program dies of
// an exception nothing handled, writes a minidump and a short text report before letting it
// end (#80). Windows Error Reporting can't do this for Neovim: libuv sets
// SEM_NOGPFAULTERRORBOX, so its crashes leave no dump, and Neovim's release ships no symbols.
// The report names the faulting module and offset and the return addresses found on the
// crashing thread's stack, which is enough to place a crash without symbols.
//
//	crashdump program [args...]
//
// The program gets crashdump's standard input, output and error, its environment and folder,
// and crashdump exits with the program's exit status. Only the program is debugged, not the
// processes it starts. Dumps go to PYXFORGE_CRASHDUMP_DIR, or the temporary folder:
// <program>-<pid>-<time>.dmp and .txt. CI starts Neovim through it (PYXFORGE_NVIM_WRAPPER).
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	debugOnlyThisProcess = 0x00000002

	exceptionDebugEvent     = 1
	createThreadDebugEvent  = 2
	createProcessDebugEvent = 3
	exitThreadDebugEvent    = 4
	exitProcessDebugEvent   = 5
	loadDLLDebugEvent       = 6
	unloadDLLDebugEvent     = 7

	dbgContinue            = 0x00010002
	dbgExceptionNotHandled = 0x80010001

	exceptionBreakpoint     = 0x80000003
	exceptionWX86Breakpoint = 0x4000001F

	contextFull = 0x0010000B // CONTEXT_AMD64 | CONTROL | INTEGER | FLOATING_POINT

	// MiniDumpWithDataSegs | WithHandleData | WithUnloadedModules |
	// WithIndirectlyReferencedMemory | WithProcessThreadData | WithThreadInfo
	dumpType = 0x1 | 0x4 | 0x20 | 0x40 | 0x100 | 0x1000
)

var (
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	waitForDebugEvent  = kernel32.NewProc("WaitForDebugEvent")
	continueDebugEvent = kernel32.NewProc("ContinueDebugEvent")
	getThreadContext   = kernel32.NewProc("GetThreadContext")
	miniDumpWriteDump  = windows.NewLazySystemDLL("dbghelp.dll").NewProc("MiniDumpWriteDump")
)

func main() {
	runtime.LockOSThread() // debug events go to the thread that started the program
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: crashdump program [args...]")
		os.Exit(2)
	}
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "crashdump:", err)
		os.Exit(1)
	}
	os.Exit(int(code))
}

// module is a DLL or the program's executable, loaded at base.
type module struct {
	base, size uintptr
	path       string
}

func run(args []string) (uint32, error) {
	exe, err := exec.LookPath(args[0])
	if err != nil {
		return 0, err
	}
	std := func(which uint32) windows.Handle {
		h, _ := windows.GetStdHandle(which)
		_ = windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT)
		return h
	}
	si := &windows.StartupInfo{
		Cb:        uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Flags:     windows.STARTF_USESTDHANDLES,
		StdInput:  std(windows.STD_INPUT_HANDLE),
		StdOutput: std(windows.STD_OUTPUT_HANDLE),
		StdErr:    std(windows.STD_ERROR_HANDLE),
	}
	exe16, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, err
	}
	cmd16, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(args))
	if err != nil {
		return 0, err
	}
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(exe16, cmd16, nil, nil, true, debugOnlyThisProcess, nil, nil, si, &pi); err != nil {
		return 0, fmt.Errorf("start %s: %w", exe, err)
	}
	_ = windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)
	// The program has its own copies of the standard handles. Closing these lets the program's
	// reader see the end of its output when the program ends, not when crashdump does.
	for _, std := range []uint32{windows.STD_INPUT_HANDLE, windows.STD_OUTPUT_HANDLE} {
		if h, err := windows.GetStdHandle(std); err == nil {
			_ = windows.CloseHandle(h)
		}
	}

	var ev [32]uint64 // DEBUG_EVENT, 176 bytes on x64
	b := (*[256]byte)(unsafe.Pointer(&ev[0]))
	modules := map[uintptr]module{}
	threads := map[uint32]windows.Handle{}
	for {
		if r, _, e := waitForDebugEvent.Call(uintptr(unsafe.Pointer(&ev[0])), uintptr(windows.INFINITE)); r == 0 {
			return 0, fmt.Errorf("WaitForDebugEvent: %v", e)
		}
		code := binary.LittleEndian.Uint32(b[0:])
		pid := binary.LittleEndian.Uint32(b[4:])
		tid := binary.LittleEndian.Uint32(b[8:])
		u := b[16:]
		cont := uintptr(dbgContinue)
		switch code {
		case createProcessDebugEvent: // hFile, hProcess, hThread, lpBaseOfImage
			addModule(pi.Process, modules, handleAt(u, 0), ptrAt(u, 24))
			threads[tid] = handleAt(u, 16)
		case createThreadDebugEvent: // hThread
			threads[tid] = handleAt(u, 0)
		case exitThreadDebugEvent:
			delete(threads, tid)
		case loadDLLDebugEvent: // hFile, lpBaseOfDll
			addModule(pi.Process, modules, handleAt(u, 0), ptrAt(u, 8))
		case unloadDLLDebugEvent: // lpBaseOfDll
			delete(modules, ptrAt(u, 0))
		case exceptionDebugEvent: // EXCEPTION_RECORD (152 bytes), dwFirstChance
			ex := binary.LittleEndian.Uint32(u[0:])
			first := binary.LittleEndian.Uint32(u[152:]) != 0
			switch {
			case ex == exceptionBreakpoint || ex == exceptionWX86Breakpoint:
				// the loader's breakpoint as the program starts
			case first:
				cont = dbgExceptionNotHandled // the program's own handlers come first
			default:
				report(pi.Process, pid, tid, threads[tid], u[:152], modules, exe)
				cont = dbgExceptionNotHandled // and then it ends, as it would have
			}
		case exitProcessDebugEvent: // dwExitCode
			status := binary.LittleEndian.Uint32(u[0:])
			continueDebugEvent.Call(uintptr(pid), uintptr(tid), dbgContinue)
			return status, nil
		}
		continueDebugEvent.Call(uintptr(pid), uintptr(tid), cont)
	}
}

func handleAt(b []byte, off int) windows.Handle {
	return windows.Handle(binary.LittleEndian.Uint64(b[off:]))
}
func ptrAt(b []byte, off int) uintptr { return uintptr(binary.LittleEndian.Uint64(b[off:])) }

// addModule records a module from its file handle, which the debugger must close, and its
// size from the PE header in the program's memory.
func addModule(process windows.Handle, modules map[uintptr]module, file windows.Handle, base uintptr) {
	m := module{base: base}
	if file != 0 {
		buf := make([]uint16, windows.MAX_LONG_PATH)
		if n, err := windows.GetFinalPathNameByHandle(file, &buf[0], uint32(len(buf)), 0); err == nil && int(n) < len(buf) {
			m.path = strings.TrimPrefix(windows.UTF16ToString(buf[:n]), `\\?\`)
		}
		_ = windows.CloseHandle(file)
	}
	if lfanew, ok := read32(process, base+0x3C); ok {
		if size, ok := read32(process, base+uintptr(lfanew)+0x50); ok { // OptionalHeader.SizeOfImage (PE32+)
			m.size = uintptr(size)
		}
	}
	modules[base] = m
}

func read32(process windows.Handle, addr uintptr) (uint32, bool) {
	var v uint32
	var n uintptr
	err := windows.ReadProcessMemory(process, addr, (*byte)(unsafe.Pointer(&v)), 4, &n)
	return v, err == nil && n == 4
}

// where names the module an address is in, as module+0xoffset.
func where(modules map[uintptr]module, addr uintptr) (string, bool) {
	for _, m := range modules {
		if m.size != 0 && addr >= m.base && addr < m.base+m.size {
			return fmt.Sprintf("%s+0x%x", filepath.Base(m.path), addr-m.base), true
		}
	}
	return fmt.Sprintf("0x%x", addr), false
}

// report writes the minidump and the text report for an exception nothing handled.
func report(process windows.Handle, pid, tid uint32, thread windows.Handle, record []byte, modules map[uintptr]module, exe string) {
	dir := os.Getenv("PYXFORGE_CRASHDUMP_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	_ = os.MkdirAll(dir, 0o755)
	name := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	base := filepath.Join(dir, fmt.Sprintf("%s-%d-%s", name, pid, time.Now().Format("20060102-150405")))

	// CONTEXT: 1232 bytes, 16-byte aligned; ContextFlags at 0x30.
	raw := make([]byte, 1232+16)
	off := (16 - int(uintptr(unsafe.Pointer(&raw[0]))&15)) & 15
	ctx := raw[off : off+1232]
	binary.LittleEndian.PutUint32(ctx[0x30:], contextFull)
	haveContext := false
	if thread != 0 {
		r, _, _ := getThreadContext.Call(uintptr(thread), uintptr(unsafe.Pointer(&ctx[0])))
		haveContext = r != 0
	}

	var rec [19]uint64 // EXCEPTION_RECORD, 8-byte aligned, in this process (ClientPointers = FALSE)
	copy((*[152]byte)(unsafe.Pointer(&rec[0]))[:], record)
	pointers := [2]uintptr{uintptr(unsafe.Pointer(&rec[0])), 0}
	if haveContext {
		pointers[1] = uintptr(unsafe.Pointer(&ctx[0]))
	}
	var info [16]byte // MINIDUMP_EXCEPTION_INFORMATION is packed: ThreadId, ExceptionPointers, ClientPointers
	binary.LittleEndian.PutUint32(info[0:], tid)
	binary.LittleEndian.PutUint64(info[4:], uint64(uintptr(unsafe.Pointer(&pointers[0]))))
	dumpErr := "written"
	if f, err := os.Create(base + ".dmp"); err != nil {
		dumpErr = err.Error()
	} else {
		if r, _, e := miniDumpWriteDump.Call(uintptr(process), uintptr(pid), f.Fd(), dumpType, uintptr(unsafe.Pointer(&info[0])), 0, 0); r == 0 {
			dumpErr = "MiniDumpWriteDump: " + e.Error()
		}
		_ = f.Close()
	}
	runtime.KeepAlive(rec)
	runtime.KeepAlive(raw)

	var w strings.Builder
	code := binary.LittleEndian.Uint32(record[0:])
	addr := uintptr(binary.LittleEndian.Uint64(record[16:]))
	at, _ := where(modules, addr)
	fmt.Fprintf(&w, "%s (pid %d) died of exception 0x%08X at %s, thread %d\n", exe, pid, code, at, tid)
	wd, _ := os.Getwd()
	fmt.Fprintf(&w, "command line: %s\nfolder: %s\n", windows.ComposeCommandLine(os.Args[1:]), wd)
	if n := binary.LittleEndian.Uint32(record[24:]); code == 0xC0000005 && n >= 2 {
		kinds := map[uint64]string{0: "reading", 1: "writing", 8: "executing"}
		fmt.Fprintf(&w, "access violation %s 0x%x\n", kinds[binary.LittleEndian.Uint64(record[32:])], binary.LittleEndian.Uint64(record[40:]))
	}
	fmt.Fprintf(&w, "minidump %s.dmp: %s\n", base, dumpErr)
	if haveContext {
		regs := []struct {
			name string
			off  int
		}{{"rip", 0xF8}, {"rsp", 0x98}, {"rbp", 0xA0}, {"rax", 0x78}, {"rbx", 0x90}, {"rcx", 0x80}, {"rdx", 0x88},
			{"rsi", 0xA8}, {"rdi", 0xB0}, {"r8", 0xB8}, {"r9", 0xC0}, {"r10", 0xC8}, {"r11", 0xD0},
			{"r12", 0xD8}, {"r13", 0xE0}, {"r14", 0xE8}, {"r15", 0xF0}}
		for _, r := range regs {
			v := uintptr(binary.LittleEndian.Uint64(ctx[r.off:]))
			name, _ := where(modules, v)
			fmt.Fprintf(&w, "  %-3s 0x%016x  %s\n", r.name, v, name)
		}
		// Return addresses on the stack: every word from rsp up that points into a module.
		rsp := uintptr(binary.LittleEndian.Uint64(ctx[0x98:]))
		stack := make([]byte, 32*1024)
		var n uintptr
		_ = windows.ReadProcessMemory(process, rsp, &stack[0], uintptr(len(stack)), &n)
		fmt.Fprintf(&w, "stack from rsp, words that point into a module (%d bytes read):\n", n)
		for i := uintptr(0); i+8 <= n; i += 8 {
			v := uintptr(binary.LittleEndian.Uint64(stack[i:]))
			if name, ok := where(modules, v); ok {
				fmt.Fprintf(&w, "  rsp+0x%04x  %s\n", i, name)
			}
		}
	}
	list := make([]module, 0, len(modules))
	for _, m := range modules {
		list = append(list, m)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].base < list[j].base })
	fmt.Fprintln(&w, "modules:")
	for _, m := range list {
		fmt.Fprintf(&w, "  0x%016x %8x %s\n", m.base, m.size, m.path)
	}
	_ = os.WriteFile(base+".txt", []byte(w.String()), 0o644)
	fmt.Fprint(os.Stderr, "crashdump: "+w.String())
}
