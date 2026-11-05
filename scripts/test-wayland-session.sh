#!/bin/bash
# Checks the window session that puts jamb dev's new build where the old one
# was on Wayland (internal/platform/linux/session.go), in a nested, headless
# KWin of its own (Plasma 6.7 or later), so nothing shows on the desktop and
# nothing takes the keyboard. It runs three builds of examples/hello in turn,
# as jamb dev's swap does: each after the last one was frozen and its windows
# closed, and expects the second and third to be placed by the compositor.
#
#	dbus-run-session -- scripts/test-wayland-session.sh > result.txt; cat result.txt
#
# (Write the output to a file: the session's D-Bus services keep a pipe open.)
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d /tmp/jamb-wlsess.XXXX)
kwin=
trap 'kill $(jobs -p) $kwin 2>/dev/null; rm -rf "$work"' EXIT
printf 'package main\n\nimport _ "github.com/tyler-builds/jamb/devtools"\n' > "$work/link.go"
printf '{"Replace":{"%s/examples/hello/jamb_devtools_link.go":"%s/link.go"}}' "$root" "$work" > "$work/overlay.json"
(cd "$root" && go build -overlay "$work/overlay.json" -o "$work/hello" ./examples/hello)
(cd "$root/cmd/jamb" && go build -o "$work/jamb" .)
mkdir -m 700 "$work/ctl"
socket=jamb-wlsess-$$
kwin_wayland --virtual --socket "$socket" --width 1600 --height 1000 --no-lockscreen > "$work/kwin.log" 2>&1 &
kwin=$!
for _ in $(seq 1 100); do [ -S "$XDG_RUNTIME_DIR/$socket" ] && break; sleep 0.1; done
export WAYLAND_DISPLAY=$socket JAMB_CONTROL=1 JAMB_CONTROL_DIR=$work/ctl JAMB_WL_SESSION_FILE=$work/session JAMB_TRACE=1 JAMB_DEV_CHANGED_AT=1
fail=0
prev=
for n in 1 2 3; do
	"$work/hello" > "$work/app$n.log" 2>&1 &
	pid=$!
	sleep 3
	"$work/jamb" ctl -app "$pid" wait for=idle > /dev/null
	if [ -n "$prev" ]; then kill "$prev"; fi
	if [ "$n" -gt 1 ] && ! grep -q "put window-1 back" "$work/app$n.log"; then
		echo "FAIL: build $n was not placed by the compositor:"; cat "$work/app$n.log"; fail=1
	fi
	"$work/jamb" ctl -app "$pid" checkpoint path="$work/state.json" freeze=true > /dev/null
	prev=$pid
done
[ "$fail" = 0 ] && echo "ok: the compositor placed builds 2 and 3 where build 1 was"
exit $fail
