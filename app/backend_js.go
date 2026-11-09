//go:build js

package app

import (
	"errors"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/platform/web"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/soft"
)

// In a js build, the window is a canvas of the web page (see platform/web),
// and the CPU renderer draws its frames.

func osRunPlatform(start func() error) error { return web.Run(start) }

func osOpenPlatform(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	return web.Open(opts, h)
}

func osPostUI(fn func()) { web.Post(fn) }

func osOnUIThread() bool { return web.OnUIThread() }

// releaseWindowSession does nothing: a page has no window session.
func osReleaseWindowSession() {}

func osQuitUI() { web.Quit() }

func osPrepareDevice() {}

func osNewRenderer(w platform.Window, width, height int) (render.Renderer, error) {
	sw, ok := w.(interface{ Target() soft.Target })
	if !ok {
		return nil, errors.New("app: the window has no target for the CPU renderer")
	}
	return soft.New(sw.Target()), nil
}

// The CPU renderer has no device to lose.
func osReleaseDevice() {}

func osRebuildDevice() error { return nil }

func osGpuAdapter() string { return "CPU (canvas)" }

func osReadPlatformSettings() platform.Settings { return web.ReadSettings() }

func osClockNow() int64 { return web.Clock() }

// processMemory reports no memory counters in a page.
func processMemory() (privateBytes, workingSet uint64) { return 0, 0 }

func setPlatformAppID(string) {}
