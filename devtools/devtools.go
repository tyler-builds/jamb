// Package devtools links the visual inspector into an application. The inspector
// shows the element tree, styles, layout, accessibility and frame timings of a
// window, and Ctrl+Shift+I opens it (see docs/tools.md, "Visual inspector").
//
// The inspector is left out of a program unless the program imports this package,
// so a release build carries none of its code. Import it for its side effect in
// the main package:
//
//	import _ "github.com/tyler-builds/jamb/devtools"
//
// `jamb dev` links it into every app it builds, without the app importing it.
// Apps that set App.DevTools(true) need the import for Ctrl+Shift+I to work.
package devtools

import "github.com/tyler-builds/jamb/app"

func init() { app.RegisterInspector() }
