package toolchain

import (
	"strings"

	"golang.org/x/sys/windows"
)

// readAppExecLink reads the app execution link at path, if it is one.
func readAppExecLink(path string) (appExecLink, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return appExecLink{}, false
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return appExecLink{}, false
	}
	defer windows.CloseHandle(h)
	buf := make([]byte, windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	var n uint32
	if err := windows.DeviceIoControl(h, windows.FSCTL_GET_REPARSE_POINT, nil, 0, &buf[0], uint32(len(buf)), &n, nil); err != nil {
		return appExecLink{}, false // not a reparse point
	}
	return parseAppExecLink(buf[:n])
}

// storeStub reports a program on PATH that is App Installer's alias offering to install it
// from the Microsoft Store, not the program.
func storeStub(path string) bool {
	if !strings.Contains(strings.ToLower(path), `\microsoft\windowsapps\`) {
		return false // aliases live only there; skip the system call for everything else
	}
	l, ok := readAppExecLink(path)
	return ok && l.installerStub()
}
