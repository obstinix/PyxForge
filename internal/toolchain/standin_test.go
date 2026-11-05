package toolchain

import (
	"context"
	"encoding/binary"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

// appExecLinkData builds an IO_REPARSE_TAG_APPEXECLINK reparse buffer as Windows stores it.
func appExecLinkData(strs ...string) []byte {
	data := binary.LittleEndian.AppendUint32(nil, 3) // version
	for _, s := range strs {
		for _, u := range utf16.Encode([]rune(s)) {
			data = binary.LittleEndian.AppendUint16(data, u)
		}
		data = binary.LittleEndian.AppendUint16(data, 0)
	}
	b := binary.LittleEndian.AppendUint32(nil, reparseTagAppExecLink)
	b = binary.LittleEndian.AppendUint16(b, uint16(len(data)))
	b = binary.LittleEndian.AppendUint16(b, 0)
	return append(b, data...)
}

// The two kinds of python3.exe alias, as App Installer and the Store's Python write them.
var (
	installerAlias = appExecLinkData("Microsoft.DesktopAppInstaller_8wekyb3d8bbwe",
		"Microsoft.DesktopAppInstaller_8wekyb3d8bbwe!PythonRedirector",
		`C:\Program Files\WindowsApps\Microsoft.DesktopAppInstaller_1.29.379.0_x64__8wekyb3d8bbwe\AppInstallerPythonRedirector.exe`, "0")
	storePython = appExecLinkData("PythonSoftwareFoundation.Python.3.13_qbz5n2kfra8p0",
		"PythonSoftwareFoundation.Python.3.13_qbz5n2kfra8p0!Python",
		`C:\Program Files\WindowsApps\PythonSoftwareFoundation.Python.3.13_3.13.2032.0_x64__qbz5n2kfra8p0\python3.13.exe`, "0")
)

func TestParseAppExecLink(t *testing.T) {
	l, ok := parseAppExecLink(installerAlias)
	// The target is a Windows path on every system the test runs on: filepath.Base splits it
	// only on Windows.
	if !ok || l.AppID != "Microsoft.DesktopAppInstaller_8wekyb3d8bbwe!PythonRedirector" || !strings.HasSuffix(l.Target, `\AppInstallerPythonRedirector.exe`) {
		t.Fatalf("App Installer's alias: %+v, %v", l, ok)
	}
	if !l.installerStub() {
		t.Error("App Installer's python3 alias is not a stand-in")
	}
	if l, ok := parseAppExecLink(storePython); !ok || l.installerStub() {
		t.Errorf("the Store's Python counted as a stand-in: %+v, %v", l, ok)
	}
	winget := appExecLinkData("Microsoft.DesktopAppInstaller_8wekyb3d8bbwe", "Microsoft.DesktopAppInstaller_8wekyb3d8bbwe!winget",
		`C:\Program Files\WindowsApps\Microsoft.DesktopAppInstaller_1.29.379.0_x64__8wekyb3d8bbwe\winget.exe`, "0")
	if l, ok := parseAppExecLink(winget); !ok || l.installerStub() {
		t.Errorf("App Installer's winget counted as a stand-in: %+v, %v", l, ok)
	}
	for name, b := range map[string][]byte{
		"empty":       nil,
		"another tag": append([]byte{0x0c, 0, 0, 0xa0}, installerAlias[4:]...), // a symbolic link
		"truncated":   installerAlias[:len(installerAlias)-6],
		"no data":     installerAlias[:8],
		"two strings": appExecLinkData("Microsoft.DesktopAppInstaller_8wekyb3d8bbwe", "x"),
	} {
		if _, ok := parseAppExecLink(b); ok {
			t.Errorf("%s: parsed", name)
		}
	}
}

// TestStoreStandInIsNotPython: with only App Installer's python3 alias on PATH, Python is
// missing, not found (#25); a real Python later in the candidate list is still found.
func TestStoreStandInIsNotPython(t *testing.T) {
	stub := filepath.Join("C:", "Users", "u", "AppData", "Local", "Microsoft", "WindowsApps", "python3.exe")
	python := Tool{ID: "python", Candidates: []string{"py", "python", "python3"}, VersionArgs: []string{"--version"}}
	d := fake(map[string]string{stub: "Python was not found; run without arguments to install from the Microsoft Store"})
	d.LookPath = func(name string) (string, error) {
		if name == "python3" {
			return stub, nil
		}
		return "", exec.ErrNotFound
	}
	d.StandIn = func(p string) bool { return p == stub }
	if st := d.Detect(context.Background(), []Tool{python})[0]; st.Found() {
		t.Errorf("the Store's stand-in was reported as Python: %+v", st)
	}

	d.LookPath = func(name string) (string, error) {
		switch name {
		case "python3":
			return stub, nil
		case "python":
			return "python", nil
		}
		return "", exec.ErrNotFound
	}
	d.Run = func(context.Context, string, []string) (string, error) { return "Python 3.13.2", nil }
	if st := d.Detect(context.Background(), []Tool{python})[0]; st.Path != "python" || st.Version != "3.13.2" {
		t.Errorf("a real python beside the stand-in: %+v", st)
	}

	// A path the user set is theirs, whatever it is.
	d.Overrides = func() Overrides { return Overrides{"python": stub} }
	d.LookPath = func(name string) (string, error) { return name, nil }
	if st := d.Detect(context.Background(), []Tool{python})[0]; st.Path != stub || st.Source != SourceConfigured {
		t.Errorf("the path set in Doctor was passed over: %+v", st)
	}
}
