// Package managed installs the toolchains PyxForge downloads for the user: version-pinned
// archives, checked against their SHA-256 before anything is unpacked, kept in PyxForge's own
// data folder (%LOCALAPPDATA%\PyxForge\toolchains on Windows), never in a project and never on
// the system PATH. Tool lookups (internal/toolchain) prefer what is installed here to what is on
// PATH, and PyxForge adds these folders, after the user's own, to the PATH of what it starts.
//
// It is not named runtime, after the standard library package every caller also imports.
package managed

import "runtime"

// Package is one archive PyxForge can install.
type Package struct {
	ID       string   // stable identifier, the folder it installs to
	Name     string   // shown to people
	Version  string   // the upstream version
	GOOS     string   // the platform it runs on
	GOARCH   string   //
	URL      string   // the official download
	SHA256   string   // of the archive, from the upstream release, checked after download
	Size     int64    // of the archive, in bytes, for the progress bar and the UI
	Bin      []string // folders in the install holding its programs, slash-separated
	Requires []string // files that must be there after unpacking: the install's own check
	Provides []string // toolchain tool IDs it satisfies
	About    string   // what it is for, for people deciding to install it
	License  string   // its license, for the notices
	Source   string   // where its source code is

	// An installer package is the vendor's own installer, which PyxForge downloads and checks
	// like an archive, then runs silently with the user's approval (QEMU's asks for
	// administrator rights). It installs where the vendor's does, not under PyxForge's folder,
	// so Bin and Requires are relative to InstallDir.
	Installer   []string // its silent-install arguments; nil for an archive
	InstallDir  string   // where it installs, with ${VAR}s from the environment
	Uninstaller string   // the program in InstallDir that removes it (run with /S)
}

// IsInstaller reports whether p is run as the vendor's installer rather than unpacked.
func (p Package) IsInstaller() bool { return p.Installer != nil }

// Packages lists what PyxForge can install, every platform's. A version changes only with a
// PyxForge release, so a release always installs what it was tested with.
var Packages = []Package{
	{
		ID: "mingw-toolchain", Name: "GCC toolchain (WinLibs)", Version: "16.2.0-r3",
		GOOS: "windows", GOARCH: "amd64",
		URL:      "https://github.com/brechtsanders/winlibs_mingw/releases/download/16.2.0posix-23.1.3-14.0.0-ucrt-r3/winlibs-x86_64-posix-seh-gcc-16.2.0-mingw-w64ucrt-14.0.0-r3.zip",
		SHA256:   "3c69305081e4a647144339a936244fcf7d3d9e558319e1ad7be2cd2886d4b6a1",
		Size:     276414382,
		Bin:      []string{"mingw64/bin"},
		Requires: []string{"mingw64/bin/gcc.exe", "mingw64/bin/g++.exe", "mingw64/bin/gdb.exe", "mingw64/bin/nasm.exe", "mingw64/bin/ld.exe", "mingw64/bin/objdump.exe", "mingw64/bin/mingw32-make.exe"},
		Provides: []string{"cc", "cxx", "ld", "nasm", "gdb", "objdump", "make"},
		About:    "GCC and G++ 16.2, GDB, NASM, GNU binutils and make for 64-bit Windows: builds, Run and Debug Current File, Debug in QEMU",
		License:  "GPL and LGPL (GCC, GDB, binutils, make), BSD (NASM), MinGW-w64's own; see the files under mingw64/licenses",
		Source:   "https://github.com/brechtsanders/winlibs_mingw",
	},
	{
		ID: "qemu", Name: "QEMU for Windows", Version: "2026-08-11 build",
		GOOS: "windows", GOARCH: "amd64",
		URL:         "https://qemu.weilnetz.de/w64/qemu-w64-setup-20260811.exe",
		SHA256:      "f98a8aeb5f7faea9765b6dee28316c266cd179d80354a2fed8e50176f9a2e59f", // upstream SHA-512 5bcf9eed…16bf543 checked when pinned
		Size:        206615928,
		Installer:   []string{"/S"},
		InstallDir:  `${ProgramFiles}\qemu`,
		Uninstaller: "qemu-uninstall.exe",
		Bin:         []string{"."},
		Requires:    []string{"qemu-system-x86_64.exe", "qemu-system-i386.exe", "qemu-img.exe"},
		Provides:    []string{"qemu-x86_64", "qemu-i386", "qemu-arm", "qemu-img"},
		About:       "Stefan Weil's QEMU build for Windows, the one qemu.org links to: Run and Debug in QEMU, machine snapshots. Its installer asks Windows for administrator approval once and installs to Program Files.",
		License:     "GPL-2.0 (QEMU), with the licenses of its libraries in the install folder",
		Source:      "https://qemu.weilnetz.de/w64/ (sources: https://gitlab.com/qemu-project/qemu)",
	},
	{
		ID: "git", Name: "Git (MinGit)", Version: "2.56.0.4",
		GOOS: "windows", GOARCH: "amd64",
		URL:      "https://github.com/git-for-windows/git/releases/download/v2.56.0.windows.4/MinGit-2.56.0.4-64-bit.zip",
		SHA256:   "e3bf78351fe777fba47ddee5950082f8a98bb0f02604fdc7ba3ee2cb68ffa877",
		Size:     39846926,
		Bin:      []string{"cmd"},
		Requires: []string{"cmd/git.exe", "ucrt64/bin/git.exe"},
		Provides: []string{"git"},
		About:    "Git for Windows' minimal distribution: the Git tab, cloning, and fetching editor plugins (the Windows installer already includes it)",
		License:  "GPL-2.0 (Git), with the licenses of its components in the distribution",
		Source:   "https://github.com/git-for-windows/git/tree/v2.56.0.windows.4",
	},
}

// ForPlatform lists the packages for this machine.
func ForPlatform() []Package { return forPlatform(runtime.GOOS, runtime.GOARCH) }

func forPlatform(goos, goarch string) []Package {
	var out []Package
	for _, p := range Packages {
		if p.GOOS == goos && p.GOARCH == goarch {
			out = append(out, p)
		}
	}
	return out
}

// Find returns this machine's package with id.
func Find(id string) (Package, bool) {
	for _, p := range ForPlatform() {
		if p.ID == id {
			return p, true
		}
	}
	return Package{}, false
}
