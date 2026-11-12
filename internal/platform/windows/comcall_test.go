//go:build windows

package windows

import (
	"os"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/win32"
)

// stackMoverValue is what the stack mover's method writes through its out pointer.
const stackMoverValue = 0x5EED0C0FFEE

// The stack mover is a fake COM object whose method 3, (this, out), grows the calling
// goroutine's stack before it writes through out, as a native method does that calls
// back into Go (a toast event source calling the handler's AddRef, or the COM modal
// loop dispatching a window message) before it writes its result. The vtable's
// IUnknown slots are never called.
var (
	stackMoverVtbl [4]uintptr
	stackMoverOnce sync.Once
)

// growStack uses about depth times 4 KB of stack, more than a new goroutine has, so
// the goroutine's stack is copied to a larger one.
//
//go:noinline
func growStack(depth int) byte {
	var pad [4096]byte
	pad[depth%len(pad)] = byte(depth)
	if depth == 0 {
		return pad[0]
	}
	return growStack(depth-1) + pad[(depth*7)%len(pad)]
}

func stackMoverWrite(this, out uintptr) uintptr {
	growStack(256)
	*(*uintptr)(ptr(out)) = stackMoverValue
	return 0
}

// TestComCallOutParamSurvivesStackMove checks that comCall keeps an out parameter off
// the goroutine's stack. The method moves the stack before it writes; an out pointer
// into the old stack would be written in freed memory, and the caller would read zero.
// This is how the toast calls lost their results and wrote COM pointers into memory
// that Go had handed to something else.
func TestComCallOutParamSurvivesStackMove(t *testing.T) {
	stackMoverOnce.Do(func() { stackMoverVtbl[3] = syscall.NewCallback(stackMoverWrite) })
	view := allocView(stackMoverVtbl[:], 0)
	if view == 0 {
		t.Fatal("allocView returned 0")
	}
	defer win32.HeapFree(win32.GetProcessHeap(), 0, view)
	for i := 0; i < 8; i++ {
		// A new goroutine starts with the smallest stack, so the method's growth
		// always copies it.
		got := make(chan uintptr, 1)
		go func() {
			var out uintptr
			if err := comCall("Write", view, 3, uintptr(unsafe.Pointer(&out))); err != nil {
				t.Error(err)
			}
			got <- out
		}()
		if out := <-got; out != stackMoverValue {
			t.Fatalf("call %d: the out parameter is 0x%X after the stack moved, want 0x%X", i, out, uint64(stackMoverValue))
		}
	}
}

// toastTestXML is a plain toast, which a test shows under toastTestAppID.
const (
	toastTestXML   = `<toast><visual><binding template="ToastGeneric"><text>jamb test</text><text>ShowToast regression test</text></binding></visual></toast>`
	toastTestAppID = "Jamb.Tests.Notify"
)

// TestShowToastRepeatedly shows more toasts than shownKeep, with a garbage collection
// between them, on a thread with COM as the UI thread has it. Each call gets and
// releases its own activation factories, and the oldest kept toasts are released.
// Windows may refuse toasts of an AppID with no Start menu shortcut, so an error is
// logged, and only a crash fails the test.
func TestShowToastRepeatedly(t *testing.T) {
	if os.Getenv("JAMB_LIVE_TESTS") != "1" {
		t.Skip("shows real notifications; set JAMB_LIVE_TESTS=1 to run it")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	InitializeCOM()
	defer func() {
		for _, toast := range shownToasts {
			release(toast)
		}
		shownToasts = nil
	}()
	for i := 0; i < shownKeep+8; i++ {
		if err := ShowToast(toastTestAppID, toastTestXML, "jamb-test"); err != nil {
			t.Logf("ShowToast %d: %v", i, err)
		}
		runtime.GC()
	}
	if len(shownToasts) > shownKeep {
		t.Errorf("%d toasts are kept, want at most %d", len(shownToasts), shownKeep)
	}
	// The history needs the process's own identity, which a test process may lack.
	if err := RemoveToast("jamb-test"); err != nil {
		t.Logf("RemoveToast: %v", err)
	}
}
