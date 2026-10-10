//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	freeConsole           = kernel32.NewProc("FreeConsole")
)

// releaseOwnConsole closes the console window Windows opened for PyxForge alone. PyxForge is a
// console program so its command line works in terminals; started from Explorer, it gets a
// console window of its own beside the desktop app. A console shared with a terminal (another
// process is attached) is left as it is.
func releaseOwnConsole() bool {
	var pids [2]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n != 1 {
		return false
	}
	r, _, _ := freeConsole.Call()
	return r != 0
}
