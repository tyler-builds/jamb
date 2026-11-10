//go:build js && !tinygo

package web

// hostExports reports whether the build exports the functions that the host
// script calls: a TinyGo build does (see host_tinygo.go).
const hostExports = false

func hostTextAddress() int { return 0 }
