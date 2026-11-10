#!/bin/sh
# TinyGo runs this in place of wasm-opt (through its WASMOPT variable) for the web
# build. It runs TinyGo's own wasm-opt with TinyGo's arguments, and with four
# changes of its own.
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
#
# Asyncify also rewrites every function that calls one it cannot see into, in
# case that one pauses, and every caller of those. Two groups of functions that
# never pause are most of that, so the remove-list names them, which makes the
# smallest app 6% smaller and the demo 4%:
#   - runtime._panic and the map functions (runtime.hashmap*). A panic prints
#     its value through the Error and String methods of every type, and a map
#     hashes and compares its keys through function pointers, so nearly every
#     function looked as if it could pause.
#   - go-text/typesetting, which shapes text. It reads a font from memory
#     (through text.shapingFile) and calls nothing else of jamb's or the app's.
# A function on this list that did pause would corrupt the program, so the list
# holds only code that cannot: it calls no code of the app's, and reads no
# reader the app gives it. A panic whose Error method waits (for a lock another
# goroutine holds while it waits) stops the program without its message.
#
# TinyGo asks wasm-opt to keep the name of every function (-g), which is 3% of
# a build. With NAMES set to a file, the names go to that file instead, as
# lines of index:name: a stack trace of the build then shows wasm-function[N]
# for a function, and the file gives its name (see tinygo.sh).
if [ "${1:-}" = --version ]; then
	exec /usr/local/tinygo/bin/wasm-opt --version
fi
if [ -n "${NAMES:-}" ]; then
	for arg in "$@"; do
		shift
		[ "$arg" = -g ] || set -- "$@" "$arg"
	done
	set -- "$@" "--symbolmap=$NAMES"
fi
exec /usr/local/tinygo/bin/wasm-opt "$@" --pass-arg=asyncify-imports@env.none \
	--one-caller-inline-max-function-size=100 \
	'--pass-arg=asyncify-removelist@runtime._panic,runtime.hashmap*,github.com/go-text/typesetting/*,(*github.com/go-text/typesetting/*).*'
