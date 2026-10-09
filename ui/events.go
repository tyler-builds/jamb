package ui

import "github.com/tyler-builds/jamb/input"

// MouseEvent describes a pointer event. Coordinates are logical pixels.
type MouseEvent struct {
	// X, Y relative to the window; LocalX, LocalY relative to the element
	// whose handler is running.
	X, Y           float32
	LocalX, LocalY float32
	Button         input.MouseButton
	Clicks         int
	Mods           input.Modifiers
	stopped        bool
}

// StopPropagation prevents ancestors from receiving the event.
func (e *MouseEvent) StopPropagation() { e.stopped = true }

// ScrollEvent describes a wheel or touchpad scroll in logical pixels.
type ScrollEvent struct {
	DeltaX, DeltaY float32
	Mods           input.Modifiers
	stopped        bool
}

// StopPropagation prevents ancestors (and built-in scrolling) from handling the event.
func (e *ScrollEvent) StopPropagation() { e.stopped = true }

// KeyEvent describes a key press or release.
type KeyEvent struct {
	Key     input.Key
	Mods    input.Modifiers
	Repeat  bool
	stopped bool
}

// StopPropagation prevents ancestors from receiving the event.
func (e *KeyEvent) StopPropagation() { e.stopped = true }
