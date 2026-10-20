# @pierre/highlights

`highlights.wasm` is the WebAssembly module of
[`@pierre/highlights`](https://www.npmjs.com/package/@pierre/highlights), the syntax
highlighter of the Pierre Computer Company
([source](https://github.com/pierrecomputer/pierre/tree/main/packages/highlights)). It is
copied unchanged from the npm package's `dist/highlights.wasm`. The release is
`Version` in `tables.go`, and its SHA-256 is `wasmSHA256` there; a test checks that
the two match.

@pierre/highlights is Copyright 2025 Pierre Computer Company and is licensed under the
Apache License, Version 2.0. The license text, from the package, is in
[LICENSE-APACHE](LICENSE-APACHE). Only the module is used: the package's themes and
its JavaScript host are not part of jamb. The Go code of this package, which runs the
module, is jamb's.

## Updating

Change the version in the `go:generate` line of `lexer.go`, then, in this directory:

```
go generate .
```

The update command downloads the release's tarball from the npm registry, checks it
against the registry's integrity hash, and writes `highlights.wasm`, `LICENSE-APACHE`
and `tables.go`. `tables.go` holds the version, the module's SHA-256, the language
names and aliases (from `dist/languages.js`) and the token names (from
`dist/token-types.js`), which must match the module. Then run the tests of this
package and of `codeedit/highlight`, and read the release's changes for the module's
interface: the memory layout and exports that `instance.go` uses come from the
package's `src/memory.wat`, `src/live.wat` and `lib/live.ts`.
