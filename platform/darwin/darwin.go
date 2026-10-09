// Package darwin will hold the macOS (Cocoa + Metal) adapter.
//
// It is intentionally empty for the first experiment: the spec orders the
// work Windows → macOS → Linux. The adapter must implement platform.Window
// and drive a platform.Handler exactly like platform/windows does; nothing in
// ui, layout or scene needs to change to add it.
package darwin
