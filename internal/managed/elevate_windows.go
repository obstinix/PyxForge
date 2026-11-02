package managed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shellExecuteInfo is SHELLEXECUTEINFOW.
type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	hwnd       uintptr
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instApp    uintptr
	idList     uintptr
	class      *uint16
	keyClass   uintptr
	hotKey     uint32
	icon       uintptr
	process    windows.Handle
}

// runElevated starts exe with administrator rights, through Windows' consent prompt, and waits
// for it to end.
func runElevated(ctx context.Context, exe string, args []string) error {
	const (
		maskNoCloseProcess = 0x40
		maskNoAsync        = 0x100
		showNormal         = 1
		errorCancelled     = 1223
	)
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	params, err := windows.UTF16PtrFromString(joinArgs(args))
	if err != nil {
		return err
	}
	info := shellExecuteInfo{mask: maskNoCloseProcess | maskNoAsync, verb: verb, file: file, parameters: params, show: showNormal}
	info.size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errors.Is(callErr, windows.Errno(errorCancelled)) {
			return ErrDeclined
		}
		return fmt.Errorf("could not start it: %w", callErr)
	}
	defer windows.CloseHandle(info.process)
	for {
		ev, err := windows.WaitForSingleObject(info.process, 200)
		if err != nil {
			return err
		}
		if ev == windows.WAIT_OBJECT_0 {
			break
		}
		if ctx.Err() != nil {
			return fmt.Errorf("stopped waiting for it (it may still be running): %w", ctx.Err())
		}
	}
	var code uint32
	if err := windows.GetExitCodeProcess(info.process, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("it exited with code %d", code)
	}
	return nil
}

// joinArgs quotes arguments that hold spaces, except an NSIS /D=, which must stay unquoted.
func joinArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t") && !strings.HasPrefix(a, "/D=") {
			a = `"` + a + `"`
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}
