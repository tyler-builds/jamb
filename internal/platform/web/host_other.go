//go:build js && !tinygo

package web

// hostExports reports whether the build exports the functions that the host
// script calls: a TinyGo build does (see host_tinygo.go).
const hostExports = false

func hostTextAddress() int { return 0 }

// hostSetText is not called: a build with Go has no host script (see host_tinygo.go).
func (w *window) hostSetText(string, int, int, bool, bool) {}

// hostLayout is not called: a build with Go has no host script, and no
// editable element (see editable.go).
func (w *window) hostLayout(string) {}
