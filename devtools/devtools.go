// Package devtools links the development tools into an application: the visual
// inspector and the control channel. The inspector shows the element tree, styles,
// layout, accessibility and frame timings of a window, and Ctrl+Shift+I opens it
// (see docs/tools.md, "Visual inspector"). The control channel lets `jamb ctl` and
// coding agents read and drive the running app when JAMB_CONTROL=1 (see
// docs/tools.md, "Control channel").
//
// The tools are left out of a program unless the program imports this package,
// so a release build carries none of their code. Import it for its side effect in
// the main package:
//
//	import _ "github.com/tyler-builds/jamb/devtools"
//
// `jamb dev` links it into every app it builds, without the app importing it.
// Apps that set App.DevTools(true) need the import for Ctrl+Shift+I to work.
package devtools

import "github.com/tyler-builds/jamb/app"

func init() {
	app.RegisterInspector()
	app.RegisterControl()
}
