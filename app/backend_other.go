//go:build !windows && !linux && !js

package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render"
)

func osRunPlatform(start func() error) error {
	return fmt.Errorf("the %s platform adapter is not implemented yet; this experiment currently runs on Windows only "+
		"(see platform/%s for what an adapter must provide)", runtime.GOOS, runtime.GOOS)
}

func osOpenPlatform(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	return nil, fmt.Errorf("no window adapter for %s yet", runtime.GOOS)
}

func osPostUI(fn func()) {}

// releaseWindowSession does nothing: windows are placed by their geometry here.
func osReleaseWindowSession() {}

func osOnUIThread() bool { return false }

func osQuitUI() {}

func osPrepareDevice() {}

func osNewRenderer(w platform.Window, width, height int) (render.Renderer, error) {
	return nil, fmt.Errorf("no GPU backend for %s yet", runtime.GOOS)
}

// No backend reports a lost device, so these are never reached.
func osReleaseDevice() {}

func osRebuildDevice() error { return fmt.Errorf("no GPU backend for %s yet", runtime.GOOS) }

func osGpuAdapter() string { return "" }

// readPlatformSettings reports the defaults: no platform adapter reads the
// settings yet.
func osReadPlatformSettings() platform.Settings { return platform.DefaultSettings() }

// clockNow returns the time since the process started, in nanoseconds, for frame
// statistics and input latency.
func osClockNow() int64 { return int64(time.Since(processStart)) }

// processMemory reports no memory counters on this platform yet.
func processMemory() (privateBytes, workingSet uint64) { return 0, 0 }

func setPlatformAppID(string) {}
