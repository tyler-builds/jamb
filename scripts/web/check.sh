#!/bin/sh
# check.sh builds jamb for the web with TinyGo, runs the builds under Node, and
# checks their size. CI runs it on every pull request (the Web build job), so a
# change that breaks the web build, or makes it much larger, fails before the web
# platform adapter exists to show it.
#
# It builds with the web build's settings: goroutines through asyncify, with the
# import list of scripts/web/wasm-opt.sh; panic unwinding off, so a panic ends
# the program and recover does not catch it; and 1 MB goroutine stacks, since
# the main code runs on one and jamb's layout recursion overflows a smaller one,
# which corrupts memory instead of failing cleanly. It builds:
#   - examples/hello, the smallest app, for its size;
#   - examples/demo/cmd/webcheck, which renders every page of the demo, and must
#     exit cleanly;
#   - internal/webcheck, a page of cards that must draw the same pixels as the
#     same program built with Go.
#
# It needs Go, Node, brotli, and docker or podman (CONTAINER picks one), and it
# runs TinyGo from its official image. The module cache is shared with the
# container read-only, so run go mod download in each module (., examples/hello
# and examples/demo) first. TinyGo keeps
# its own build cache in TINYGO_CACHE (by default ~/.cache/jamb-tinygo).
set -eu

image=docker.io/tinygo/tinygo:0.44.0@sha256:7998bda3b044521b650a6a671229712b13f59a29ae373c60669907122e8d42f7
root=$(cd "$(dirname "$0")/../.." && pwd)
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
engine=${CONTAINER:-$(command -v docker >/dev/null 2>&1 && echo docker || echo podman)}
modcache=$(go env GOMODCACHE)
cache=${TINYGO_CACHE:-$HOME/.cache/jamb-tinygo}
mkdir -p "$cache"

# tinygo MODULE PKG OUT builds PKG of the module in the directory MODULE (both
# relative to the repository) into $out/OUT. GOWORK passes through, so CI builds
# each module on its own, as its other jobs do. The build runs as root, and git
# refuses a checkout that another user owns, as CI's is, so it stamps no VCS
# information; TinyGo passes GOFLAGS, not its own -buildvcs flag, to go list.
tinygo() {
	echo "== tinygo build $1/$2"
	"$engine" run --rm -u 0 \
		-v "$root:/src:ro,z" -v "$modcache:/gomod:ro,z" -v "$out:/out:z" -v "$cache:/tmp/.cache:z" \
		-e GOMODCACHE=/gomod -e GOPROXY=off -e GOFLAGS=-buildvcs=false -e GOWORK="${GOWORK:-}" -e HOME=/tmp \
		-e WASMOPT=/src/scripts/web/wasm-opt.sh \
		-w "/src/$1" "$image" \
		tinygo build -target wasm -no-debug -scheduler=asyncify -panic-unwind=none -stack-size=1MB \
		-o "/out/$3" "$2"
}

tinygo examples/hello . hello.wasm
tinygo examples/demo ./cmd/webcheck demo.wasm
tinygo . ./internal/webcheck cards-tinygo.wasm
"$engine" run --rm "$image" cat /usr/local/tinygo/targets/wasm_exec.js >"$out/tinygo_wasm_exec.js"
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
