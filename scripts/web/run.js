// run.js WASM_EXEC MODULE [ARGS]: runs a js/wasm module under Node and exits with
// its exit status. WASM_EXEC is the wasm_exec.js of the compiler that built the
// module (Go's or TinyGo's). The module reads ARGS from globalThis.webcheckArgs.
// A js build fetches its fonts from the page's server, which Node has not: it
// reads them from the directory JAMB_FONT_DIR names (see
// internal/text/fonts_js.go), which check.sh writes with fonts.go.
const fs = require("fs");
const path = require("path");
globalThis.require = require;
globalThis.fs ??= fs;
globalThis.performance ??= require("perf_hooks").performance;
globalThis.crypto ??= require("crypto").webcrypto;
if (process.env.JAMB_FONT_DIR) {
  globalThis.jambFetchFont = (name) => fs.promises.readFile(path.join(process.env.JAMB_FONT_DIR, name));
}
require(path.resolve(process.argv[2]));
globalThis.webcheckArgs = process.argv[4] ?? "";
const go = new Go();
go.exit = (code) => process.exit(code);
WebAssembly.instantiate(fs.readFileSync(process.argv[3]), go.importObject)
  .then((r) => go.run(r.instance))
  .then(() => process.exit(0))
  .catch((err) => {
    console.error(err);
    process.exit(1);
  });
