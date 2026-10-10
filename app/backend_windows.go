//go:build windows

package app

import (
	"github.com/tyler-builds/jamb/platform"
	"github.com/tyler-builds/jamb/platform/windows"
	"github.com/tyler-builds/jamb/render"
	"github.com/tyler-builds/jamb/render/d3d11"
)

// gpu is the Direct3D 11 device that every window renders with. The first
// window creates it; later windows only get a swap chain.
var gpu *d3d11.Device

func runPlatform(start func() error) error { return windows.Run(start) }

func openPlatform(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	return windows.Open(opts, h)
}

func postUI(fn func()) { windows.Post(fn) }

func quitUI() { windows.Quit() }

// releaseDevice releases the shared device and its objects, as a removal
// leaves them; see simulateDeviceLoss.
func releaseDevice() {
	if gpu != nil {
		gpu.ReleaseDevice()
	}
}

// rebuildDevice recreates the shared device after a loss. Each window then
// recreates its swap chain; see rebuildGPU.
func rebuildDevice() error { return gpu.Recover() }

// gpuAdapter names the adapter the shared device runs on, for the recovery
// message: the hardware GPU, or WARP after a fallback.
func gpuAdapter() string {
	if gpu == nil {
		return ""
	}
	return gpu.Adapter()
}

func newRenderer(w platform.Window, width, height int) (render.Renderer, error) {
	if gpu == nil {
		dev, err := d3d11.NewDevice()
		if err != nil {
			return nil, err
		}
		gpu = dev
	}
	r, err := gpu.NewRenderer(w.Handle(), width, height)
	if err != nil {
		return nil, err
	}
	if loadDevOptions().noVSync {
		r.SetVSync(false)
	}
	return r, nil
}

// readPlatformSettings reads the operating system's accessibility and text
// settings (see platform.Settings).
func readPlatformSettings() platform.Settings { return windows.ReadSettings() }
