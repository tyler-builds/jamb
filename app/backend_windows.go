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

// gpuPending delivers the device that prepareDevice started creating, until
// the first window takes it.
var gpuPending chan gpuResult

type gpuResult struct {
	dev *d3d11.Device
	err error
}

// prepareDevice starts creating the shared device on another goroutine.
// Creating it loads the graphics driver, which takes longer than anything
// else at startup, so it overlaps with creating the first window and parsing
// fonts. A Direct3D 11 device may be created on any thread.
func prepareDevice() {
	gpuPending = make(chan gpuResult, 1)
	go func() {
		dev, err := d3d11.NewDevice()
		gpuPending <- gpuResult{dev, err}
	}()
}

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
		var res gpuResult
		if gpuPending != nil {
			res = <-gpuPending
			gpuPending = nil
		} else {
			res.dev, res.err = d3d11.NewDevice()
		}
		if res.err != nil {
			return nil, res.err
		}
		gpu = res.dev
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
