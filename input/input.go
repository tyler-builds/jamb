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
	// ButtonLeft is the left mouse button.
	ButtonLeft MouseButton = iota
	// ButtonRight is the right mouse button.
	ButtonRight
	// ButtonMiddle is the middle mouse button, the wheel.
	ButtonMiddle
)

// Modifiers is a bit set of held modifier keys.
type Modifiers uint8

const (
	// ModShift is the Shift key.
	ModShift Modifiers = 1 << iota
	// ModCtrl is the Ctrl key.
	ModCtrl
	// ModAlt is the Alt key.
	ModAlt
	// ModSuper is the Windows key.
	ModSuper
)

// Has reports whether all modifiers in m2 are held.
func (m Modifiers) Has(m2 Modifiers) bool { return m&m2 == m2 }

// Cursor is a mouse cursor shape.
type Cursor uint8

const (
	// CursorDefault is the arrow cursor.
	CursorDefault Cursor = iota
	// CursorPointer is the hand cursor, for a link or a button.
	CursorPointer
	// CursorText is the I-beam cursor, for text.
	CursorText
	// CursorResizeEW is the cursor for resizing horizontally.
	CursorResizeEW
	// CursorResizeNS is the cursor for resizing vertically.
	CursorResizeNS
	// CursorNotAllowed is the cursor for an action that cannot be done.
	CursorNotAllowed
)

// Key identifies a physical-ish key. Printable characters should be read from
// text input events rather than from keys.
type Key uint16

const (
	// KeyUnknown is a key that the platform does not name.
	KeyUnknown Key = iota
	// KeyEnter is the Enter key.
	KeyEnter
	// KeyEscape is the Escape key.
	KeyEscape
	// KeyBackspace is the Backspace key.
	KeyBackspace
	// KeyDelete is the Delete key.
	KeyDelete
	// KeyTab is the Tab key.
	KeyTab
	// KeySpace is the space bar.
	KeySpace
	// KeyLeft is the Left arrow key.
	KeyLeft
	// KeyRight is the Right arrow key.
	KeyRight
	// KeyUp is the Up arrow key.
	KeyUp
	// KeyDown is the Down arrow key.
	KeyDown
	// KeyHome is the Home key.
	KeyHome
	// KeyEnd is the End key.
	KeyEnd
	// KeyPageUp is the Page Up key.
	KeyPageUp
	// KeyPageDown is the Page Down key.
	KeyPageDown
	// KeyInsert is the Insert key.
	KeyInsert
	// KeyF1 is the F1 function key.
	KeyF1
	// KeyF2 is the F2 function key.
	KeyF2
	// KeyF3 is the F3 function key.
	KeyF3
	// KeyF4 is the F4 function key.
	KeyF4
	// KeyF5 is the F5 function key.
	KeyF5
	// KeyF6 is the F6 function key.
	KeyF6
	// KeyF7 is the F7 function key.
	KeyF7
	// KeyF8 is the F8 function key.
	KeyF8
	// KeyF9 is the F9 function key.
	KeyF9
	// KeyF10 is the F10 function key.
	KeyF10
	// KeyF11 is the F11 function key.
	KeyF11
	// KeyF12 is the F12 function key.
	KeyF12
	// KeyA is the A key.
	KeyA
	// KeyB is the B key.
	KeyB
	// KeyC is the C key.
	KeyC
	// KeyD is the D key.
	KeyD
	// KeyE is the E key.
	KeyE
	// KeyF is the F key.
	KeyF
	// KeyG is the G key.
	KeyG
	// KeyH is the H key.
	KeyH
	// KeyI is the I key.
	KeyI
	// KeyJ is the J key.
	KeyJ
	// KeyK is the K key.
	KeyK
	// KeyL is the L key.
	KeyL
	// KeyM is the M key.
	KeyM
	// KeyN is the N key.
	KeyN
	// KeyO is the O key.
	KeyO
	// KeyP is the P key.
	KeyP
	// KeyQ is the Q key.
	KeyQ
	// KeyR is the R key.
	KeyR
	// KeyS is the S key.
	KeyS
	// KeyT is the T key.
	KeyT
	// KeyU is the U key.
	KeyU
	// KeyV is the V key.
	KeyV
	// KeyW is the W key.
	KeyW
	// KeyX is the X key.
	KeyX
	// KeyY is the Y key.
	KeyY
	// KeyZ is the Z key.
	KeyZ
	// Key0 is the 0 key of the main keyboard.
	Key0
	// Key1 is the 1 key of the main keyboard.
	Key1
	// Key2 is the 2 key of the main keyboard.
	Key2
	// Key3 is the 3 key of the main keyboard.
	Key3
	// Key4 is the 4 key of the main keyboard.
	Key4
	// Key5 is the 5 key of the main keyboard.
	Key5
	// Key6 is the 6 key of the main keyboard.
	Key6
	// Key7 is the 7 key of the main keyboard.
	Key7
	// Key8 is the 8 key of the main keyboard.
	Key8
	// Key9 is the 9 key of the main keyboard.
	Key9
	// KeyEqual is the =/+ key (and numpad +); KeyMinus is -/_ (and numpad -).
	KeyEqual
	// KeyMinus is the minus key.
	KeyMinus
	// KeyMenu is the Menu (Apps) key, which opens a context menu.
	KeyMenu
	// The punctuation keys are named by the characters the US layout prints on them. They name
	// the key's position, so on another layout the key may print a different character.
	KeySlash        // /?
	KeyBackslash    // \|
	KeyLeftBracket  // [{
	KeyRightBracket // ]}
	KeySemicolon    // ;:
	KeyApostrophe   // '"
	KeyComma        // ,<
	KeyPeriod       // .>
	KeyBacktick     // `~
)
