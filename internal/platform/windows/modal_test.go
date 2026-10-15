//go:build windows

package windows

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// queueHandler runs the functions queued for it in OnFrame, as the runtime runs
// posted functions on a frame. It has no swap chain, so the loop renders it at
// once.
type queueHandler struct {
	mu    sync.Mutex
	queue []func()
}

func (h *queueHandler) OnCreate(platform.Window) error          { return nil }
func (h *queueHandler) OnEvent(platform.Window, platform.Event) {}
func (h *queueHandler) OnClose(platform.Window) bool            { return true }
func (h *queueHandler) OnDestroy(platform.Window)               { Quit() }

func (h *queueHandler) OnFrame(platform.Window) bool {
	h.mu.Lock()
	fns := h.queue
	h.queue = nil
	h.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
	return false
}

// post queues fn for the next frame of w, from any goroutine.
func (h *queueHandler) post(w platform.Window, fn func()) {
	h.mu.Lock()
	h.queue = append(h.queue, fn)
	h.mu.Unlock()
	w.Wake()
}

// The system menu that F10 or a lone Alt opens in a window without a menu bar
// runs its own message loop, as a dialog does. A function posted to the window
// from another goroutine must still run while that loop is open, on a frame.
func TestPostRunsDuringNativeMenuLoop(t *testing.T) {
	if os.Getenv("JAMB_LIVE_TESTS") != "1" {
		t.Skip("opens a real window and its system menu; set JAMB_LIVE_TESTS=1 to run it")
	}
	h := &queueHandler{}
	var inLoop atomic.Bool
	ran := make(chan bool, 1)      // whether the posted function ran while the loop was open
	result := make(chan string, 1) // what the driver found; always sent before the window closes
	err := Run(func() error {
		w, err := Open(platform.WindowOptions{Title: "menu loop", Width: 200, Height: 120}, h)
		if err != nil {
			return err
		}
		Post(func() {
			inLoop.Store(true)
			// A paint pending from Show would render the window inside the loop,
			// so the test clears it: only the wake-up can draw a frame there.
			win32.ValidateRect(w.Handle())
			// The menu loop runs here until it is cancelled.
			win32.DefWindowProc(w.Handle(), win32.WM_SYSCOMMAND, win32.SC_KEYMENU, 0)
			inLoop.Store(false)
		})
		go func() {
			deadline := time.Now().Add(5 * time.Second)
			for !inLoop.Load() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			h.post(w, func() { ran <- inLoop.Load() })
			outcome := ""
			select {
			case inside := <-ran:
				if !inside {
					outcome = "the posted function ran after the menu loop, not during it"
				}
			case <-time.After(2 * time.Second):
				outcome = "the posted function did not run while the menu loop was open"
			}
			result <- outcome
			win32.PostMessage(w.Handle(), win32.WM_CANCELMODE, 0, 0)
			deadline = time.Now().Add(5 * time.Second)
			for inLoop.Load() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			w.Close()
		}()
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case got := <-result:
		if got != "" {
			t.Fatal(got)
		}
	default:
		t.Fatal("the window did not run its test")
	}
}
