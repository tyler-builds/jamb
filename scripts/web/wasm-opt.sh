#!/bin/sh
# TinyGo runs this in place of wasm-opt (through its WASMOPT variable) for the web
# build. It runs TinyGo's own wasm-opt with TinyGo's arguments, and tells asyncify
# that no import can pause.
#
# TinyGo runs asyncify, which gives a web build its goroutines, without a list of
# the imports that can pause, so asyncify assumes that all of them can. Every panic
# path prints through an import (fd_write), so nearly every function is rewritten
# to save and restore its stack. TinyGo pauses only from its own code, never inside
# an import, so the list is empty. With panic unwinding off, the list makes a web
# build of jamb about 7% smaller, rebuilds its UI about 10% faster and draws 15%
# faster. scripts/web/check.sh runs the result, so a TinyGo release that pauses
# inside an import fails CI.
exec /usr/local/tinygo/bin/wasm-opt "$@" --pass-arg=asyncify-imports@env.none
