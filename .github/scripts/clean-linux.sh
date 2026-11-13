#!/usr/bin/env bash
# Checks PyxForge's Debian package on a clean Ubuntu: run inside a fresh ubuntu:24.04 container
# (ci.yml), which has none of the libraries a desktop has. The package is installed with only
# what it declares (Depends, no Recommends), so a library it needs and doesn't name shows here,
# not on a user's machine. Then the window itself is opened under a virtual X server: the window
# toolkit loads X11 and OpenGL libraries at run time, which ldd can't see.
#
#   clean-linux.sh /dist/PyxForge-<version>-linux-x64.deb
set -euo pipefail
deb=$1
export DEBIAN_FRONTEND=noninteractive
apt_opts=(-o Acquire::Retries=3 -o Acquire::http::Timeout=30 -q)
ok() { echo "ok   $*"; }
fail() { echo "::error title=Clean Linux::$*"; exit 1; }

apt-get "${apt_opts[@]}" update >/dev/null
command -v nvim >/dev/null && fail "the image already has Neovim"

# 1. The package and nothing it doesn't declare.
apt-get "${apt_opts[@]}" install -y --no-install-recommends "$deb" >/dev/null || fail "apt could not install $deb"
ok "installed $(dpkg-query -W -f '${Package} ${Version}' pyxforge)"
missing=$(ldd /usr/bin/pyxforge | grep 'not found' || true)
[ -z "$missing" ] || fail "libraries the package doesn't declare: $missing"
ok "every library the program links is installed"
v=$(pyxforge version)
[[ $v == pyxforge\ * ]] || fail "pyxforge version said: $v"
ok "$v"

# 2. Doctor: Neovim is recommended, not required, so it is missing here, with a way to get it.
set +e
pyxforge doctor > /tmp/doctor.txt
code=$?
set -e
[ $code -le 1 ] || fail "doctor exited with $code: $(cat /tmp/doctor.txt)"
grep -E '^ +missing +Neovim' /tmp/doctor.txt >/dev/null || fail "doctor did not say Neovim is missing: $(cat /tmp/doctor.txt)"
grep -q 'broken' /tmp/doctor.txt && fail "doctor found a broken tool: $(cat /tmp/doctor.txt)"
ok "doctor runs and names what is missing"

# 3. The window, as a user would open it: an ordinary account with a home folder, in a D-Bus
#    session, on a virtual X server. It must start and stay up until stopped.
apt-get "${apt_opts[@]}" install -y --no-install-recommends xvfb xauth dbus >/dev/null
useradd -m dev
su dev -s /bin/bash -c 'mkdir -p ~/project && cd ~ && xvfb-run -a -s "-screen 0 1440x900x24" dbus-run-session -- timeout 15 pyxforge ~/project' > /tmp/window.log 2>&1 && code=0 || code=$?
cat /tmp/window.log
if [ $code -ne 124 ]; then # 124: still running when timeout stopped it
	fail "the window did not stay up: it exited with $code"
fi
# What would stop a user: the window toolkit or OpenGL failing, no display, a crash.
if grep -iE 'glfw|glx|failed to create window|cannot open display|panic:|segmentation|sigsegv' /tmp/window.log; then
	fail "the window reported an error"
fi
# Anything else the toolkit complains about is shown, not fatal: a bare container lacks parts
# of a desktop session (the settings portal) that change nothing a user sees.
grep -i 'error' /tmp/window.log | sed 's/^/::warning title=Clean Linux::/' || true
ok "the window opens and stays up"

# 4. Removed, it is gone.
apt-get "${apt_opts[@]}" remove -y pyxforge >/dev/null
[ ! -e /usr/bin/pyxforge ] || fail "/usr/bin/pyxforge is still there after removing the package"
ok "removed"
echo "PyxForge's package installs, runs, opens its window and is removed on a clean Ubuntu."
