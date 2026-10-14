//go:build windows

package windows

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/win32"
)

// TestContentDelta checks how a DirectManipulation content transform is read:
// its scale, its translation, and whether it holds a change.
func TestContentDelta(t *testing.T) {
	cases := []struct {
		name              string
		m                 [6]float32
		scale, panX, panY float32
		moved             bool
	}{
		{"identity", [6]float32{1, 0, 0, 1, 0, 0}, 1, 0, 0, false},
		{"zoom in", [6]float32{1.25, 0, 0, 1.25, 0, 0}, 1.25, 0, 0, true},
		{"zoom out with the center moved", [6]float32{0.5, 0, 0, 0.5, 30, 40}, 0.5, 30, 40, true},
		{"pan without a scale", [6]float32{1, 0, 0, 1, 12, -4}, 1, 12, -4, true},
		{"noise below the epsilon", [6]float32{1 + 1e-7, 0, 0, 1 + 1e-7, 1e-8, 0}, 1, 0, 0, false},
		{"no scale", [6]float32{0, 0, 0, 0, 5, 5}, 1, 0, 0, false},
	}
	for _, c := range cases {
		scale, panX, panY, moved := contentDelta(c.m)
		if moved != c.moved {
			t.Errorf("%s: moved = %v, want %v", c.name, moved, c.moved)
			continue
		}
		if moved && (scale != c.scale || panX != c.panX || panY != c.panY) {
			t.Errorf("%s: scale %v pan (%v, %v), want %v (%v, %v)", c.name, scale, panX, panY, c.scale, c.panX, c.panY)
		}
	}
}

// TestGestureStateOrder checks the phases a gesture goes through: it begins with
// its first change, continues through inertia, and ends when the viewport is
// ready, or when a new gesture interrupts the inertia.
func TestGestureStateOrder(t *testing.T) {
	var g gestureState
	if !g.motion() {
		t.Fatal("the first change did not begin a gesture")
	}
	if g.motion() {
		t.Error("a change within a gesture began another one")
	}
	if g.status(dmRunning, dmReady) {
		t.Error("a ready to running change ended the gesture")
	}
	if g.status(dmInertia, dmRunning) || !g.inertia {
		t.Error("the inertia phase did not end the gesture, and was not marked")
	}
	if g.motion() {
		t.Error("a change during inertia began a new gesture")
	}
	if !g.status(dmReady, dmInertia) || g.active || g.inertia {
		t.Error("the viewport became ready, and the gesture did not end")
	}
	if g.status(dmReady, dmReady) {
		t.Error("a ready status with no gesture ended one")
	}

	// A new gesture that interrupts inertia ends the old one, and then begins.
	g.motion()
	g.status(dmInertia, dmRunning)
	if !g.status(dmRunning, dmInertia) {
		t.Error("a new gesture during inertia did not end the old one")
	}
	if !g.motion() {
		t.Error("the new gesture did not begin")
	}
}

// TestDirectManipulationManagerClass checks the class id and the interface id
// of the manager: the class creates an object that answers for the interface,
// and the update manager that the object returns answers for its interface and
// updates. No window is made.
func TestDirectManipulationManagerClass(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	win32.OleInitialize()
	var mgr uintptr
	if hr := win32.CoCreateInstance(unsafe.Pointer(&clsidDMManager), unsafe.Pointer(&iidDMManager), &mgr); hresult(hr).failed() {
		t.Fatalf("CoCreateInstance(DirectManipulation manager) failed with HRESULT 0x%08X", uint32(hr))
	}
	defer release(mgr)
	var updates uintptr
	if err := comCall("GetUpdateManager", mgr, 7, uintptr(unsafe.Pointer(&iidDMUpdates)), uintptr(unsafe.Pointer(&updates))); err != nil {
		t.Fatal(err)
	}
	defer release(updates)
	if err := comCall("Update", updates, 5, 0); err != nil {
		t.Fatal(err)
	}
}
