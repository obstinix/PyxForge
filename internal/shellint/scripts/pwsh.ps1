# PyxForge shell integration for PowerShell 7 and Windows PowerShell (D24), loaded with
# -NoExit -Command after your profile. It marks prompts and commands (OSC 133) and the folder
# (OSC 7) so the Terminal panel can show command blocks. Your files are not changed.
$global:__PyxLastHistoryId = -1
$global:__PyxUserPrompt = $function:prompt

function global:prompt {
    $ok = $?
    $code = $global:LASTEXITCODE
    $e = [char]27; $a = [char]7
    $st = if ($ok) { 0 } elseif ($code) { $code } else { 1 }
    $out = ''
    $last = Get-History -Count 1
    if ($global:__PyxLastHistoryId -ne -1) {
        if ($null -eq $last -or $last.Id -eq $global:__PyxLastHistoryId) {
            $out += "$e]133;D$a"            # Enter at an empty prompt: nothing ran
        } else {
            $out += "$e]133;D;$st$a"
        }
    }
    if ($null -ne $last) { $global:__PyxLastHistoryId = $last.Id } else { $global:__PyxLastHistoryId = 0 }
    $loc = $executionContext.SessionState.Path.CurrentLocation
    if ($loc.Provider.Name -eq 'FileSystem') {
        $p = $loc.ProviderPath.Replace([string][char]92, '/').Replace('%', '%25').Replace(' ', '%20').Replace('#', '%23').Replace('?', '%3F')
        if (-not $p.StartsWith('/')) { $p = '/' + $p }
        $out += "$e]7;file://$env:COMPUTERNAME$p$a"
    }
    $user = & $global:__PyxUserPrompt
    $global:LASTEXITCODE = $code
    "$out$e]133;A$a$user$e]133;B$a"
}

if (Get-Command PSConsoleHostReadLine -ErrorAction SilentlyContinue) {
    $global:__PyxReadLine = $function:PSConsoleHostReadLine
    function global:PSConsoleHostReadLine {
        $line = & $global:__PyxReadLine
        [Console]::Write("$([char]27)]133;C$([char]7)")
        $line
    }
}
