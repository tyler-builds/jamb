#!/bin/sh
# browser.sh builds the demo as a page and checks it in Chromium with
# browser.cjs: it draws with WebGL 2, its frame matches the CPU renderer's, and
# its text fields take typing. CI runs it in the Web build job.
#
# It needs what build.sh needs, and Node with npm. It installs Playwright, at the
# version pinned here, in a temporary directory; Playwright downloads Chromium
# into its own cache (~/.cache/ms-playwright) the first time.
# PLAYWRIGHT_DEPS=1 also installs Chromium's system libraries, which needs
# root, as on a CI runner.
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT

sh "$root/scripts/web/build.sh" examples/demo . "$out/page"

npm install --silent --no-audit --no-fund --prefix "$out/pw" playwright@1.64.1 >/dev/null
deps=
if [ "${PLAYWRIGHT_DEPS:-}" = 1 ]; then
	deps=--with-deps
fi
"$out/pw/node_modules/.bin/playwright" install $deps chromium

# The search field of the demo's header.
NODE_PATH="$out/pw/node_modules" timeout 300 node "$root/scripts/web/browser.cjs" "$out/page" 1000 49
