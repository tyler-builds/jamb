#!/bin/sh
# TinyGo runs this in place of wasm-opt (through its WASMOPT variable) for the web
# build. It runs TinyGo's own wasm-opt with TinyGo's arguments, and with two
# settings of its own.
#
# TinyGo runs asyncify, which gives a web build its goroutines, without a list of
# the imports that can pause, so asyncify assumes that all of them can. Every panic
# path prints through an import (fd_write), so nearly every function is rewritten
# to save and restore its stack. TinyGo pauses only from its own code, never inside
# an import, so the list is empty. With panic unwinding off, the list makes a web
# build of jamb about 7% smaller, rebuilds its UI about 10% faster and draws 15%
# faster. scripts/web/check.sh runs the result, so a TinyGo release that pauses
# inside an import fails CI.
#
# It also keeps a function that has one caller out of line unless it is small.
# wasm-opt inlines such a function whatever its size, and ignores //go:noinline,
# so the large, non-recursive parts of the UI's build (see ui.Runtime.buildDiv)
# became part of the frame of each level of its recursion. Safari gives each
# local of a WebAssembly function its own stack slot, and ran out of stack on
# the demo's deepest page.
exec /usr/local/tinygo/bin/wasm-opt "$@" --pass-arg=asyncify-imports@env.none \
	--one-caller-inline-max-function-size=100
