package ui

import "github.com/tyler-builds/jamb/input"

// MouseEvent describes a pointer event. Coordinates are logical pixels.
type MouseEvent struct {
	// X, Y relative to the window; LocalX, LocalY relative to the element
	// whose handler is running.
	X, Y           float32
	LocalX, LocalY float32
	// Width and Height are the size of the element whose handler is running.
	Width, Height float32
	Button        input.MouseButton
	Clicks        int
	Mods          input.Modifiers
	stopped       bool
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

// GesturePhase says where a pinch gesture is (see PinchEvent).
type GesturePhase uint8

const (
	// GestureBegin starts a gesture. It carries no change.
	GestureBegin GesturePhase = iota
	// GestureUpdate carries the change since the previous event of the gesture.
	GestureUpdate
	// GestureEnd ends a gesture. It carries no change.
	GestureEnd
)

// PinchEvent describes a pinch (zoom) gesture of a touchpad or touchscreen. A
// gesture is a GestureBegin, any number of GestureUpdate events, and a
// GestureEnd. Scale, PanX and PanY are the change since the previous event of
// the gesture: Scale is multiplicative, 1 when nothing changed, and PanX and
// PanY are in logical pixels.
type PinchEvent struct {
	Scale      float32
	PanX, PanY float32
	Phase      GesturePhase
	// Inertia is set while the gesture continues after the fingers have lifted.
	// With reduced motion the change of an inertia event is left out.
	Inertia bool
	// X, Y relative to the window, at the center of the gesture; LocalX, LocalY
	// relative to the element whose handler is running.
	X, Y           float32
	LocalX, LocalY float32
	Mods           input.Modifiers
	stopped        bool
}

// StopPropagation prevents ancestors from handling the gesture. The zoom of the
// window is then left alone too.
func (e *PinchEvent) StopPropagation() { e.stopped = true }

// KeyEvent describes a key press or release.
type KeyEvent struct {
	Key     input.Key
	Mods    input.Modifiers
	Repeat  bool
	stopped bool
}

// StopPropagation prevents ancestors from receiving the event.
func (e *KeyEvent) StopPropagation() { e.stopped = true }
