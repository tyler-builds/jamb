package app

// drawWanted reports whether a window draws a frame. It must be on screen, and
// have a client area. A hidden or minimized window draws nothing, so its
// animations stop, and the platform schedules no timed wake-ups for it (see
// platform.Window.Visible).
func drawWanted(visible bool, width, height int) bool {
	return visible && width > 0 && height > 0
}
