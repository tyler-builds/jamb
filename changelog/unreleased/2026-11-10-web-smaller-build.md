### Changed

- A web build is smaller: the smallest app is 958 KB in Brotli, down from 1.05 MB, and the demo
  1.77 MB, down from 1.88 MB. The build no longer rewrites the text shaper, the map functions
  and every caller of a panic so that goroutines can pause in them, since none of them pauses
  (see `scripts/web/wasm-opt.sh`). It also leaves the names of the app's functions out of
  `app.wasm` and writes them to `app.names`, which the page fetches only when the app stops
  because of an error, to show the stack with the names. `KEEP_NAMES=1` keeps the names in
  `app.wasm` (see docs/web.md).
