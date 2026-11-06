# Checks a PyxForge Windows release on a clean machine: run inside a fresh Windows Server Core
# container (release-windows.yml), which has no Go, Git, Neovim, compiler, Python, Node or QEMU.
# It installs PyxForge from its installer as a user would, checks that what PyxForge needs comes
# with it (Neovim and Git, bundled) and that Doctor names what it doesn't (with how to get it),
# uninstalls it, then checks the portable archive and the programs on their own.
#
#   powershell -File clean-machine.ps1 -Version 3.0.0-rc.3 [-Files C:\files]
#
# Server Core has no desktop, so the window itself isn't opened here: pyxforge.com, the console
# build of the same program, is.
param(
    [Parameter(Mandatory)][string]$Version,
    [string]$Files = 'C:\files'
)
$ErrorActionPreference = 'Stop'

function Fail([string]$message) {
    Write-Host "::error title=Clean machine::$message"
    exit 1
}
function Check($ok, [string]$what) {
    if ($ok -isnot [bool]) { Fail "the check '$what' got $($ok.GetType().Name), not true or false: $(@($ok) -join ' | ')" }
    if (-not $ok) { Fail $what }
    Write-Host "ok   $what"
}
# What a program printed on standard output, a line per item. The version check wants exactly
# one line: anything a clean machine adds there would also break the JSON commands.
function Lines([string]$exe, [string[]]$arguments) {
    $out = @(& $exe @arguments | ForEach-Object { [string]$_ })
    if ($out.Count -ne 1) { Write-Host "$exe $arguments printed $($out.Count) lines:"; $out | ForEach-Object { Write-Host "  | $_" } }
    return ,$out
}
function VersionIs([string]$exe, [string]$what) {
    $out = Lines $exe 'version'
    Check ($out.Count -eq 1 -and $out[0] -like "pyxforge $Version *") "${what}: $($out -join ' | ')"
}
function Doctor([string]$com) {
    $out = @(& $com doctor --json)
    if ($LASTEXITCODE -gt 1) { Fail "$com doctor --json exited with $LASTEXITCODE" } # 1 = something required is missing
    try { return (($out -join "`n") | ConvertFrom-Json).tools }
    catch { Fail "$com doctor --json printed what isn't JSON: $($out -join ' | ')" }
}

Write-Host "Windows: $((Get-CimInstance Win32_OperatingSystem).Caption) $([Environment]::OSVersion.Version)"

# 1. A clean machine: none of the tools PyxForge drives.
foreach ($tool in 'go', 'git', 'nvim', 'gcc', 'gdb', 'nasm', 'qemu-system-x86_64', 'python', 'node') {
    Check (-not (Get-Command $tool -ErrorAction SilentlyContinue)) "no $tool on PATH"
}

# 2. The installer, silent and per user, as a user's double-click without the pages.
$dir = 'C:\PyxForge'
$setup = Start-Process "$Files\PyxForge-Setup-$Version.exe" -PassThru -Wait `
    -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/CURRENTUSER', "/DIR=`"$dir`""
Check ($setup.ExitCode -eq 0) "the installer exited with 0 (got $($setup.ExitCode))"
$com = "$dir\pyxforge.com"
foreach ($f in 'PyxForge.exe', 'pyxforge.com', 'nvim\bin\nvim.exe', 'git\cmd\git.exe', 'THIRD-PARTY-NOTICES.txt', 'unins000.exe') {
    Check (Test-Path "$dir\$f") "installed $f"
}
VersionIs $com 'pyxforge.com version'

# 3. Doctor: Neovim and Git from the installation itself; the rest missing, each with a way to
#    get it (PyxForge's toolchain manager for GCC, GDB, NASM and QEMU).
$tools = Doctor $com
foreach ($id in 'nvim', 'git') {
    $t = $tools | Where-Object id -eq $id
    Check ($t.found -and $t.source -eq 'bundled') "Doctor: $($t.label) found, bundled: $($t.path) $($t.version)"
}
foreach ($id in 'cc', 'gdb', 'qemu-x86_64') {
    $t = $tools | Where-Object id -eq $id
    if ($t) { Check ((-not $t.found) -and $t.hint) "Doctor: $($t.label) missing, with a hint: $($t.hint)" }
}

# 4. The bundled Neovim, PyxForge's editor engine, runs.
$nv = & "$dir\nvim\bin\nvim.exe" --headless -u NONE -i NONE "+lua io.write(vim.version().major .. '.' .. vim.version().minor .. '.' .. vim.version().patch)" +qa 2>&1 | Out-String
Check ($nv.Trim() -match '^\d+\.\d+\.\d+$') "the bundled Neovim runs: $($nv.Trim())"

# 5. The bundled Git makes a repository and a commit.
$git = "$dir\git\cmd\git.exe"
$repo = 'C:\work\hello'
New-Item -ItemType Directory -Force $repo | Out-Null
Set-Content "$repo\boot.asm" 'org 0x7c00'
& $git -C $repo init -q
& $git -C $repo add boot.asm
& $git -C $repo -c user.name=PyxForge -c user.email=check@example.invalid commit -q -m "First commit"
Check ($LASTEXITCODE -eq 0) "the bundled Git committed: $(& $git -C $repo log --oneline)"

# 6. The command line works on a project: info, and the toolchains it can install.
$info = & $com info $repo --json | Out-String | ConvertFrom-Json
Check ($null -ne $info) "pyxforge info reads the project"
$list = & $com toolchains list | Out-String
Check ($list -match 'QEMU' -and $list -match 'GCC') "pyxforge toolchains list offers QEMU and GCC"

# 7. Uninstall: the programs go; the user's projects stay.
$un = Start-Process "$dir\unins000.exe" -PassThru -Wait -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART'
Check ($un.ExitCode -eq 0) "the uninstaller exited with 0 (got $($un.ExitCode))"
for ($i = 0; $i -lt 30 -and (Test-Path "$dir\PyxForge.exe"); $i++) { Start-Sleep -Seconds 1 } # it finishes in a copy of itself
Check (-not (Test-Path "$dir\PyxForge.exe")) "uninstalled: PyxForge.exe is gone"
Check (Test-Path "$repo\boot.asm") "the user's project is still there"

# 8. The portable archive, unpacked anywhere, works the same way.
Expand-Archive "$Files\PyxForge-Portable-$Version.zip" C:\portable
$pcom = (Get-ChildItem C:\portable -Recurse -Filter pyxforge.com | Select-Object -First 1).FullName
Check ($null -ne $pcom) "the archive holds pyxforge.com"
VersionIs $pcom 'portable pyxforge.com version'
$t = (Doctor $pcom) | Where-Object id -eq 'nvim'
Check ($t.found -and $t.source -eq 'bundled') "portable: Neovim found, bundled: $($t.path)"

# 9. The program on its own runs, and says Neovim is missing rather than failing.
$scom = "$Files\pyxforge-$Version.com"
VersionIs $scom "pyxforge-$Version.com on its own"
$t = (Doctor $scom) | Where-Object id -eq 'nvim'
Check ((-not $t.found) -and $t.hint) "on its own: Doctor says Neovim is missing, with a hint: $($t.hint)"

Write-Host "PyxForge $Version installs, runs and uninstalls on a clean Windows."
exit 0
