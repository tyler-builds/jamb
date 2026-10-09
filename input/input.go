// Package input defines the platform-neutral input vocabulary shared by the
// platform adapters and the ui package: keys, mouse buttons, modifiers and
// cursor shapes.
//
// It has no dependencies so that neither ui nor platform has to import the
// other.
package input

// MouseButton identifies a mouse button.
type MouseButton uint8

const (
	ButtonLeft MouseButton = iota
	ButtonRight
	ButtonMiddle
)

// Modifiers is a bit set of held modifier keys.
type Modifiers uint8

const (
	ModShift Modifiers = 1 << iota
	ModCtrl
	ModAlt
	ModSuper
)

// Has reports whether all modifiers in m2 are held.
func (m Modifiers) Has(m2 Modifiers) bool { return m&m2 == m2 }

// Cursor is a mouse cursor shape.
type Cursor uint8

const (
	CursorDefault Cursor = iota
	CursorPointer
	CursorText
	CursorResizeEW
	CursorResizeNS
	CursorNotAllowed
)

// Key identifies a physical-ish key. Printable characters should be read from
// text input events rather than from keys.
type Key uint16

const (
	KeyUnknown Key = iota
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyDelete
	KeyTab
	KeySpace
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyInsert
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	// KeyEqual is the =/+ key (and numpad +); KeyMinus is -/_ (and numpad -).
	KeyEqual
	KeyMinus
	// KeyMenu is the Menu (Apps) key, which opens a context menu.
	KeyMenu
)
