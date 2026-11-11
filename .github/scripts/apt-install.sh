#!/usr/bin/env bash
# Installs Ubuntu packages on a CI runner without hanging when a package mirror is down.
#
#   apt-install.sh [apt-get install options] package...
#
# On 11 November 2026 the runners' mirror (azure.archive.ubuntu.com) stopped resolving for
# hours: "apt-get install" waited until a job's 45-minute limit, or failed, and every CI run
# was red for a reason that had nothing to do with PyxForge. Each download now gives up after
# 30 seconds and is tried three times; if the runner's mirror still fails, the sources are
# pointed at Ubuntu's own archive and it is tried once more. A real failure is said in one line.
set -euo pipefail
opts=(-o Acquire::Retries=3 -o Acquire::http::Timeout=30 -o Acquire::https::Timeout=30 -o DPkg::Lock::Timeout=120)
try() {
	sudo timeout 600 apt-get "${opts[@]}" update -q &&
		sudo timeout 900 apt-get "${opts[@]}" install -y "$@"
}
if try "$@"; then
	exit 0
fi
echo "::warning title=Package mirror::the runner's Ubuntu mirror failed; trying archive.ubuntu.com"
sudo sed -i -E 's#https?://azure\.archive\.ubuntu\.com/ubuntu/?#http://archive.ubuntu.com/ubuntu/#g' \
	/etc/apt/sources.list /etc/apt/sources.list.d/* 2>/dev/null || true
if try "$@"; then
	exit 0
fi
echo "::error title=Package mirror::could not install $* from the runner's mirror or archive.ubuntu.com (a runner or network fault, not a test failure)"
exit 1
