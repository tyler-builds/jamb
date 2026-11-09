#!/bin/sh
# build.sh [-go] MODULE PKG DIR builds PKG of the Go module in the directory
# MODULE (relative to the repository) as a web page in DIR: index.html, which
# loads app.wasm with wasm_exec.js, and the fonts directory, the font files the
# app fetches. Serve DIR with any static file server, such as
# python3 -m http.server -d DIR, and open it in a browser.
#
# It builds with TinyGo and the web build's settings (see tinygo.sh), or with
# Go when -go comes first: a larger and slower build that is quicker to make.
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
gobuild=false
if [ "${1:-}" = -go ]; then
	gobuild=true
	shift
fi
if [ $# -ne 3 ]; then
	echo "usage: build.sh [-go] MODULE PKG DIR" >&2
	exit 2
fi
module=$1 pkg=$2
mkdir -p "$3"
dir=$(cd "$3" && pwd)
if $gobuild; then
	(cd "$root/$module" && GOOS=js GOARCH=wasm go build -o "$dir/app.wasm" "$pkg")
	cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$dir/wasm_exec.js"
else
	sh "$root/scripts/web/tinygo.sh" "$module" "$pkg" "$dir/app.wasm"
	sh "$root/scripts/web/tinygo.sh" wasm_exec "$dir/wasm_exec.js"
fi
# The fonts a page fetches (see internal/text/fonts_js.go).
(cd "$root" && go run scripts/web/fonts.go "$dir/fonts")
cp "$root/scripts/web/index.html" "$dir/index.html"
echo "built $dir"
