// Package devhook connects the development tools to the packages they watch,
// without those packages importing the tools. Each hook is nil until package
// devtools sets it (see devtools/hot), so a program that does not link the
// tools only checks a nil function.
package devhook

// Local is called when ui.Local creates a component's local signal: id is the
// component's runtime ID, name the local's name and signal the *ui.Signal[T].
// The hook may set the signal's value before the component reads it. current
// returns the signal while its component is alive, and nil once it is gone;
// call it on the UI thread. It holds neither the component nor the signal.
var Local func(id uint64, name string, signal any, current func() any)

// Checkpoint writes the state that survives a restart under `jamb dev` to the
// file at path, and returns how many values it wrote (see devtools/hot).
var Checkpoint func(path string) (int, error)

// Started is called by the app when its startup code has run: in app.Run,
// before the first window's first frame. Values kept across a restart for the
// signals made until then are applied then, after the startup code's own.
var Started func()
