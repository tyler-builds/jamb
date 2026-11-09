#!/bin/sh
# tinygo.sh MODULE PKG OUT builds PKG of the Go module in the directory MODULE
# (relative to the repository) for the web with TinyGo, into the file OUT.
# tinygo.sh wasm_exec OUT writes TinyGo's wasm_exec.js, which runs the build in a
# page or under Node, to OUT.
#
# TinyGo runs from its official image, pinned by digest, in docker or podman
# (CONTAINER picks one). The module cache is shared with the container read-only,
# so run go mod download in the module first. TinyGo keeps its own build cache in
# TINYGO_CACHE (by default ~/.cache/jamb-tinygo).
#
# The build uses the web build's settings: goroutines through asyncify, with the
# import list of scripts/web/wasm-opt.sh; panic unwinding off, so a panic ends
# the program and recover does not catch it; and 1 MB goroutine stacks, since
# the main code runs on one and jamb's layout recursion overflows a smaller one,
# which corrupts memory instead of failing cleanly.
#
# GOWORK passes through, so CI builds each module on its own. The build runs as
# root, and git refuses a checkout that another user owns, as CI's is, so it
# stamps no VCS information; TinyGo passes GOFLAGS, not its own -buildvcs flag,
# to go list.
set -eu

image=docker.io/tinygo/tinygo:0.44.0@sha256:7998bda3b044521b650a6a671229712b13f59a29ae373c60669907122e8d42f7
root=$(cd "$(dirname "$0")/../.." && pwd)
engine=${CONTAINER:-$(command -v docker >/dev/null 2>&1 && echo docker || echo podman)}

if [ "$1" = wasm_exec ]; then
	"$engine" run --rm "$image" cat /usr/local/tinygo/targets/wasm_exec.js >"$2"
	exit 0
fi

module=$1 pkg=$2
outdir=$(cd "$(dirname "$3")" && pwd)
outname=$(basename "$3")
modcache=$(go env GOMODCACHE)
cache=${TINYGO_CACHE:-$HOME/.cache/jamb-tinygo}
mkdir -p "$cache"
echo "== tinygo build $module/$pkg"
"$engine" run --rm -u 0 \
	-v "$root:/src:ro,z" -v "$modcache:/gomod:ro,z" -v "$outdir:/out:z" -v "$cache:/tmp/.cache:z" \
	-e GOMODCACHE=/gomod -e GOPROXY=off -e GOFLAGS=-buildvcs=false -e GOWORK="${GOWORK:-}" -e HOME=/tmp \
	-e WASMOPT=/src/scripts/web/wasm-opt.sh \
	-w "/src/$module" "$image" \
	tinygo build -target wasm -no-debug -scheduler=asyncify -panic-unwind=none -stack-size=1MB \
	-o "/out/$outname" "$pkg"
