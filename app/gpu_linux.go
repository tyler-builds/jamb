//go:build linux && (amd64 || arm64)

package app

import (
	"fmt"
	"os"
	"time"

	"github.com/tyler-builds/jamb/internal/dmabuf"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/vulkan"
	"github.com/tyler-builds/jamb/internal/scene"
)

// gpu is the Vulkan device that every window renders with, or nil when there
// is none: windows are then drawn on the CPU. The first window creates it.
var gpu *vulkan.Device

// gpuPrepared is closed when the device that prepareDevice started creating
// is ready, in gpuPrepResult, and nil once the first window has taken it.
var (
	gpuPrepared   chan struct{}
	gpuPrepResult gpuResult
)

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
	ready := make(chan struct{})
	gpuPrepared = ready
	go func() {
		gpuPrepResult.dev, gpuPrepResult.err = vulkan.NewDevice()
		close(ready)
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
	// Creating the device takes a GPU driver a long time, 100 ms or more. Until
	// it is ready, the window is drawn on the CPU, so that it appears at once.
	if ready := gpuPrepared; gpu == nil && ready != nil && !closed(ready) {
		if soft, err := newSoftRenderer(w); err == nil {
			go func() {
				<-ready
				w.Wake() // the next frame moves the window to the GPU
			}()
			return &bootRenderer{Renderer: soft, w: w, ready: ready}, nil
		}
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
		if gpuPrepared != nil {
			<-gpuPrepared
			res = gpuPrepResult
			gpuPrepared, gpuPrepResult = nil, gpuResult{}
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

func closed(c chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

// bootRenderer draws a window on the CPU while the GPU device is still being
// created, and moves it to the GPU at the first frame after the device is
// ready. A window that cannot be drawn on the GPU stays on the CPU.
type bootRenderer struct {
	render.Renderer
	w     platform.Window
	ready chan struct{} // closed when the device is ready; nil once moved
}

// BeginFrame implements render.Renderer.
func (b *bootRenderer) BeginFrame(width, height int, clear scene.Color) error {
	if b.ready != nil && closed(b.ready) {
		b.ready = nil
		r, err := newGPURenderer(b.w)
		if err == nil {
			b.Renderer.Close()
			b.Renderer = r
			application.dev.tracef("window moved to the GPU %v after process start", time.Since(processStart))
		} else if !gpuReported {
			gpuReported = true
			fmt.Fprintf(os.Stderr, "jamb: drawing on the CPU: %v\n", err)
		}
	}
	return b.Renderer.BeginFrame(width, height, clear)
}

// Describe names the renderer in use, for JAMB_TRACE.
func (b *bootRenderer) Describe() string {
	if b.ready != nil {
		return "CPU until the GPU is ready"
	}
	if d, ok := b.Renderer.(interface{ Describe() string }); ok {
		return d.Describe()
	}
	return "CPU"
}

// Recreate rebuilds the GPU renderer's objects after a device loss.
func (b *bootRenderer) Recreate() error {
	if rb, ok := b.Renderer.(interface{ Recreate() error }); ok {
		return rb.Recreate()
	}
	return nil
}

// LastPresent reports the last frame's present times, for JAMB_FRAMESTATS.
func (b *bootRenderer) LastPresent() (start, end int64) {
	if p, ok := b.Renderer.(interface{ LastPresent() (start, end int64) }); ok {
		return p.LastPresent()
	}
	now := clockNow()
	return now, now
}
