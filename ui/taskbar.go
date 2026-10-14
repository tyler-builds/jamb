package ui

// ProgressState is the state of a taskbar button's progress bar; see
// TaskbarProgress.
type ProgressState uint8

const (
	// TaskbarNone shows no progress bar. It is the state of a new window.
	TaskbarNone ProgressState = iota
	// TaskbarNormal shows progress up to the value.
	TaskbarNormal
	// TaskbarIndeterminate shows that work is going on, without a value.
	TaskbarIndeterminate
	// TaskbarPaused shows progress up to the value, in the paused color.
	TaskbarPaused
	// TaskbarError shows progress up to the value, in the error color.
	TaskbarError
)

// TaskbarProgress is the progress bar of a window's taskbar button, which
// Window.SetTaskbarProgress sets. The button is shown while the window is.
type TaskbarProgress struct {
	State ProgressState
	// Value is the progress, from 0 to 1. A value outside that range, and NaN,
	// are clamped to it. TaskbarNone and TaskbarIndeterminate ignore it.
	Value float64
}
