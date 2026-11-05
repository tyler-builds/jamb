//go:build linux

package app

import (
	"errors"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/platform/linux"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/soft"
)

// On Linux, windows are Wayland surfaces and frames are drawn on the CPU (see
// render/soft) into shared memory. There is no GPU device to prepare or lose.

func runPlatform(start func() error) error { return linux.Run(start) }

func openPlatform(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	return linux.Open(opts, h)
}

func postUI(fn func()) { linux.Post(fn) }

func onUIThread() bool { return linux.OnUIThread() }

// releaseWindowSession lets go of the compositor's window session, for the
// build of jamb dev that takes this one's place (see platform/linux/session.go).
func releaseWindowSession() { linux.ReleaseSession() }

func quitUI() { linux.Quit() }

// setPlatformAppID makes the AppID the Wayland app ID of the windows, which the
// desktop matches to the app's .desktop file.
func setPlatformAppID(id string) { linux.SetAppID(id) }

func prepareDevice() {}

// softWindow is a window whose frames the CPU renderer draws.
type softWindow interface {
	Target() soft.Target
}

func newRenderer(w platform.Window, width, height int) (render.Renderer, error) {
	sw, ok := w.(softWindow)
	if !ok {
		return nil, errors.New("app: the window has no target for the CPU renderer")
	}
	return soft.New(sw.Target()), nil
}

// The CPU renderer never loses a device.
func releaseDevice() {}

func rebuildDevice() error { return nil }

func gpuAdapter() string { return "CPU" }

// readPlatformSettings reads the desktop's settings from the settings portal
// (see linux.ReadSettings).
func readPlatformSettings() platform.Settings { return linux.ReadSettings() }

// clockNow returns the monotonic clock that Event.Arrived uses.
func clockNow() int64 { return linux.Clock() }

// processMemory reports no memory counters on this platform yet.
func processMemory() (privateBytes, workingSet uint64) { return 0, 0 }
