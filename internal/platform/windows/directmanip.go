//go:build windows

package windows

import (
	"fmt"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// Touchpad and touchscreen pinch, through DirectManipulation (DM).
//
// DM is the system's recognizer of pinch and pan gestures. A window receives
// DM_POINTERHITTEST for a touchpad or touchscreen contact, hands the contact to
// its viewport with SetContact, and the viewport reports the gesture as changes
// of a content transform. The viewport is configured for scaling only. The pan
// stays with what Windows already sends: a touchpad's two-finger scroll arrives
// as wheel messages, which the window scrolls with, and DM's pan would run beside
// them and scroll twice. A single finger on a touchscreen is a mouse drag, which
// DM's pan would take over, so pan is not enabled for touch either. Scaling needs
// two contacts, so a single finger is left to the mouse. Both points are design
// intent that the touchpad and touchscreen have not yet confirmed.
//
// DM is pulled, not pushed. While contacts are active the window calls the update
// manager's Update from a timer, and DM reports what the contacts did from inside
// that call, on the UI thread. The content transform is reset after each report,
// so each report is a change, not the total of the gesture.

// The DirectManipulation constants, from directmanipulation.h, and the
// pointer types, from winuser.h.
const (
	dmPointerHitTest = 0x0250 // DM_POINTERHITTEST: the low word of wParam is the contact's pointer id
	dmTimerID        = 3      // the timer that calls Update (1 and 2 are taken, see services.go and dialog.go)
	dmTimerMS        = 16     // about one frame
	// dmIdleTicks is how many ticks the timer keeps running with the viewport
	// ready, after the last contact or change. A contact that rests longer than
	// this and then moves, with no new contact, may not be followed, until the
	// next contact restarts the timer.
	dmIdleTicks = 60

	dmRunning = 3 // DIRECTMANIPULATION_RUNNING
	dmInertia = 4 // DIRECTMANIPULATION_INERTIA
	dmReady   = 5 // DIRECTMANIPULATION_READY

	dmConfiguration   = 0x1 | 0x10 | 0x80 // INTERACTION | SCALING | SCALING_INERTIA
	dmViewportOptions = 0x2 | 0x10        // MANUALUPDATE | DISABLEPIXELSNAPPING
	dmViewportSize    = 1000              // the content is never drawn, so only its scale matters

	ptTouch    = 2 // PT_TOUCH
	ptTouchpad = 5 // PT_TOUCHPAD

	dmENoInterface = 0x80004002 // E_NOINTERFACE
	// dmEpsilon is the change below which a content transform is unchanged.
	dmEpsilon = 1e-4
)

// The class and interface ids. The class is the one that directmanipulation.dll
// registers for the manager; TestDirectManipulationManagerClass checks the pair.
var (
	clsidDMManager = guid{0x54e211b6, 0x3650, 0x4f75, [8]byte{0x83, 0x34, 0xfa, 0x35, 0x95, 0x98, 0xe1, 0xc5}}
	iidDMManager   = guid{0xFBF5D3B4, 0x70C7, 0x4163, [8]byte{0x93, 0x22, 0x5A, 0x6F, 0x66, 0x0D, 0x6F, 0xBC}}
	iidDMUpdates   = guid{0xB0AE62FD, 0xBE34, 0x46E7, [8]byte{0x9C, 0xAA, 0xD3, 0x61, 0xFA, 0xCB, 0xB9, 0xCC}}
	iidDMViewport  = guid{0x28b85a3d, 0x60a0, 0x48bd, [8]byte{0x9b, 0xa1, 0x5c, 0xe8, 0xd9, 0xea, 0x3a, 0x6d}}
	iidDMHandler   = guid{0x952121DA, 0xD69F, 0x45F9, [8]byte{0xB0, 0xF9, 0xF2, 0x39, 0x44, 0x32, 0x1A, 0x6D}}
)

var (
	dmUser32           = syscall.NewLazyDLL("user32.dll")
	procGetPointerType = dmUser32.NewProc("GetPointerType")
	procGetCursorPos   = dmUser32.NewProc("GetCursorPos")
)

// dmIdentity is the content transform with nothing applied. It is read only, so
// it can be passed to DM at any time.
var dmIdentity = [6]float32{1, 0, 0, 1, 0, 0}

// dmHandlerVtbl is the vtable of the viewport's event handler object: IUnknown,
// then IDirectManipulationViewportEventHandler. dmVtblOnce fills it.
var (
	dmHandlerVtbl [6]uintptr
	dmVtblOnce    sync.Once
	// dmByHandle maps the handle of each handler object to its window's state.
	// It is only touched on the UI thread, as DM calls the handler there.
	dmByHandle   = map[uintptr]*directManipulation{}
	dmLastHandle uintptr
)

// directManipulation is the DirectManipulation state of one window. The
// objects are COM objects called through their vtables, as the UI Automation
// providers and the file dialogs are.
type directManipulation struct {
	w        *window
	manager  uintptr // IDirectManipulationManager
	updates  uintptr // IDirectManipulationUpdateManager
	viewport uintptr // IDirectManipulationViewport
	handler  uintptr // the event handler object, a view made by allocView
	handle   uintptr // the handler object's key in dmByHandle
	cookie   uint32  // the handler's cookie, for RemoveEventHandler
	rect     win32.RECT
	// matrix is what GetContentTransform fills. The fields are kept here, not on
	// the stack, because DM may call back into Go while it holds their address.
	matrix  [6]float32
	gesture gestureState
	status  uint32 // the viewport's status, as last reported
	timer   bool   // the Update timer runs
	idle    int    // ticks since the last contact or change, see dmIdleTicks
	syncing bool   // the content transform is being reset, and a reset reports again
}

// newDirectManipulation sets DM up for the window, which must exist and be
// ready for events. It returns nil when the system has no DM (before Windows
// 8.1) or the setup fails. The window then keeps the Ctrl+wheel pinch of a
// touchpad, which the wheel path already handles.
func newDirectManipulation(w *window) *directManipulation {
	dmVtblOnce.Do(initDMVtbls)
	d := &directManipulation{w: w, status: dmReady}
	if err := d.create(); err != nil {
		d.destroy()
		return nil
	}
	return d
}

// create makes the manager, the viewport and the event handler, in the order
// DM requires. Each failing call stops the setup, and the error names the call.
func (d *directManipulation) create() error {
	var err error
	step := func(what string, obj uintptr, i int, args ...uintptr) {
		if err == nil {
			err = comCall(what, obj, i, args...)
		}
	}
	hwnd := d.w.hwnd
	if hr := win32.CoCreateInstance(unsafe.Pointer(&clsidDMManager), unsafe.Pointer(&iidDMManager), &d.manager); hresult(hr).failed() {
		return fmt.Errorf("platform/windows: CoCreateInstance(DirectManipulation manager) failed with HRESULT 0x%08X", uint32(hr))
	}
	step("GetUpdateManager", d.manager, 7, uintptr(unsafe.Pointer(&iidDMUpdates)), uintptr(unsafe.Pointer(&d.updates)))
	step("CreateViewport", d.manager, 8, 0, hwnd, uintptr(unsafe.Pointer(&iidDMViewport)), uintptr(unsafe.Pointer(&d.viewport)))
	step("ActivateConfiguration", d.viewport, 22, dmConfiguration)
	step("SetViewportOptions", d.viewport, 19, dmViewportOptions)
	d.rect = win32.RECT{Right: dmViewportSize, Bottom: dmViewportSize}
	step("SetViewportRect", d.viewport, 12, uintptr(unsafe.Pointer(&d.rect)))
	step("Activate", d.manager, 3, hwnd)
	step("Enable", d.viewport, 3)
	dmLastHandle++
	d.handle = dmLastHandle
	dmByHandle[d.handle] = d
	d.handler = allocView(dmHandlerVtbl[:], d.handle)
	if d.handler == 0 && err == nil {
		err = fmt.Errorf("platform/windows: no memory for the DirectManipulation event handler")
	}
	step("AddEventHandler", d.viewport, 25, hwnd, d.handler, uintptr(unsafe.Pointer(&d.cookie)))
	step("Update", d.updates, 5, 0)
	return err
}

// destroy stops the viewport, and releases the objects in the order DM requires.
// The window is still valid. It does nothing for a nil state.
func (d *directManipulation) destroy() {
	if d == nil {
		return
	}
	d.stopTimer()
	// Teardown errors leave nothing to undo, so the calls' results are not kept.
	if d.viewport != 0 {
		_ = comCall("Stop", d.viewport, 29)
		if d.handler != 0 {
			_ = comCall("RemoveEventHandler", d.viewport, 26, uintptr(d.cookie))
		}
		_ = comCall("Abandon", d.viewport, 30)
	}
	if d.manager != 0 {
		_ = comCall("Deactivate", d.manager, 4, d.w.hwnd)
	}
	release(d.viewport)
	release(d.updates)
	release(d.manager)
	if d.handler != 0 {
		win32.HeapFree(win32.GetProcessHeap(), 0, d.handler)
	}
	delete(dmByHandle, d.handle)
	d.viewport, d.updates, d.manager, d.handler = 0, 0, 0, 0
}

// hitTest gives a touchpad or touchscreen contact to the viewport, which starts
// the gesture recognition for it. Other pointer types are left to the window.
func (d *directManipulation) hitTest(pointerID uint32) {
	if d == nil {
		return
	}
	var t uint32
	if r, _, _ := procGetPointerType.Call(uintptr(pointerID), uintptr(unsafe.Pointer(&t))); r == 0 || (t != ptTouch && t != ptTouchpad) {
		return
	}
	// A failed SetContact leaves the contact to the window, which is no loss.
	_ = comCall("SetContact", d.viewport, 5, uintptr(pointerID))
	d.idle = 0
	if !d.timer {
		win32.SetTimer(d.w.hwnd, dmTimerID, dmTimerMS)
		d.timer = true
	}
}

// stopTimer stops the timer that calls Update, if it runs.
func (d *directManipulation) stopTimer() {
	if d.timer {
		win32.KillTimer(d.w.hwnd, dmTimerID)
		d.timer = false
	}
}

// onDMTimer runs on WM_TIMER for dmTimerID. It calls Update, which reports what
// the contacts did, and stops the timer once the viewport has been ready for
// dmIdleTicks ticks.
func (w *window) onDMTimer() {
	d := w.dm
	if d == nil {
		win32.KillTimer(w.hwnd, dmTimerID)
		return
	}
	_ = comCall("Update", d.updates, 5, 0)
	if d.status != dmReady {
		d.idle = 0
		return
	}
	d.idle++
	if d.idle > dmIdleTicks {
		d.stopTimer()
	}
}

// statusChanged records a change of the viewport's status, and ends the gesture
// under way when the status says it has ended.
func (d *directManipulation) statusChanged(cur, prev uint32) {
	d.status = cur
	if d.gesture.status(cur, prev) {
		d.emit(platform.GestureEnd, 1, 0, 0)
	}
	if cur != dmReady {
		d.idle = 0
	}
}

// contentUpdated reads a change of the content transform, resets the transform
// so that the next change is reported on its own, and reports the change. A
// reset reports again, so the callback is not re-entered while it runs.
func (d *directManipulation) contentUpdated(content uintptr) {
	if d.syncing {
		return
	}
	if err := comCall("GetContentTransform", content, 9, uintptr(unsafe.Pointer(&d.matrix[0])), 6); err != nil {
		return
	}
	m := d.matrix
	if m != dmIdentity {
		d.syncing = true
		_ = comCall("SyncContentTransform", content, 10, uintptr(unsafe.Pointer(&dmIdentity[0])), 6)
		d.syncing = false
	}
	scale, panX, panY, moved := contentDelta(m)
	if !moved {
		return
	}
	d.idle = 0
	if d.gesture.motion() {
		d.emit(platform.GestureBegin, 1, 0, 0)
	}
	d.emit(platform.GestureUpdate, scale, panX, panY)
}

// emit reports a phase of the gesture to the window. DM does not report where
// the gesture is, so the center is the pointer's position.
func (d *directManipulation) emit(phase platform.GesturePhase, scale, panX, panY float32) {
	var p win32.POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	win32.ScreenToClient(d.w.hwnd, &p)
	d.w.emit(platform.Event{Kind: platform.EventGesture, X: float32(p.X), Y: float32(p.Y),
		Phase: phase, Scale: scale, PanX: panX, PanY: panY, Inertia: d.gesture.inertia})
}

// pinching reports whether a pinch is under way. The window drops the Ctrl+wheel
// messages that a touchpad sends for the same pinch, so that it zooms once.
func (d *directManipulation) pinching() bool {
	return d != nil && d.gesture.active
}

// contentDelta returns the change that a content transform m holds: its scale,
// and its translation in physical pixels. DM lays a transform out as
// [scale 0 0 scale tx ty]. moved is false when the change is below dmEpsilon, or
// when the transform has no scale.
func contentDelta(m [6]float32) (scale, panX, panY float32, moved bool) {
	if !(m[0] > 0) {
		return 1, 0, 0, false
	}
	scale, panX, panY = m[0], m[4], m[5]
	moved = math.Abs(float64(scale-1)) > dmEpsilon || math.Abs(float64(panX)) > dmEpsilon || math.Abs(float64(panY)) > dmEpsilon
	return scale, panX, panY, moved
}

// gestureState is the gesture that DM reports. A gesture begins with its first
// change, and ends when the viewport is ready again, or when a new gesture
// interrupts the inertia of the previous one.
type gestureState struct {
	active  bool // a gesture has begun and has not ended
	inertia bool // the viewport is in its inertia phase
}

// status records a change of the viewport's status from prev to cur, and reports
// whether the gesture under way ends.
func (g *gestureState) status(cur, prev uint32) (end bool) {
	g.inertia = cur == dmInertia
	end = g.active && (cur == dmReady || (cur == dmRunning && prev == dmInertia))
	g.active = g.active && !end
	return end
}

// motion records a change of the content, and reports whether it begins a gesture.
func (g *gestureState) motion() (begin bool) {
	begin = !g.active
	g.active = true
	return begin
}

// initDMVtbls makes the callbacks of the handler object and fills its vtable. It
// runs once, as syscall.NewCallback has a process-wide limit.
func initDMVtbls() {
	dmHandlerVtbl = [6]uintptr{
		syscall.NewCallback(dmQueryInterface),
		syscall.NewCallback(dmAddRef),
		syscall.NewCallback(dmRelease),
		syscall.NewCallback(dmStatusChanged),
		syscall.NewCallback(dmViewportUpdated),
		syscall.NewCallback(dmContentUpdated),
	}
}

// dmOf returns the state of the window whose handler object this is, or nil.
func dmOf(this uintptr) *directManipulation {
	return dmByHandle[*(*uintptr)(ptr(this + wordSize))]
}

// dmQueryInterface answers for IUnknown and the event handler interface. The
// object's lifetime is the window's, so AddRef and Release keep no count.
func dmQueryInterface(this, riid, out uintptr) uintptr {
	iid := *(*guid)(ptr(riid))
	if iid != iidIUnknown && iid != iidDMHandler {
		*(*uintptr)(ptr(out)) = 0
		return dmENoInterface
	}
	*(*uintptr)(ptr(out)) = this
	return 0
}

func dmAddRef(this uintptr) uintptr  { return 1 }
func dmRelease(this uintptr) uintptr { return 1 }

// dmStatusChanged is OnViewportStatusChanged.
func dmStatusChanged(this, viewport, current, previous uintptr) uintptr {
	if d := dmOf(this); d != nil {
		d.statusChanged(uint32(current), uint32(previous))
	}
	return 0
}

// dmViewportUpdated is OnViewportUpdated, which the window does not need.
func dmViewportUpdated(this, viewport uintptr) uintptr { return 0 }

// dmContentUpdated is OnContentUpdated.
func dmContentUpdated(this, viewport, content uintptr) uintptr {
	if d := dmOf(this); d != nil {
		d.contentUpdated(content)
	}
	return 0
}
