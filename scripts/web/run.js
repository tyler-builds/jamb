// run.js WASM_EXEC MODULE [ARGS]: runs a js/wasm module under Node and exits with
// its exit status. WASM_EXEC is the wasm_exec.js of the compiler that built the
// module (Go's or TinyGo's). The module reads ARGS from globalThis.webcheckArgs.
// A js build fetches its fonts from the page's server, which Node has not: it
// reads them from the directory JAMB_FONT_DIR names (see
// internal/text/fonts_js.go), which check.sh writes with fonts.go. A TinyGo
// build leaves the names of its functions out of the module, in MODULE's .names
// file (see tinygo.sh): an error's stack is printed with the names from it.
const fs = require("fs");
const path = require("path");
globalThis.require = require;
globalThis.fs ??= fs;
globalThis.performance ??= require("perf_hooks").performance;
globalThis.crypto ??= require("crypto").webcrypto;
if (process.env.JAMB_FONT_DIR) {
  globalThis.jambFetchFont = (name) => fs.promises.readFile(path.join(process.env.JAMB_FONT_DIR, name));
}
// report prints an error, naming the functions of its stack from the .names
// file of the module, where the stack shows them as wasm-function[N].
const report = (err) => {
  const stack = String((err && err.stack) || err);
  const names = new Map();
  try {
    for (const line of fs.readFileSync(process.argv[3].replace(/\.wasm$/, "") + ".names", "utf8").split("\n")) {
      const i = line.indexOf(":");
      if (i > 0) names.set(line.slice(0, i), line.slice(i + 1));
    }
  } catch {
    // No file: a build that keeps its names.
  }
  console.error(stack.replace(/wasm-function\[(\d+)\]/g, (frame, n) => names.get(n) || frame));
};
process.on("uncaughtException", (err) => {
  report(err);
  process.exit(1);
});
require(path.resolve(process.argv[2]));
globalThis.webcheckArgs = process.argv[4] ?? "";
const go = new Go();
go.exit = (code) => process.exit(code);
WebAssembly.instantiate(fs.readFileSync(process.argv[3]), go.importObject)
  .then((r) => go.run(r.instance))
  .then(() => process.exit(0))
  .catch((err) => {
    report(err);
    process.exit(1);
  });
