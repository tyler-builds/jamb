package app

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"strings"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/ui"
)

// nativeState returns the platform's state for a taskbar progress state. An
// unknown state shows nothing.
func nativeState(s ui.ProgressState) platform.TaskbarState {
	switch s {
	case ui.TaskbarNormal:
		return platform.TaskbarNormal
	case ui.TaskbarIndeterminate:
		return platform.TaskbarIndeterminate
	case ui.TaskbarPaused:
		return platform.TaskbarPaused
	case ui.TaskbarError:
		return platform.TaskbarError
	}
	return platform.TaskbarNone
}

// clampProgress returns v limited to the range from 0 to 1. NaN is 0.
func clampProgress(v float64) float64 {
	switch {
	case math.IsNaN(v) || v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}

// checkBadge reports a badge that cannot be shown: one with an image and no
// description. A nil image removes the badge, and needs none.
func checkBadge(img image.Image, description string) error {
	if img != nil && strings.TrimSpace(description) == "" {
		return errors.New("SetTaskbarBadge: a badge needs a description, which assistive technology reads")
	}
	return nil
}

// SetTaskbarProgress implements ui.Window. It is safe from any goroutine.
func (w *appWindow) SetTaskbarProgress(p ui.TaskbarProgress) {
	state, value := nativeState(p.State), float32(clampProgress(p.Value))
	w.do(func(h *host) { h.win.SetTaskbarProgress(state, value) })
}

// SetTaskbarBadge implements ui.Window. It is safe from any goroutine.
func (w *appWindow) SetTaskbarBadge(img image.Image, description string) {
	if err := checkBadge(img, description); err != nil {
		fmt.Fprintln(os.Stderr, "jamb:", err)
		return
	}
	w.do(func(h *host) {
		if err := h.win.SetTaskbarOverlay(img, description); err != nil {
			fmt.Fprintln(os.Stderr, "jamb: SetTaskbarBadge:", err)
		}
	})
}
