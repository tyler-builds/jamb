//go:build js

package app

import (
	"errors"
	"fmt"
	"os"
	"syscall/js"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/platform/web"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/soft"
	"github.com/tyler-builds/jamb/internal/render/webgl"
)

// In a js build, the window is a canvas of the web page (see platform/web).
// WebGL 2 draws its frames, or the CPU renderer when the browser has no WebGL
// 2 or JAMB_RENDERER=cpu asks for it (see web.Getenv).

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
	if web.Getenv("JAMB_RENDERER") != "cpu" {
		if cw, ok := w.(interface{ Canvas() js.Value }); ok {
			r, err := webgl.New(cw.Canvas())
			if err == nil {
				adapter = r.Describe()
				return r, nil
			}
			fmt.Fprintf(os.Stderr, "jamb: drawing on the CPU: %v\n", err)
		}
	}
	sw, ok := w.(interface{ Target() soft.Target })
	if !ok {
		return nil, errors.New("app: the window has no target for the CPU renderer")
	}
	adapter = "CPU (canvas)"
	return soft.New(sw.Target()), nil
}

// adapter names what draws the window, for osGpuAdapter.
var adapter = "CPU (canvas)"

// A WebGL context that was lost is restored by the browser, and the renderer
// makes its resources again (see webgl.Renderer.Recreate): there is no device
// to release or rebuild.
func osReleaseDevice() {}

func osRebuildDevice() error { return nil }

func osGpuAdapter() string { return adapter }

func osReadPlatformSettings() platform.Settings { return web.ReadSettings() }

func osClockNow() int64 { return web.Clock() }

// processMemory reports no memory counters in a page.
func processMemory() (privateBytes, workingSet uint64) { return 0, 0 }

func setPlatformAppID(string) {}
