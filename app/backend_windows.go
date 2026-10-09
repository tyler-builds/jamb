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
