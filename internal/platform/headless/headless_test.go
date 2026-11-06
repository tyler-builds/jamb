package headless

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
)

// handler counts frames and events. It asks for frames until it has drawn
// want, then quits.
type handler struct {
	frames, want int
	events       []platform.EventKind
	created      bool
}

func (h *handler) OnCreate(w platform.Window) error { h.created = true; return nil }
func (h *handler) OnEvent(w platform.Window, e platform.Event) {
	h.events = append(h.events, e.Kind)
}
func (h *handler) OnFrame(w platform.Window) bool {
	h.frames++
	if h.frames >= h.want {
		Quit()
		return false
	}
	return true
}
func (h *handler) OnClose(w platform.Window) bool { return true }
func (h *handler) OnDestroy(w platform.Window)    {}

func TestOpenOutsideRun(t *testing.T) {
	if _, err := Open(platform.WindowOptions{Width: 10, Height: 10}, &handler{}); err == nil {
		t.Fatal("Open outside Run is not an error")
	}
}

func TestFramesArePacedAndTheWindowHasFocus(t *testing.T) {
	h := &handler{want: 6}
	start := time.Now()
	var w platform.Window
	err := Run(func() error {
		if !OnUIThread() {
			t.Error("start is not on the UI thread")
		}
		var err error
		w, err = Open(platform.WindowOptions{Width: 200, Height: 100}, h)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !h.created || h.frames != 6 {
		t.Fatalf("created %v, frames %d; want 6", h.created, h.frames)
	}
	// Five paced frames follow the first, at 60 per second.
	if d := time.Since(start); d < 5*frameInterval*8/10 {
		t.Errorf("6 frames took %v; they are not paced", d)
	}
	if len(h.events) == 0 || h.events[0] != platform.EventFocus {
		t.Errorf("events %v; want focus first", h.events)
	}
	if pw, ph := w.Size(); pw != 200 || ph != 100 {
		t.Errorf("size %d×%d", pw, ph)
	}
	if OnUIThread() {
		t.Error("OnUIThread after Run returned")
	}
}

func TestBufferAndClipboard(t *testing.T) {
	w := &window{}
	pix, stride, err := w.Buffer(4, 3)
	if err != nil || len(pix) != 48 || stride != 16 {
		t.Fatalf("Buffer = %d bytes, stride %d, %v", len(pix), stride, err)
	}
	if _, _, err := w.Buffer(0, 3); err == nil {
		t.Error("an empty buffer is not an error")
	}
	w.SetClipboard("hi")
	if s, ok := w.Clipboard(); !ok || s != "hi" {
		t.Errorf("clipboard %q, %v", s, ok)
	}
}
