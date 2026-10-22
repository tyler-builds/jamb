package app

// drawWanted reports whether a window draws a frame. It must be on screen, and
// have a client area. A hidden or minimized window draws nothing, so its
// animations stop, and the platform schedules no timed wake-ups for it (see
// platform.Window.Visible). The first frame is the exception: the platform draws
// it before it shows the window (see windows.Open), so the window never appears
// blank. drawn is whether a frame has been presented already.
func drawWanted(visible, drawn bool, width, height int) bool {
	return (visible || !drawn) && width > 0 && height > 0
}
