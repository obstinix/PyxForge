# PyxForge shell integration for fish (D24), loaded with --init-command after your config.
# It marks prompts and commands (OSC 133) and the folder (OSC 7). Your files are not changed.
function __pyx_prompt_start --on-event fish_prompt
    set -l s $status
    if set -q __pyx_ran
        printf '\e]133;D;%s\a' $s
        set -e __pyx_ran
    end
    set -l p (string replace -a '%' '%25' -- $PWD | string replace -a ' ' '%20' | string replace -a '#' '%23' | string replace -a '?' '%3F')
    printf '\e]7;file://%s%s\a' (hostname) $p
    printf '\e]133;A\a'
    # fish_prompt is loaded lazily, after this script runs: mark its end once it exists.
    if functions -q fish_prompt; and not functions -q __pyx_user_prompt
        functions -c fish_prompt __pyx_user_prompt
        function fish_prompt
            __pyx_user_prompt
            printf '\e]133;B\a'
        end
    end
end
function __pyx_preexec --on-event fish_preexec
    set -g __pyx_ran 1
    printf '\e]133;C\a'
end
