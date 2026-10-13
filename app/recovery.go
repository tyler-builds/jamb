package app

import (
	"fmt"
	"os"
	"time"

	"github.com/tyler-builds/jamb/internal/render"
)

// The GPU device is shared by every window. When it is lost, the first window
// to draw afterwards rebuilds it, then every window's swap chain, and then all
// windows draw again. The rebuild happens once, however many windows there are.
// While the device is unavailable, a failed rebuild is retried after a delay
// that doubles up to a cap, instead of on every frame. The ui runtime of each
// window is never touched, so components, signals and text fields keep their
// state.

const (
	rebuildDelay    = 250 * time.Millisecond // wait after the first failed rebuild
	rebuildDelayMax = 5 * time.Second        // cap on the wait after later failures
)

// errSimulatedLoss is what the devicelost script step reports, as if the next
// Present had failed.
var errSimulatedLoss = fmt.Errorf("%w (simulated by JAMB_SCRIPT devicelost)", render.ErrDeviceLost)

// gpuRecovery is the state of the shared GPU device after a loss. It is used
// only on the UI thread.
type gpuRecovery struct {
	lost     bool
	retryAt  time.Time       // the earliest time the next rebuild may run
	failures int             // rebuilds that failed since the loss
	reported map[string]bool // failure messages already printed for this loss
	// logf receives the recovery messages; nil prints them to stderr.
	logf func(format string, args ...any)
	// adapter names the adapter the rebuilt device runs on, for the recovery
	// message; nil leaves the name out.
	adapter func() string
}

// lose marks the device as lost at now. It reports whether the device was not
// already lost, so the caller wakes every window once.
func (g *gpuRecovery) lose(now time.Time, err error) bool {
	if g.lost {
		return false
	}
	g.lost, g.retryAt, g.failures, g.reported = true, now, 0, map[string]bool{}
	g.print("%v; rebuilding the device and every window", err)
	return true
}

// poll is called by a window before it draws. While the device is lost and the
// retry time has come, it rebuilds the device with rebuild. ready reports
// whether the window may draw now. recovered is true on the call that rebuilt
// the device, so the caller wakes every window. Otherwise retryAt is when the
// window should be woken to try again.
func (g *gpuRecovery) poll(now time.Time, rebuild func() error) (ready, recovered bool, retryAt time.Time) {
	if !g.lost {
		return true, false, time.Time{}
	}
	if now.Before(g.retryAt) {
		return false, false, g.retryAt
	}
	if err := rebuild(); err != nil {
		g.failures++
		g.retryAt = now.Add(rebuildDelayAfter(g.failures))
		// Each distinct failure is printed once per loss, not on every retry.
		if msg := err.Error(); !g.reported[msg] {
			g.reported[msg] = true
			g.print("GPU device not available yet, retrying: %v", err)
		}
		return false, false, g.retryAt
	}
	g.lost, g.failures, g.reported = false, 0, nil
	msg := "GPU device recovered"
	if g.adapter != nil {
		msg += " on " + g.adapter()
	}
	g.print("%s; redrawing every window", msg)
	return true, true, time.Time{}
}

// rebuildDelayAfter is the wait after the n-th failed rebuild: rebuildDelay,
// doubled after each further failure, up to rebuildDelayMax.
func rebuildDelayAfter(n int) time.Duration {
	d := rebuildDelay
	for i := 1; i < n && d < rebuildDelayMax; i++ {
		d *= 2
	}
	return min(d, rebuildDelayMax)
}

func (g *gpuRecovery) print(format string, args ...any) {
	if g.logf != nil {
		g.logf(format, args...)
		return
	}
	fmt.Fprintf(os.Stderr, "jamb: "+format+"\n", args...)
}

// loseGPU handles a device loss reported by a frame. It marks the device lost
// and wakes every window, so the next one to draw rebuilds it.
func loseGPU(err error) {
	if application.gpu.lose(time.Now(), err) {
		wakeWindows()
	}
}

// simulateDeviceLoss is the devicelost script step. It releases the device for
// real, so the rebuild creates it again, and handles the loss as if the next
// frame's Present had reported DXGI_ERROR_DEVICE_REMOVED.
func simulateDeviceLoss() {
	releaseDevice()
	loseGPU(errSimulatedLoss)
}

// gpuReady is called by a window before it draws. It rebuilds a lost device
// when a rebuild is due, and it reports whether the window may draw now. A
// window that may not draw is woken at the next retry time.
func (h *host) gpuReady(now time.Time) bool {
	ready, recovered, retryAt := application.gpu.poll(now, rebuildGPU)
	if recovered {
		wakeWindows()
	}
	if !ready {
		h.win.WakeAt(retryAt)
	}
	return ready
}

// rebuildGPU recreates the shared device, then the swap chain of every open
// window. A failure leaves the rest for the next attempt, which starts over.
func rebuildGPU() error {
	if err := rebuildDevice(); err != nil {
		return err
	}
	for _, h := range application.windows {
		if rb, ok := h.r.(interface{ Recreate() error }); ok {
			if err := rb.Recreate(); err != nil {
				return err
			}
		}
	}
	return nil
}

// wakeWindows asks every open window for a frame.
func wakeWindows() {
	for _, h := range application.windows {
		h.win.Wake()
	}
}
