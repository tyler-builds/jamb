//go:build !windows

package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render"
)

func runPlatform(start func() error) error {
	return fmt.Errorf("the %s platform adapter is not implemented yet; this experiment currently runs on Windows only "+
		"(see platform/%s for what an adapter must provide)", runtime.GOOS, runtime.GOOS)
}

func openPlatform(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	return nil, fmt.Errorf("no window adapter for %s yet", runtime.GOOS)
}

func postUI(fn func()) {}

func onUIThread() bool { return false }

func quitUI() {}

func prepareDevice() {}

func newRenderer(w platform.Window, width, height int) (render.Renderer, error) {
	return nil, fmt.Errorf("no GPU backend for %s yet", runtime.GOOS)
}

// No backend reports a lost device, so these are never reached.
func releaseDevice() {}

func rebuildDevice() error { return fmt.Errorf("no GPU backend for %s yet", runtime.GOOS) }

func gpuAdapter() string { return "" }

// readPlatformSettings reports the defaults: no platform adapter reads the
// settings yet.
func readPlatformSettings() platform.Settings { return platform.DefaultSettings() }

// clockNow returns the time since the process started, in nanoseconds, for frame
// statistics and input latency.
func clockNow() int64 { return int64(time.Since(processStart)) }

// processMemory reports no memory counters on this platform yet.
func processMemory() (privateBytes, workingSet uint64) { return 0, 0 }
