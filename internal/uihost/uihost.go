// Package uihost connects the app package to the methods of ui.Runtime that
// only a window host calls: the wake-up function, platform services, the
// cursor, window focus and the window areas of a custom title bar. ui/uitest
// uses it the same way for a headless runtime's clock and clipboard. These
// methods are not part of ui's public API, so apps and tests never see them.
package uihost

// Of returns the host side of a *ui.Runtime. The ui package sets it when it is
// initialized; app and ui/uitest assert the result to the interface of the
// methods they use.
var Of func(rt any) any

// Sample is one element as the latest paint drew it, as the host side's
// Samples reports it to ui/uitest's motion recording.
type Sample struct {
	// ID is the element's identity in the runtime, stable across frames as its
	// state is; Parent is its parent's, 0 for the root.
	ID, Parent uint64
	// Key is the element's key, or "". Label names an element without one:
	// "div", `text "Saved"`, "icon check".
	Key, Label string
	// Index is the element's position among its parent's children.
	Index int
	// Drawn reports whether the latest paint drew the element. An element
	// that it did not, as it was off screen, has only its ID, Parent, Key,
	// Label and Index set.
	Drawn bool
	// Exiting reports an element that was removed and is drawn while its exit
	// plays.
	Exiting bool
	// Visible reports whether some of the element lies inside its clip.
	Visible bool
	// Looping reports an element that a repeating animation moves: its own
	// keyframes, an Animate that renders it, or those of an ancestor.
	Looping bool
	// Blank reports an element that draws nothing of its own: a container
	// with no fill, border or shadow.
	Blank bool
	// ScrollX and ScrollY are the summed scroll offsets of its scrolling
	// ancestors; OwnScrollX and OwnScrollY are its own, when it scrolls.
	ScrollX, ScrollY, OwnScrollX, OwnScrollY float32
	// Style is the element's ui.DrawnStyle.
	Style any
}
