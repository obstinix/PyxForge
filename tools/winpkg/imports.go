package main

import (
	"debug/pe"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// What a program needs to start on a Windows that has nothing installed.
//
// A release once shipped a Neovim that could not start on a clean machine: nvim.exe imports
// VCRUNTIME140.dll, Microsoft's Visual C++ runtime, which is not part of Windows; a PC has it
// only if some other program installed it. The build machine and the hosted runner both did,
// so every check passed, and Windows answered 0xC0000135 (DLL not found) in a clean container.
// checkImports reads the import table of every program and library in the staged folder and
// fails on any DLL that is neither beside the file that asks for it nor part of every desktop
// Windows 10 and 11.

// systemDLLs are in System32 on every desktop Windows 10 and 11 (lower case). The API sets
// (api-ms-win-*, ext-ms-*) are resolved by the loader itself; the Universal C Runtime
// (ucrtbase, the api-ms-win-crt-* sets) has been part of Windows since Windows 10.
var systemDLLs = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`kernel32 kernelbase ntdll user32 gdi32 gdi32full advapi32 shell32 ole32
		oleaut32 ws2_32 wsock32 iphlpapi netapi32 userenv dbghelp psapi version winmm comdlg32 comctl32
		shlwapi crypt32 bcrypt bcryptprimitives ncrypt secur32 rpcrt4 imm32 opengl32 glu32 dwmapi uxtheme
		setupapi cfgmgr32 winhttp wininet urlmon mswsock dnsapi powrprof sspicli wtsapi32 msvcrt ucrtbase
		winspool.drv msimg32 dwrite d3d11 dxgi d2d1 shcore propsys authz wldap32 normaliz cryptbase
		profapi mpr iertutil pdh credui wintrust imagehlp dbgcore usp10 mscoree oleacc hid gdiplus fontsub
		win32u combase sechost cryptsp samcli netutils wkscli srvcli logoncli winnsi nsi fwpuclnt
		ntmarta msasn1 apphelp kernel.appcore coremessaging avrt uiautomationcore xinput1_4 dinput8`) {
		systemDLLs[n] = true
	}
}

// partOfWindows reports whether the loader finds dll on any desktop Windows.
func partOfWindows(dll string) bool {
	n := strings.ToLower(dll)
	if strings.HasPrefix(n, "api-ms-win-") || strings.HasPrefix(n, "ext-ms-") {
		return true
	}
	return systemDLLs[n] || systemDLLs[strings.TrimSuffix(n, ".dll")]
}

// missingImports lists, for every program and library under root, the DLLs it imports that
// the loader would not find on a clean Windows: "nvim/bin/nvim.exe needs VCRUNTIME140.dll".
// A file finds a DLL in its own folder, in Windows, or beside any program of the same bundled
// product (the first folder under root: nvim, git): a Tree-sitter parser under nvim/lib is
// loaded by nvim/bin/nvim.exe and served by what is beside it, and Git starts its helpers
// with its own bin folders first on PATH.
func missingImports(root string) ([]string, error) {
	type file struct {
		rel     string
		program bool
		libs    []string
	}
	var files []file
	programDirs := map[string][]string{} // product → folders holding its programs
	product := func(rel string) string {
		first, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
		return first
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".exe" && ext != ".dll" && ext != ".com" {
			return nil
		}
		f, err := pe.Open(path)
		if err != nil {
			return nil // not a Windows program (a script named .com, a data file)
		}
		// debug/pe's ImportedLibraries returns nothing for any file; the symbols, as
		// "name:library", do name the libraries.
		syms, err := f.ImportedSymbols()
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		rel, _ := filepath.Rel(root, path)
		fl := file{rel: rel, program: ext != ".dll"}
		seen := map[string]bool{}
		for _, s := range syms {
			if i := strings.LastIndexByte(s, ':'); i >= 0 && !seen[strings.ToLower(s[i+1:])] {
				seen[strings.ToLower(s[i+1:])] = true
				fl.libs = append(fl.libs, s[i+1:])
			}
		}
		if fl.program {
			programDirs[product(rel)] = append(programDirs[product(rel)], filepath.Dir(path))
		}
		files = append(files, fl)
		return nil
	})
	if err != nil {
		return nil, err
	}
	exists := func(dir, lib string) bool {
		_, err := os.Stat(filepath.Join(dir, lib))
		return err == nil
	}
	var out []string
	for _, f := range files {
		own := filepath.Dir(filepath.Join(root, f.rel))
		for _, lib := range f.libs {
			if partOfWindows(lib) || exists(own, lib) {
				continue
			}
			found := false
			for _, dir := range programDirs[product(f.rel)] {
				if found = exists(dir, lib); found {
					break
				}
			}
			if !found {
				out = append(out, filepath.ToSlash(f.rel)+" needs "+lib)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// checkImports fails when a staged program could not start on a clean Windows.
func checkImports(stage string) error {
	missing, err := missingImports(stage)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("these programs would not start on a clean Windows (0xC0000135): a DLL they load is neither beside them nor part of Windows:\n  %s",
			strings.Join(missing, "\n  "))
	}
	return nil
}

// vcRuntime is Microsoft's Visual C++ runtime as Visual Studio provides it for shipping beside
// a program ("app-local deployment"): the x64 Microsoft.VC143.CRT folder of the newest Visual
// Studio or Build Tools on the build machine, found with vswhere.
type vcRuntime struct {
	Dir     string // …\VC\Redist\MSVC\<version>\x64\Microsoft.VC143.CRT
	Version string // the Redist folder's version, such as 14.44.35112
}

func findVCRuntime() (vcRuntime, error) {
	vswhere := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
	out, err := exec.Command(vswhere, "-latest", "-products", "*", "-property", "installationPath").Output()
	install := strings.TrimSpace(string(out))
	if err != nil || install == "" {
		return vcRuntime{}, fmt.Errorf("the Visual C++ runtime Neovim loads comes from Visual Studio or its Build Tools, and neither was found (vswhere: %v)", err)
	}
	dirs, _ := filepath.Glob(filepath.Join(install, "VC", "Redist", "MSVC", "*", "x64", "Microsoft.VC*.CRT"))
	sort.Strings(dirs)
	for i := len(dirs) - 1; i >= 0; i-- { // the newest version first
		if _, err := os.Stat(filepath.Join(dirs[i], "vcruntime140.dll")); err == nil {
			version := filepath.Base(filepath.Dir(filepath.Dir(dirs[i])))
			return vcRuntime{Dir: dirs[i], Version: version}, nil
		}
	}
	return vcRuntime{}, fmt.Errorf("no Visual C++ runtime under %s\\VC\\Redist\\MSVC", install)
}

// bundleVCRuntime copies, from the Visual C++ runtime, each DLL a staged program asks for and
// lacks, into that program's own folder, and returns what it copied (relative to stage).
// Only what is asked for is copied: Neovim needs vcruntime140.dll today.
func bundleVCRuntime(stage string, rt vcRuntime) ([]string, error) {
	missing, err := missingImports(stage)
	if err != nil {
		return nil, err
	}
	var copied []string
	done := map[string]bool{}
	for _, m := range missing {
		rel, dll, _ := strings.Cut(m, " needs ")
		if strings.EqualFold(filepath.Ext(rel), ".dll") {
			continue // a library: served by what goes beside its product's programs
		}
		src := filepath.Join(rt.Dir, strings.ToLower(dll))
		dst := filepath.Join(stage, filepath.Dir(rel), strings.ToLower(dll))
		if done[dst] {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			continue // not a runtime DLL: checkImports reports it
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, err
		}
		done[dst] = true
		r, _ := filepath.Rel(stage, dst)
		copied = append(copied, filepath.ToSlash(r))
	}
	sort.Strings(copied)
	return copied, nil
}
