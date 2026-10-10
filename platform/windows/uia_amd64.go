//go:build windows && amd64

package windows

import "syscall"

// ElementProviderFromPoint takes two doubles. The Windows x64 convention passes
// them in XMM1 and XMM2, and syscall.NewCallback reads only the integer
// registers, so the method's vtable entry is a trampoline (uia_amd64.s) that
// moves the doubles into the integer registers and jumps to the callback.
//
// elementFromPointCB is the callback the trampoline jumps to. The trampoline
// reads it.
var elementFromPointCB uintptr

// elementFromPointStub is the trampoline. It is written in assembly.
func elementFromPointStub()

// elementFromPointStubAddr returns the address of elementFromPointStub. It is
// written in assembly.
func elementFromPointStubAddr() uintptr

// newElementFromPointSlot returns the vtable entry of ElementProviderFromPoint.
func newElementFromPointSlot() uintptr {
	elementFromPointCB = syscall.NewCallback(uiaElementFromPoint)
	return elementFromPointStubAddr()
}

// setScrollPercentCB is the callback that the SetScrollPercent trampoline jumps
// to. The trampoline reads it.
var setScrollPercentCB uintptr

// setScrollPercentStub is the trampoline of SetScrollPercent. It is written in
// assembly, as elementFromPointStub is.
func setScrollPercentStub()

// setScrollPercentStubAddr returns the address of setScrollPercentStub.
func setScrollPercentStubAddr() uintptr

// newSetScrollPercentSlot returns the vtable entry of IScrollProvider::
// SetScrollPercent.
func newSetScrollPercentSlot() uintptr {
	setScrollPercentCB = syscall.NewCallback(uiaSetScrollPercent)
	return setScrollPercentStubAddr()
}
