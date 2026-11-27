# PyxForge shell integration for bash (D24). PyxForge starts bash with --rcfile pointing here;
# it loads your ~/.bashrc as bash would, then marks prompts and commands (OSC 133) and the
# folder (OSC 7) so the Terminal panel can show command blocks. Your files are not changed.
if [ -f ~/.bashrc ]; then . ~/.bashrc; fi

__pyx_osc7() {
	local p="$PWD"
	if command -v cygpath >/dev/null 2>&1; then p="/$(cygpath -m "$PWD")"; fi
	p="${p//%/%25}"; p="${p// /%20}"; p="${p//#/%23}"; p="${p//\?/%3F}"
	printf '\e]7;file://%s%s\a' "${HOSTNAME:-localhost}" "$p"
}
__pyx_prompt() {
	local s=$?
	# Every prompt but the first ends the command before it; after an Enter at an empty prompt
	# no command started, and the panel ignores the end.
	if [ -n "$__pyx_started" ]; then printf '\e]133;D;%s\a' "$s"; fi
	__pyx_started=1
	__pyx_osc7
	case "$PS1" in *'133;A'*) ;; *) PS1='\[\e]133;A\a\]'"$PS1"'\[\e]133;B\a\]' ;; esac
	__pyx_armed=1
	return $s
}
if [ "${BASH_VERSINFO[0]}" -gt 4 ] || { [ "${BASH_VERSINFO[0]}" -eq 4 ] && [ "${BASH_VERSINFO[1]}" -ge 4 ]; }; then
	# PS0 shows once a command is read, before it runs, whatever the command is.
	PS0="${PS0}"'\e]133;C\a'
else
	# Older bash (macOS's 3.2): the DEBUG trap, which misses a command that starts with a subshell.
	__pyx_preexec() {
		[ -n "$__pyx_armed" ] || return
		case "$BASH_COMMAND" in __pyx_prompt*) return ;; esac
		[ -n "$COMP_LINE" ] && return
		__pyx_armed=
		printf '\e]133;C\a'
	}
	trap '__pyx_preexec' DEBUG
fi
PROMPT_COMMAND="__pyx_prompt${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
