package app

import (
	"testing"
	"time"
)

// A window that does not exist answers reads with zero values, also from
// another goroutine once the UI loop has ended, and actions do nothing.
func TestAppWindowWithoutAWindow(t *testing.T) {
	w := &appWindow{}
	endUI() // as after the loop ends: a read must not wait for it
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Focus()
		w.SetSize(10, 10)
		if x, y := w.Position(); x != 0 || y != 0 {
			t.Errorf("position %d,%d of a missing window", x, y)
		}
		if w.Maximized() || w.Fullscreen() || w.Closed() {
			t.Error("a window that was never opened reports a state")
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a read waited for a UI loop that has ended")
	}
	w.closed.Store(true)
	if !w.Closed() {
		t.Fatal("Closed is false for a closed window")
	}
}
