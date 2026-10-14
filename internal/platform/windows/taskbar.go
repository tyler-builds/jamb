//go:build windows

package windows

import (
	"image"
	"math"
	"runtime"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// A taskbar button's progress bar and overlay are set through ITaskbarList3 of
// shobjidl.h, called through its vtable as the jump list is. Vtable indices are
// counted from the declaration order, with IUnknown at 0 to 2.
const (
	itlHrInit           = 3
	itlSetProgressValue = 9
	itlSetProgressState = 10
	itlSetOverlayIcon   = 18

	// The TBPFLAG values of shobjidl.h.
	tbpfNoProgress    = 0x0
	tbpfIndeterminate = 0x1
	tbpfNormal        = 0x2
	tbpfError         = 0x4
	tbpfPaused        = 0x8

	// progressScale is the total of the progress values: the taskbar takes a
	// completed count out of a total, and a value of 1 is progressScale.
	progressScale = 10000
)

// The class and interface identifiers, from shobjidl.h.
var (
	clsidTaskbarList = guid{0x56fdf344, 0xfd6d, 0x11d0, [8]byte{0x95, 0x8a, 0x00, 0x60, 0x97, 0xc9, 0xa0, 0x90}}
	iidTaskbarList3  = guid{0xea1afb91, 0x9e28, 0x4b86, [8]byte{0x90, 0xe9, 0x9e, 0x9f, 0x8a, 0x5e, 0xef, 0xaf}}
)

// taskbarButtonMsg is the message the taskbar sends a window when it makes the
// window's taskbar button, or makes it again. It is registered in Run.
var taskbarButtonMsg uint32

// taskbarButton is a window's taskbar button, as the progress bar and the
// overlay are set on it. comTaskbarButton is the real one; tests use a fake.
type taskbarButton interface {
	SetProgressState(flags uint32) error
	SetProgressValue(completed, total uint64) error
	SetOverlayIcon(icon uintptr, description string) error
}

// comTaskbarButton is a button made from an ITaskbarList3, for the window hwnd.
type comTaskbarButton struct {
	list uintptr // the ITaskbarList3; the caller releases it
	hwnd uintptr
}

func (b *comTaskbarButton) SetProgressState(flags uint32) error {
	return comCall("SetProgressState", b.list, itlSetProgressState, b.hwnd, uintptr(flags))
}

func (b *comTaskbarButton) SetProgressValue(completed, total uint64) error {
	return comCall("SetProgressValue", b.list, itlSetProgressValue, b.hwnd, uintptr(completed), uintptr(total))
}

// SetOverlayIcon sets the badge; a zero icon removes it.
func (b *comTaskbarButton) SetOverlayIcon(icon uintptr, description string) error {
	var live []*uint16
	var desc uintptr
	if description != "" {
		desc = wide(&live, description)
	}
	err := comCall("SetOverlayIcon", b.list, itlSetOverlayIcon, b.hwnd, icon, desc)
	runtime.KeepAlive(live)
	return err
}

// taskbarState is what a window shows on its taskbar button. The window keeps
// it, so a state set before the button exists, or again after the taskbar made
// the button anew, is applied when the button is there (see apply).
type taskbarState struct {
	progress    platform.TaskbarState
	value       float32
	overlay     uintptr // the badge icon, which the window owns; 0 for none
	description string
}

// apply sets the state on b, in order: the progress state, the progress value,
// then the overlay. It stops at the first call that fails.
func (s *taskbarState) apply(b taskbarButton) error {
	if err := b.SetProgressState(progressFlags(s.progress)); err != nil {
		return err
	}
	completed, total := progressValue(s.value)
	if err := b.SetProgressValue(completed, total); err != nil {
		return err
	}
	return b.SetOverlayIcon(s.overlay, s.description)
}

// progressFlags maps a progress state to its TBPFLAG.
func progressFlags(s platform.TaskbarState) uint32 {
	switch s {
	case platform.TaskbarIndeterminate:
		return tbpfIndeterminate
	case platform.TaskbarNormal:
		return tbpfNormal
	case platform.TaskbarPaused:
		return tbpfPaused
	case platform.TaskbarError:
		return tbpfError
	}
	return tbpfNoProgress
}

// progressValue returns the completed count and the total for a value from 0 to
// 1. A value outside that range, or NaN, is clamped to it.
func progressValue(v float32) (completed, total uint64) {
	f := float64(v)
	if !(f > 0) {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return uint64(math.Round(f * progressScale)), progressScale
}

// SetTaskbarProgress sets the progress bar of the window's taskbar button, and
// remembers it for the button when it is made. It is called on the UI thread.
func (w *window) SetTaskbarProgress(state platform.TaskbarState, value float32) {
	w.taskbar.progress, w.taskbar.value = state, value
	w.applyTaskbar()
}

// SetTaskbarOverlay sets the badge over the taskbar button's icon, or removes it
// when img is nil. The badge is drawn at the window's small icon size. It is
// called on the UI thread.
func (w *window) SetTaskbarOverlay(img image.Image, description string) error {
	var icon uintptr
	if img != nil {
		var err error
		if icon, err = newIcon(img, iconSize(w.dpi, smallIconMetric)); err != nil {
			return err
		}
	}
	old := w.taskbar.overlay
	w.taskbar.overlay, w.taskbar.description = icon, description
	w.applyTaskbar()
	// The taskbar has the new badge now, so the old icon can go.
	win32.DestroyIcon(old)
	return nil
}

// applyTaskbar sets the window's state on its button, if the taskbar has made one.
// A failure is not reported: the button may be hidden with its window.
func (w *window) applyTaskbar() {
	if w.taskbarBtn != nil {
		w.taskbar.apply(w.taskbarBtn)
	}
}

// taskbarButtonCreated makes the window's ITaskbarList3 when the taskbar makes its
// button, and applies the window's state to it. The taskbar sends the message
// again when it makes the button anew, such as after Explorer restarts.
func (w *window) taskbarButtonCreated() {
	w.releaseTaskbarButton()
	list, err := createCOM("ITaskbarList3", &clsidTaskbarList, &iidTaskbarList3)
	if err != nil {
		return
	}
	if err := comCall("HrInit", list, itlHrInit); err != nil {
		release(list)
		return
	}
	w.taskbarBtn = &comTaskbarButton{list: list, hwnd: w.hwnd}
	w.applyTaskbar()
}

// releaseTaskbarButton drops the window's ITaskbarList3, if it has one.
func (w *window) releaseTaskbarButton() {
	if w.taskbarBtn != nil {
		release(w.taskbarBtn.list)
		w.taskbarBtn = nil
	}
}

// destroyTaskbar releases the taskbar objects of a window that is being destroyed.
func (w *window) destroyTaskbar() {
	w.releaseTaskbarButton()
	win32.DestroyIcon(w.taskbar.overlay)
	w.taskbar.overlay = 0
}
