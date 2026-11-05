// Package devhook connects the development tools to the packages they watch,
// without those packages importing the tools. Each hook is nil until package
// devtools sets it (see devtools/hot), so a program that does not link the
// tools only checks a nil function.
package devhook

// Local is called when ui.Local creates a component's local signal: path names
// the component by its place in the tree (see ui's sitePath), name is the
// local's name and signal the *ui.Signal[T].
// The hook may set the signal's value before the component reads it. current
// returns the signal while its component is alive, and nil once it is gone;
// call it on the UI thread. It holds neither the component nor the signal.
var Local func(path, name string, signal any, current func() any)

// Checkpoint writes the state that survives a restart under `jamb dev` to the
// file at path, and returns how many values it wrote (see devtools/hot).
var Checkpoint func(path string) (int, error)

// Started is called by the app when its startup code has run: in app.Run,
// before the first window's first frame. Values kept across a restart for the
// signals made until then are applied then, after the startup code's own.
var Started func()

// TagComponent names a *ui.ComponentElement by its call in the app's source,
// for jamb dev's rewrite. Package ui sets it.
var TagComponent func(component any, site string)

// Runtime is called when a ui.Runtime is made, with a function that returns it
// while it is alive and nil after. Windows make theirs in the order they open,
// which numbers them.
var Runtime func(current func() any)

// ViewCapture returns a runtime's view state (focus, scroll, carets) as JSON,
// and ViewRestore gives it back to a new runtime, applied after its first
// layout. Package ui sets them.
var (
	ViewCapture func(runtime any) []byte
	ViewRestore func(runtime any, state []byte)
)
