#!/usr/bin/env bash
# Installs a fixed MinGW-w64 toolchain (WinLibs GCC, UCRT, POSIX threads) on a Windows runner
# and puts it first on PATH for the following steps. The archive is pinned by version and
# SHA-256, so the compiler that builds a release is the one recorded here, not whatever the
# runner image or Chocolatey's feed has that day (#35). The same build includes GDB, so the
# debugger tests run on Windows too (#38).
#
#   install-winlibs.sh            # installs and prints gcc and gdb versions
#
# The release workflow records `gcc --version` with the release (tools/sbom).
set -euo pipefail

# The release internal/managed offers as "GCC toolchain (WinLibs)"; this is its .7z archive.
release=16.2.0posix-23.1.3-14.0.0-ucrt-r3
file=winlibs-x86_64-posix-seh-gcc-16.2.0-mingw-w64ucrt-14.0.0-r3.7z
sha256=db014a73b71907c6c778fd1c938056cddcae9023f89f8dfff7783d9e45baf975

dir="${RUNNER_TEMP:?}/winlibs"
archive="$RUNNER_TEMP/$file"
curl -fsSL --retry 5 --retry-all-errors -o "$archive" \
	"https://github.com/brechtsanders/winlibs_mingw/releases/download/$release/$file"
echo "$sha256  $archive" | sha256sum -c -
7z x -y -bso0 -bsp0 -o"$dir" "$archive"
rm -f "$archive"
bin="$dir/mingw64/bin"
[ -x "$bin/gcc.exe" ] || { echo "::error::gcc.exe is not in $bin"; exit 1; }
# GITHUB_PATH entries come before the image's own MinGW (C:\mingw64\bin) in later steps.
cygpath -w "$bin" >> "${GITHUB_PATH:?}"
"$bin/gcc.exe" --version | head -1
"$bin/gdb.exe" --version | head -1
