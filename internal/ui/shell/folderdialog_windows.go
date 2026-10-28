package shell

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/obstinix/PyxForge/internal/ui/notifications"
	"golang.org/x/sys/windows"
)

// Windows' native folder picker runs on its own STA thread. Only an accepted filesystem
// path reaches the workspace; cancelling it leaves the shell untouched.
func (s *Shell) pickFolder(title string, selected func(string)) {
	go func() {
		path, err := nativeFolder(title)
		s.dispatch(func() {
			if err != nil {
				s.Notify(notifications.Error, title, err.Error())
				return
			}
			if path != "" {
				selected(path)
			}
		})
	}()
}

func nativeFolder(title string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := windows.NewLazySystemDLL("ole32.dll")
	hr, _, _ := ole.NewProc("OleInitialize").Call(0)
	if int32(hr) < 0 {
		return "", fmt.Errorf("initialize folder dialog: HRESULT %#x", hr)
	}
	defer ole.NewProc("OleUninitialize").Call()
	shell := windows.NewLazySystemDLL("shell32.dll")
	t, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return "", err
	}
	var display [windows.MAX_PATH]uint16
	info := struct {
		owner, root     uintptr
		display, title  *uint16
		flags           uint32
		callback, param uintptr
		image           int32
	}{display: &display[0], title: t, flags: 0x41} // BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE
	pidl, _, _ := shell.NewProc("SHBrowseForFolderW").Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return "", nil
	}
	defer ole.NewProc("CoTaskMemFree").Call(pidl)
	var path [32768]uint16
	ok, _, _ := shell.NewProc("SHGetPathFromIDListEx").Call(pidl, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)), 0)
	if ok == 0 {
		return "", fmt.Errorf("the selection is not a filesystem folder")
	}
	return windows.UTF16ToString(path[:]), nil
}
