//go:build windows

package windows

import (
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// A custom title bar keeps the native window (WS_OVERLAPPEDWINDOW, so resize,
// Aero Snap, Win+arrows, the minimize and maximize animations and the rounded
// corners of Windows 11 all still apply) and removes only the caption.
//
// As in Windows Terminal and Chromium, Windows still computes the frame: the
// resize borders are outside the client area on the left, right and bottom,
// exactly where a native window's invisible borders are, so the content
// reaches the visible edges. Only the top of the frame is removed: the client
// area starts at the window's top edge, and a thin band inside it stays a
// resize handle. The app's title-bar areas are reported through
// platform.TitleBarHandler.

// applyCustomFrame is called once the window exists. A 1 px margin makes DWM
// draw the shadow around the window.
//
// The top margin must stay 0. With the frame extended at the top, Windows 11
// no longer shows the Snap Layouts flyout when the pointer rests on the
// maximize button (HTMAXBUTTON), although the button otherwise works. The
// shadow does not need it.
func (w *window) applyCustomFrame() {
	win32.ExtendFrameIntoClientArea(w.hwnd, win32.Margins{Left: 1, Right: 1, Bottom: 1})
	// Windows only recalculates the non-client area when the frame changes.
	win32.SetWindowPos(w.hwnd, 0, 0, 0, 0,
		win32.SWP_NOMOVE|win32.SWP_NOSIZE|win32.SWP_NOZORDER|win32.SWP_NOACTIVATE|win32.SWP_FRAMECHANGED)
}

// frameInsets is the width of the resize frame in physical pixels, horizontal
// and vertical. The resize border is this wide, outside the client area on
// the sides and bottom and, for the top, inside it.
func (w *window) frameInsets() (x, y int32) {
	pad := win32.SystemMetricsForDpi(win32.SM_CXPADDEDBORDER, w.dpi)
	return win32.SystemMetricsForDpi(win32.SM_CXSIZEFRAME, w.dpi) + pad,
		win32.SystemMetricsForDpi(win32.SM_CYSIZEFRAME, w.dpi) + pad
}

// calcClientSize implements WM_NCCALCSIZE (wp is TRUE, lp is an
// NCCALCSIZE_PARAMS whose first rectangle is the proposed window rectangle).
// Windows computes the client area with the caption; the caption is taken
// back out by restoring the top edge. A maximized window's client area must
// also start below its top frame, which hangs above the work area.
func (w *window) calcClientSize(lp uintptr) uintptr {
	params := (*win32.NCCalcSizeParams)(ptr(lp))
	top := params.Rgrc[0].Top
	win32.DefWindowProc(w.hwnd, win32.WM_NCCALCSIZE, 1, lp)
	params.Rgrc[0].Top = top
	if win32.IsZoomed(w.hwnd) {
		_, iy := w.frameInsets()
		params.Rgrc[0].Top += iy
	}
	return 0
}

// hitTest implements WM_NCHITTEST for a custom title bar. The borders come
// from Windows, which knows the frame; the top band, inside the client area,
// is found here, as are the title-bar areas.
func (w *window) hitTest(lp uintptr) uintptr {
	ht := win32.DefWindowProc(w.hwnd, win32.WM_NCHITTEST, 0, lp)
	sx, sy := win32.PointFromLParam(lp)
	p := win32.POINT{X: sx, Y: sy}
	win32.ScreenToClient(w.hwnd, &p)
	if !win32.IsZoomed(w.hwnd) {
		_, fy := w.frameInsets()
		if p.Y < fy {
			switch ht {
			case win32.HTLEFT:
				return win32.HTTOPLEFT
			case win32.HTRIGHT:
				return win32.HTTOPRIGHT
			case win32.HTCLIENT:
				return win32.HTTOP
			}
		}
	}
	switch ht {
	case win32.HTLEFT, win32.HTRIGHT, win32.HTTOP, win32.HTBOTTOM,
		win32.HTBOTTOMLEFT, win32.HTBOTTOMRIGHT, win32.HTTOPLEFT, win32.HTTOPRIGHT:
		return ht
	}
	switch w.titleArea(float32(p.X), float32(p.Y)) {
	case platform.TitleDrag:
		return win32.HTCAPTION
	case platform.TitleMinimize:
		return win32.HTMINBUTTON
	case platform.TitleMaximize:
		return win32.HTMAXBUTTON
	case platform.TitleClose:
		return win32.HTCLOSE
	}
	return win32.HTCLIENT
}

// titleArea asks the handler which title-bar area (x, y) is in. Points are
// physical pixels relative to the client area.
func (w *window) titleArea(x, y float32) platform.TitleArea {
	if !w.opts.CustomTitleBar {
		return platform.TitleNone
	}
	if th, ok := w.handler.(platform.TitleBarHandler); ok {
		return th.TitleArea(w, x, y)
	}
	return platform.TitleNone
}

// captionArea maps a non-client hit-test code of a title-bar button to its
// area. Other codes map to TitleNone.
func captionArea(ht uintptr) platform.TitleArea {
	switch ht {
	case win32.HTMINBUTTON:
		return platform.TitleMinimize
	case win32.HTMAXBUTTON:
		return platform.TitleMaximize
	case win32.HTCLOSE:
		return platform.TitleClose
	}
	return platform.TitleNone
}
