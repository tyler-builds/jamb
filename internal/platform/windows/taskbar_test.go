//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"reflect"
	"testing"

	"github.com/tyler-builds/jamb/internal/platform"
)

// fakeButton records the calls a taskbar button receives, in order. It does not
// touch the taskbar.
type fakeButton struct {
	calls   []string
	failAt  string // the call that fails, if any
	failErr error
}

func (f *fakeButton) record(call string) error {
	f.calls = append(f.calls, call)
	if call == f.failAt {
		return f.failErr
	}
	return nil
}

func (f *fakeButton) SetProgressState(flags uint32) error {
	return f.record(fmt.Sprintf("state:%d", flags))
}

func (f *fakeButton) SetProgressValue(completed, total uint64) error {
	return f.record(fmt.Sprintf("value:%d/%d", completed, total))
}

func (f *fakeButton) SetOverlayIcon(icon uintptr, description string) error {
	return f.record(fmt.Sprintf("overlay:%d:%s", icon, description))
}

// TestTaskbarApplyOrder checks that a window's state reaches a new button in
// order: the progress state, the value, then the overlay.
func TestTaskbarApplyOrder(t *testing.T) {
	s := taskbarState{progress: platform.TaskbarNormal, value: 0.4, overlay: 7, description: "3 unread"}
	var b fakeButton
	if err := s.apply(&b); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{"state:2", "value:4000/10000", "overlay:7:3 unread"}
	if !reflect.DeepEqual(b.calls, want) {
		t.Errorf("calls = %q; want %q", b.calls, want)
	}
}

// TestTaskbarApplyClearsWithNoState checks that a new button gets the empty state
// too, so no progress bar or badge is left over from the taskbar's default.
func TestTaskbarApplyClearsWithNoState(t *testing.T) {
	var s taskbarState
	var b fakeButton
	if err := s.apply(&b); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := []string{"state:0", "value:0/10000", "overlay:0:"}
	if !reflect.DeepEqual(b.calls, want) {
		t.Errorf("calls = %q; want %q", b.calls, want)
	}
}

// TestTaskbarApplyStopsAtFailure checks that a failed call ends the apply.
func TestTaskbarApplyStopsAtFailure(t *testing.T) {
	s := taskbarState{progress: platform.TaskbarError, value: 0.5, overlay: 3, description: "x"}
	b := fakeButton{failAt: "value:5000/10000", failErr: errors.New("refused")}
	if err := s.apply(&b); err == nil {
		t.Fatal("apply = nil; want the failure")
	}
	want := []string{"state:4", "value:5000/10000"}
	if !reflect.DeepEqual(b.calls, want) {
		t.Errorf("calls = %q; want %q", b.calls, want)
	}
}

func TestProgressFlags(t *testing.T) {
	for state, want := range map[platform.TaskbarState]uint32{
		platform.TaskbarNone:          0x0,
		platform.TaskbarIndeterminate: 0x1,
		platform.TaskbarNormal:        0x2,
		platform.TaskbarError:         0x4,
		platform.TaskbarPaused:        0x8,
		platform.TaskbarState(99):     0x0,
	} {
		if got := progressFlags(state); got != want {
			t.Errorf("progressFlags(%d) = %#x; want %#x", state, got, want)
		}
	}
}

func TestProgressValue(t *testing.T) {
	for _, tc := range []struct {
		v         float32
		completed uint64
	}{
		{0, 0},
		{0.25, 2500},
		{1, 10000},
		{-1, 0},
		{1.5, 10000},
		{float32(nan()), 0},
	} {
		completed, total := progressValue(tc.v)
		if completed != tc.completed || total != 10000 {
			t.Errorf("progressValue(%v) = %d/%d; want %d/10000", tc.v, completed, total, tc.completed)
		}
	}
}

// nan returns a NaN, which is not a constant expression in Go.
func nan() float64 {
	zero := 0.0
	return zero / zero
}

// TestIconSize checks the small icon size at each DPI: the system's metric when
// it has one, else 16 pixels at 96 DPI, scaled and never below 16.
func TestIconSize(t *testing.T) {
	system := func(metric int32) func(uint32) int32 {
		return func(uint32) int32 { return metric }
	}
	if got := iconSize(144, system(24)); got != 24 {
		t.Errorf("with the metric 24 at 144 DPI, size = %d; want 24", got)
	}
	for dpi, want := range map[uint32]int{96: 16, 120: 20, 144: 24, 168: 28, 192: 32, 0: 16} {
		if got := iconSize(dpi, system(0)); got != want {
			t.Errorf("without a metric at %d DPI, size = %d; want %d", dpi, got, want)
		}
	}
}

// TestIconPixels checks the byte order of the icon rows: blue, green, red, alpha.
func TestIconPixels(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	img.Set(1, 0, color.NRGBA{R: 40, G: 50, B: 60, A: 255})
	img.Set(0, 1, color.NRGBA{R: 70, G: 80, B: 90, A: 255})
	img.Set(1, 1, color.NRGBA{R: 100, G: 110, B: 120, A: 255})
	got := iconPixels(img, 2)
	want := []byte{
		30, 20, 10, 255, 60, 50, 40, 255,
		90, 80, 70, 255, 120, 110, 100, 255,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("iconPixels = %v; want %v", got, want)
	}
}
