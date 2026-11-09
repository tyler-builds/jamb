#!/bin/sh
# check.sh builds jamb for the web with TinyGo, runs the builds under Node, and
# checks their size. CI runs it on every pull request (the Web build job), so a
# change that breaks the web build, or makes it much larger, fails before the web
# platform adapter exists to show it.
#
# It builds, with TinyGo and the web build's settings (see tinygo.sh):
#   - examples/hello, the smallest app, for its size;
#   - examples/demo/cmd/webcheck, which renders every page of the demo, and must
#     exit cleanly;
#   - internal/webcheck, a page of cards that must draw the same pixels as the
#     same program built with Go.
#
# It needs Go, Node, brotli, and docker or podman for TinyGo (see tinygo.sh). Run
# go mod download in each module it builds (., examples/hello and examples/demo)
# first.
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
tinygo() { sh "$root/scripts/web/tinygo.sh" "$1" "$2" "$out/$3"; }

tinygo examples/hello . hello.wasm
tinygo examples/demo ./cmd/webcheck demo.wasm
tinygo . ./internal/webcheck cards-tinygo.wasm
sh "$root/scripts/web/tinygo.sh" wasm_exec "$out/tinygo_wasm_exec.js"
(cd "$root" && GOOS=js GOARCH=wasm go build -o "$out/cards-go.wasm" ./internal/webcheck)
go_exec=$(go env GOROOT)/lib/wasm/wasm_exec.js

echo "== demo pages (TinyGo)"
timeout 300 node "$root/scripts/web/run.js" "$out/tinygo_wasm_exec.js" "$out/demo.wasm"

echo "== cards page, TinyGo against Go"
timeout 300 node "$root/scripts/web/run.js" "$out/tinygo_wasm_exec.js" "$out/cards-tinygo.wasm" | tee "$out/cards-tinygo.txt"
timeout 300 node "$root/scripts/web/run.js" "$go_exec" "$out/cards-go.wasm" | tee "$out/cards-go.txt"
grep '^pixels' "$out/cards-go.txt" >"$out/pixels-go.txt"
grep '^pixels' "$out/cards-tinygo.txt" >"$out/pixels-tinygo.txt"
if ! cmp -s "$out/pixels-go.txt" "$out/pixels-tinygo.txt" || ! [ -s "$out/pixels-go.txt" ]; then
	echo "FAIL: the TinyGo build draws different pixels from the Go build"
	exit 1
fi
echo "pixels match"

echo "== sizes (Brotli)"
status=0
for name in hello demo; do
	size=$(brotli -q 11 -c "$out/$name.wasm" | wc -c | tr -d ' ')
	base=$(awk -v n="$name" '$1 == n { print $2 }' "$root/scripts/web/sizes.txt")
	if [ -z "$base" ] || [ "$base" -eq 0 ]; then
		echo "$name: $size bytes (no baseline in scripts/web/sizes.txt)"
		status=1
	elif [ "$size" -gt $((base + base / 20)) ]; then
		echo "FAIL: $name is $size bytes, more than 5% over its baseline of $base (scripts/web/sizes.txt)"
		status=1
	elif [ "$size" -lt $((base - base / 20)) ]; then
		echo "$name: $size bytes, more than 5% under its baseline of $base: lower the baseline in scripts/web/sizes.txt"
	else
		echo "$name: $size bytes (baseline $base)"
	fi
done
exit $status
