//go:build linux && (amd64 || arm64)

package app

import (
	"fmt"
	"os"

	"github.com/tyler-builds/jamb/internal/dmabuf"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/vulkan"
)

// gpu is the Vulkan device that every window renders with, or nil when there
// is none: windows are then drawn on the CPU. The first window creates it.
var gpu *vulkan.Device

// gpuPending delivers the device that prepareDevice started creating, until
// the first window takes it.
var gpuPending chan gpuResult

type gpuResult struct {
	dev *vulkan.Device
	err error
}

// gpuReported is set once the reason a window is drawn on the CPU was printed.
var gpuReported bool

// cpuOnly reports whether JAMB_RENDERER=cpu asks for the CPU renderer.
func cpuOnly() bool { return os.Getenv("JAMB_RENDERER") == "cpu" }

// prepareDevice starts creating the device on another goroutine: loading the
// Vulkan driver takes long enough to overlap with creating the first window.
func osPrepareDevice() {
	if cpuOnly() {
		return
	}
	gpuPending = make(chan gpuResult, 1)
	go func() {
		dev, err := vulkan.NewDevice()
		gpuPending <- gpuResult{dev, err}
	}()
}

// dmabufWindow is a window that can show frames from dma-bufs.
type dmabufWindow interface {
	DmabufSurface() dmabuf.Surface
}

// osNewRenderer returns a Vulkan renderer for the window, or the CPU renderer
// when Vulkan, the GPU or the compositor cannot draw it. The reason is printed
// once.
func osNewRenderer(w platform.Window, width, height int) (render.Renderer, error) {
	if cpuOnly() {
		return newSoftRenderer(w)
	}
	r, err := newGPURenderer(w)
	if err == nil {
		return r, nil
	}
	if !gpuReported {
		gpuReported = true
		fmt.Fprintf(os.Stderr, "jamb: drawing on the CPU: %v\n", err)
	}
	return newSoftRenderer(w)
}

func newGPURenderer(w platform.Window) (render.Renderer, error) {
	dw, ok := w.(dmabufWindow)
	if !ok {
		return nil, fmt.Errorf("the window %T cannot show GPU buffers", w)
	}
	s := dw.DmabufSurface()
	if s == nil {
		return nil, fmt.Errorf("the compositor does not import ARGB8888 dma-bufs with modifiers (zwp_linux_dmabuf_v1 version 3)")
	}
	if gpu == nil {
		var res gpuResult
		if gpuPending != nil {
			res = <-gpuPending
			gpuPending = nil
		} else {
			res.dev, res.err = vulkan.NewDevice()
		}
		if res.err != nil {
			return nil, res.err
		}
		gpu = res.dev
		// The device was chosen before the compositor named its own. On a
		// machine with several GPUs, draw on that one when it qualifies, so
		// frames are not copied between GPUs. When it does not, the renderer
		// shares its frames with that GPU through system memory, or the
		// window is drawn on the CPU (see vulkan.Renderer.setup).
		if main, ok := s.MainDevice(); ok && !gpu.OnDevice(main) {
			gpu.Prefer(main)
			if err := gpu.Recover(); err != nil {
				gpu.Close()
				gpu = nil
				return nil, err
			}
		}
	}
	return gpu.NewRenderer(s)
}

// releaseDevice releases the shared device and its objects, as a loss leaves
// them; see simulateDeviceLoss.
func osReleaseDevice() {
	if gpu != nil {
		gpu.ReleaseDevice()
	}
}

// rebuildDevice recreates the shared device after a loss. Each window then
// recreates its images; see rebuildGPU.
func osRebuildDevice() error {
	if gpu == nil {
		return nil
	}
	return gpu.Recover()
}

// gpuAdapter names the GPU the device runs on, for the recovery message.
func osGpuAdapter() string {
	if gpu == nil {
		return "CPU"
	}
	return gpu.Adapter()
}
