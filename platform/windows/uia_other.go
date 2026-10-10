//go:build windows && !amd64

package windows

import "syscall"

// newElementFromPointSlot returns the vtable entry of ElementProviderFromPoint.
// The doubles of the call cannot be read without the amd64 trampoline, so the
// method reports E_NOTIMPL and UI Automation falls back to navigation.
func newElementFromPointSlot() uintptr {
	return syscall.NewCallback(uiaElementFromPointUnsupported)
}

func uiaElementFromPointUnsupported(this, x, y, out uintptr) uintptr {
	if out != 0 {
		putPtr(out, 0)
	}
	return hrNotImpl
}

// newSetScrollPercentSlot returns the vtable entry of SetScrollPercent. Its
// doubles cannot be read without the amd64 trampoline, so the method reports
// E_NOTIMPL, and a client falls back to the Scroll method or the scroll bar.
func newSetScrollPercentSlot() uintptr {
	return syscall.NewCallback(uiaSetScrollPercentUnsupported)
}

func uiaSetScrollPercentUnsupported(this, horizontal, vertical uintptr) uintptr {
	return hrNotImpl
}

// newSetRangeValueSlot returns the vtable entry of IRangeValueProvider::SetValue.
// Its double cannot be read without the amd64 trampoline, so the method reports
// E_NOTIMPL, and a client falls back to the slider's keys.
func newSetRangeValueSlot() uintptr {
	return syscall.NewCallback(uiaSetRangeValueUnsupported)
}

func uiaSetRangeValueUnsupported(this, val uintptr) uintptr {
	return hrNotImpl
}
