// Package uihost connects the app package to the methods of ui.Runtime that
// only a window host calls: the wake-up function, platform services, the
// cursor, window focus and the window areas of a custom title bar. ui/uitest
// uses it the same way for a headless runtime's clock and clipboard. These
// methods are not part of ui's public API, so apps and tests never see them.
package uihost

import "github.com/tyler-builds/jamb/internal/text"

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
	// Matched is the element's shared-element id (ui.DivElement.Matched), or "".
	Matched string
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

// TextContext is the focused text field as the host side reports it to a
// window whose input methods read and edit the text around the caret (see
// platform.TextContext, whose fields and numbering it has). Focused is clear
// when no element that takes text has the focus.
type TextContext struct {
	Focused       bool
	Text          string
	Anchor, Caret int
	HasText       bool
	Purpose       uint8 // a platform.TextPurpose
	Multiline     bool
	Enter         uint8 // a platform.EnterKey
	// Box is where the field shows its text, and TextX, TextY where its text
	// starts, FontSize its size and LineHeight the distance between its lines,
	// in logical pixels (see platform.TextContext). Box is empty when unknown.
	Box                  [4]float32 // x, y, width, height
	TextX, TextY         float32
	FontSize, LineHeight float32
	// WrapWidth is the width the field's lines wrap at, 0 when they do not.
	WrapWidth float32
	// Runs are the field's text as it lays it out, in pieces that cover it in
	// order, and Faces the faces they are in (see platform.TextContext.Runs).
	// Both are shared and unchanging, the same slices while the layout is.
	Runs  []TextRun
	Faces []text.FaceData
	// Carets returns where the field puts the caret at each offset of its
	// text from start to end, for a test of a window that lays the same text
	// out. Nil when the field reports no runs.
	Carets func(start, end int) []Caret
}

// TextRun is a piece of a field's text as the field lays it out: the bytes
// from Start to End of its text, on the line Line, of the kind Kind (a
// text.PieceKind), in Faces[Face] or none (-1), Width wide, with Spacing added
// after each character to the font's advances. Widths are logical pixels.
type TextRun struct {
	Start, End, Line, Face int
	Kind                   uint8
	Width, Spacing         float32
}

// Caret is where a field puts the caret at the byte Offset of its text: X
// from the start of its lines, on the line Line, in logical pixels.
type Caret struct {
	Offset int
	X      float32
	Line   int
}
