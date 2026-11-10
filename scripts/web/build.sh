#!/bin/sh
# build.sh [-go] MODULE PKG DIR builds PKG of the Go module in the directory
# MODULE (relative to the repository) as a web page in DIR: index.html, which
# loads app.wasm with wasm_exec.js and jamb.js (the host script, which delivers
# the app's frames and input), and the fonts directory, the font files the app
# fetches. Serve DIR with any static file server, such as
# python3 -m http.server -d DIR, and open it in a browser.
#
# It builds with TinyGo and the web build's settings (see tinygo.sh), or with
# Go when -go comes first: a larger and slower build that is quicker to make.
#
# A TinyGo build leaves the names of the app's functions out of app.wasm and
# writes them to app.names, which the page fetches only when the app stops
# because of an error, to name the functions of the stack it shows. Without the
# file the page shows their numbers. KEEP_NAMES=1 keeps the names in app.wasm.
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
	rm -f "$dir/app.names" # a build with Go keeps its names
	cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$dir/wasm_exec.js"
else
	sh "$root/scripts/web/tinygo.sh" "$module" "$pkg" "$dir/app.wasm"
	sh "$root/scripts/web/tinygo.sh" wasm_exec "$dir/wasm_exec.js"
fi
# The fonts a page fetches (see internal/text/fonts_js.go).
(cd "$root" && go run scripts/web/fonts.go "$dir/fonts")
cp "$root/scripts/web/index.html" "$root/scripts/web/jamb.js" "$dir/"
echo "built $dir"
