#!/usr/bin/env bash
# Runs a command, and if it fails, reports its last 60 lines of output as a GitHub Actions error
# annotation. Annotations are readable through the public API, unlike job logs, so a failure
# can be diagnosed without repository admin rights.
#
#   run-annotated.sh "<title>" <command> [args...]
set -uo pipefail

title=$1
shift
log=$(mktemp)
"$@" 2>&1 | tee "$log"
status=${PIPESTATUS[0]}
if [ "$status" -ne 0 ]; then
	# Escape for workflow commands: % first, then CR and LF.
	msg=$(tail -n 60 "$log" | sed -e 's/%/%25/g' -e 's/\r/%0D/g' | awk 'BEGIN { ORS = "%0A" } { print }')
	echo "::error title=${title}::${msg}"
fi
rm -f "$log"
exit "$status"
