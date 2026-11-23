package windows

// This file holds the icon sizes. They call nothing of Windows, so the file has
// no build constraint, and its tests run on every platform.

// iconSize returns the side, in pixels, of a small icon at dpi, such as a
// window's small icon (its caption, the title row of its taskbar thumbnail), a
// notification area icon or a taskbar overlay. metric is
// GetSystemMetricsForDpi(SM_CXSMICON); when it reports nothing, the size is 16
// pixels at 96 DPI, scaled.
func iconSize(dpi uint32, metric func(dpi uint32) int32) int {
	return metricOrScaled(metric(dpi), 16, dpi)
}

// bigIconSize returns the side, in pixels, of a window's big icon at dpi, which
// Alt+Tab and the taskbar button show. metric is
// GetSystemMetricsForDpi(SM_CXICON); when it reports nothing, the size is 32
// pixels at 96 DPI, scaled.
func bigIconSize(dpi uint32, metric func(dpi uint32) int32) int {
	return metricOrScaled(metric(dpi), 32, dpi)
}

// metricOrScaled returns metric when the system reported one, and otherwise
// base pixels at 96 DPI scaled to dpi, rounded, and never below base.
func metricOrScaled(metric int32, base int, dpi uint32) int {
	if metric > 0 {
		return int(metric)
	}
	return max(base, (base*int(dpi)+48)/96)
}
