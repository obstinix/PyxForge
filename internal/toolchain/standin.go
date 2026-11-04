package toolchain

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// Microsoft Store stand-ins (#25). Windows keeps App Execution Aliases in
// %LOCALAPPDATA%\Microsoft\WindowsApps. Most start an app installed from the Store (Python
// from the Store has real python.exe and python3.exe aliases there), but App Installer adds
// python.exe and python3.exe aliases that only offer to install Python: given a script, they
// print "Python was not found" and exit. An alias's reparse data names the app it starts,
// which tells the two apart without running anything.

// reparseTagAppExecLink is IO_REPARSE_TAG_APPEXECLINK.
const reparseTagAppExecLink = 0x8000001B

// appExecLink is what an App Execution Alias starts.
type appExecLink struct {
	Package string // package family name, such as Microsoft.DesktopAppInstaller_8wekyb3d8bbwe
	AppID   string // application user model ID, such as …!PythonRedirector
	Target  string // the executable inside the package
}

// parseAppExecLink decodes a REPARSE_DATA_BUFFER that holds an app execution link: the tag,
// the data length and a reserved word, then a version (3) and three NUL-terminated UTF-16
// strings.
func parseAppExecLink(b []byte) (appExecLink, bool) {
	if len(b) < 8 || binary.LittleEndian.Uint32(b) != reparseTagAppExecLink {
		return appExecLink{}, false
	}
	n := int(binary.LittleEndian.Uint16(b[4:]))
	if n < 4 || 8+n > len(b) {
		return appExecLink{}, false
	}
	rest := b[12 : 8+n] // after the version
	var strs [3]string
	for i := range strs {
		var u []uint16
		terminated := false
		for len(rest) >= 2 {
			c := binary.LittleEndian.Uint16(rest)
			rest = rest[2:]
			if c == 0 {
				terminated = true
				break
			}
			u = append(u, c)
		}
		if !terminated {
			return appExecLink{}, false
		}
		strs[i] = string(utf16.Decode(u))
	}
	return appExecLink{Package: strs[0], AppID: strs[1], Target: strs[2]}, true
}

// installerStub reports one of App Installer's redirector aliases (python.exe and python3.exe
// start AppInstallerPythonRedirector.exe), which offer to install a program from the Store
// instead of being it. App Installer's other aliases, such as winget.exe, are real programs.
func (l appExecLink) installerStub() bool {
	target := strings.ToLower(l.Target[strings.LastIndexAny(l.Target, `\/`)+1:])
	return strings.HasPrefix(strings.ToLower(l.Package), "microsoft.desktopappinstaller_") &&
		strings.Contains(target, "redirector")
}
