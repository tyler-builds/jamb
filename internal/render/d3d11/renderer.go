//go:build windows

// Package d3d11 is the Direct3D 11 backend for render.Renderer.
//
// It is pure Go: Direct3D is reached through COM vtables with syscall, and
// HLSL is compiled at startup with the system's d3dcompiler_47.dll.
package d3d11

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/win32"
)

var (
	modD3D11 = syscall.NewLazyDLL("d3d11.dll")

	procD3D11CreateDeviceAndSwapChain = modD3D11.NewProc("D3D11CreateDeviceAndSwapChain")
	procD3D11CreateDevice             = modD3D11.NewProc("D3D11CreateDevice")
)

// Per-instance vertex data; layouts must match the HLSL input structs. CShape
// and CRadii are the innermost rounded clip of the level (see clipState).
type quadInstance struct {
	Rect, Clip, Color, Border, Radii, Params, Color2, Grad, Sides, CShape, CRadii [4]float32
}

type maskInstance struct {
	Rect, UV, Color, Clip, CShape, CRadii [4]float32
}

type imageInstance struct {
	Rect, UV, Clip, Radii, Params, CShape, CRadii [4]float32
}

type pipelineKind uint8

const (
	pipeQuad pipelineKind = iota
	pipeMask
	pipeImage
	pipeSubpixel // ClearType glyphs: mask instances, dual-source blending
	pipeCount
)

type pipeline struct {
	vs, ps, layout *com
	stride         uint32
}

type command struct {
	pipe         pipelineKind
	start, count uint32
	tex          *scene.Texture
}

type gpuTexture struct {
	tex, srv      *com
	width, height int
	version       uint64
}

type instanceBuffer struct {
	buf      *com
	capacity int // in bytes
}

// Device is the Direct3D 11 device and the GPU state every window shares:
// shaders, render states, instance buffers and textures. Create one per
// process with NewDevice; all windows render from the UI thread, so the
// device's immediate context is used by one window at a time.
type Device struct {
	device  *com
	ctx     *com
	factory *com // IDXGIFactory2 of the adapter, creates the window swap chains
	warp    bool // the device runs on the WARP software rasterizer, not the hardware
	cbuf    *com
	blend   *com
	// blendDual blends subpixel glyphs with dual-source blending; nil when
	// the device rejected it, in which case they are drawn as grayscale.
	blendDual *com
	raster    *com
	samplers  [2]*com
	pipes     [pipeCount]pipeline
	vbufs     [pipeCount]instanceBuffer
	// dualSource is set when subpixel glyphs can use dual-source blending.
	dualSource bool
	// textures are uploaded once per device, so a glyph atlas or image is
	// shared by every window that draws it. After Recover the map is empty, and
	// each texture is uploaded again from its scene.Texture.Pix. That copy lives
	// as long as the texture, so recovery repeats no decode or rasterization. It
	// costs no memory beyond what the texture already holds: w*h*4 bytes for an
	// image, and each glyph atlas at its current size (up to 16 MB grayscale and
	// 64 MB ClearType).
	textures map[uint64]*gpuTexture
}

// clipState is one level of the clip stack. rect is the intersection of every
// clip so far. shape and radii are the innermost rounded clip (zero when there
// is none). Its corners are applied to its own rectangle, not to the
// intersection, so an outer clip that cuts it does not round the cut. Only the
// innermost rounded clip keeps its corners; outer ones act as rectangles.
type clipState struct {
	rect  scene.Rect
	shape scene.Rect
	radii scene.Corners
}

// Renderer implements render.Renderer with Direct3D 11 for one window: a swap
// chain and its render target. The device it draws with is shared.
type Renderer struct {
	*Device
	hwnd uintptr
	swap *com
	rtv  *com
	flip bool

	width, height int
	clear         scene.Color
	clips         []clipState

	quads    []quadInstance
	masks    []maskInstance
	subs     []maskInstance // subpixel glyphs
	images   []imageInstance
	text     scene.TextParams // of the subpixel atlas seen this frame
	commands []command

	capture func(*image.RGBA, error)
	noVSync bool
	swap1   bool // created via CreateSwapChainForHwnd (unscaled flip model)
	// waitable is the swap chain's frame latency waitable object, 0 when the
	// chain has none. It is signaled when the chain can take another frame.
	waitable    uintptr
	swapFlags   uint32
	resized     bool // the swap chain changed size this frame
	presentWait time.Duration
	// occluded is set when the last Present found the window covered by another
	// window, so its frame was not shown. occlusionFactory and occlusionCookie
	// record the registration that wakes the window when it is visible again.
	occluded         bool
	occlusionFactory *com
	occlusionCookie  uint32
}

// Occluded reports whether the last frame was not shown because another window
// covers this one. Such a window should stop asking for frames until it is
// woken: DXGI posts win32.WM_OCCLUSION to it when it is visible again.
func (r *Renderer) Occluded() bool { return r.occluded }

// Describe summarizes the swap chain configuration, for diagnostics.
func (r *Renderer) Describe() string {
	switch {
	case r.swap1:
		return "Direct3D 11, flip model, unscaled"
	case r.flip:
		return "Direct3D 11, flip model, stretch-scaled (fallback)"
	}
	return "Direct3D 11, blit model (fallback)"
}

// LastPresentWait reports how long the last Present call blocked.
func (r *Renderer) LastPresentWait() time.Duration { return r.presentWait }

// SetVSync enables (default) or disables waiting for vertical blank in
// Present. Disabling it is only useful for measuring raw throughput.
func (r *Renderer) SetVSync(on bool) { r.noVSync = !on }

var _ render.Renderer = (*Renderer)(nil)

// errDeviceReleased is returned for a window created or recreated on a device
// that was released and not recovered yet.
var errDeviceReleased = errors.New("d3d11: the GPU device was released and has not been recovered")

// NewDevice creates the Direct3D 11 device and the pipelines and states every
// window shares. Call it once; NewRenderer adds a window to it.
func NewDevice() (*Device, error) {
	d := &Device{textures: map[uint64]*gpuTexture{}}
	if err := d.create(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// create creates the Direct3D device and every object it owns: pipelines,
// blend, raster and sampler states, and the constant buffer.
func (d *Device) create() error {
	if err := d.createDevice(); err != nil {
		return err
	}
	if err := d.createPipelines(); err != nil {
		return err
	}
	return d.createStates()
}

// Recover recreates the device and every object it owns after a loss (or after
// ReleaseDevice). The shaders come from the bytecode cache. Textures are not
// recreated here: each is uploaded again from its CPU pixels the next time a
// frame draws it, so glyph atlases and images come back without being
// rasterized or decoded again. Each window then calls Renderer.Recreate. Recover
// may fail while the GPU is still unavailable; it can be called again.
func (d *Device) Recover() error {
	d.release()
	return d.create()
}

// ReleaseDevice releases the device and every object it owns, as a removal
// leaves them, without waiting for one. It is the simulation hook for the
// devicelost script step (see app/dev.go): nothing may draw until Recover runs.
// The windows' swap chains are released by Renderer.Recreate.
func (d *Device) ReleaseDevice() { d.release() }

// NewRenderer creates a swap chain of width×height physical pixels for the
// window hwnd, on this device.
func (d *Device) NewRenderer(hwnd uintptr, width, height int) (*Renderer, error) {
	if d.device == nil {
		return nil, errDeviceReleased
	}
	r := &Renderer{Device: d, hwnd: hwnd, width: width, height: height}
	if err := r.createSwapChain(); err != nil {
		return nil, err
	}
	if err := r.createTarget(); err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

func (d *Device) createDevice() error {
	if err := procD3D11CreateDevice.Find(); err != nil {
		return fmt.Errorf("d3d11: d3d11.dll is not available: %v", err)
	}
	flags := uintptr(d3dCreateBGRASupport)
	if os.Getenv("JAMB_D3D_DEBUG") == "1" {
		flags |= d3dCreateDebug
	}
	levels := [3]uint32{0xb000, 0xa100, 0xa000} // 11_0, 10_1, 10_0

	// Hardware first; the WARP software rasterizer keeps the app starting on
	// odd hardware or in VMs.
	var lastErr error
	for _, driver := range []uintptr{d3dDriverTypeHardware, d3dDriverTypeWARP} {
		if err := d.createOn(driver, flags, levels[:]); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("%w; no usable Direct3D 11 device (tried hardware and WARP)", lastErr)
}

// createOn creates the device on one driver and keeps the DXGI factory of its
// adapter, which creates the swap chains.
func (d *Device) createOn(driver, flags uintptr, levels []uint32) error {
	var device, ctx *com
	var level uint32
	hr, _, _ := syscall.SyscallN(procD3D11CreateDevice.Addr(),
		0, driver, 0, flags,
		uintptr(unsafe.Pointer(&levels[0])), uintptr(len(levels)), d3dSDKVersion,
		uintptr(unsafe.Pointer(&device)), uintptr(unsafe.Pointer(&level)), uintptr(unsafe.Pointer(&ctx)))
	if err := check("D3D11CreateDevice", hr); err != nil {
		return err
	}
	fail := func(err error) error {
		ctx.Release()
		device.Release()
		return err
	}
	// device -> IDXGIDevice -> adapter -> IDXGIFactory2
	var dxgiDev, adapter, factory *com
	hr, _, _ = syscall.SyscallN(device.fn(0), device.this(), uintptr(unsafe.Pointer(&iidIDXGIDevice)), uintptr(unsafe.Pointer(&dxgiDev)))
	if err := check("QueryInterface(IDXGIDevice)", hr); err != nil {
		return fail(err)
	}
	defer dxgiDev.Release()
	hr, _, _ = syscall.SyscallN(dxgiDev.fn(7), dxgiDev.this(), uintptr(unsafe.Pointer(&adapter))) // GetAdapter
	if err := check("IDXGIDevice::GetAdapter", hr); err != nil {
		return fail(err)
	}
	defer adapter.Release()
	hr, _, _ = syscall.SyscallN(adapter.fn(6), adapter.this(), uintptr(unsafe.Pointer(&iidIDXGIFactory2)), uintptr(unsafe.Pointer(&factory))) // GetParent
	if err := check("IDXGIAdapter::GetParent(IDXGIFactory2)", hr); err != nil {
		return fail(err)
	}
	d.device, d.ctx, d.factory = device, ctx, factory
	d.warp = driver == d3dDriverTypeWARP
	return nil
}

// Adapter names the adapter the device runs on, for diagnostics: "hardware",
// or "WARP software rasterizer" when Direct3D fell back to WARP.
func (d *Device) Adapter() string {
	if d.warp {
		return "WARP software rasterizer"
	}
	return "hardware"
}

// createSwapChain creates the window's swap chain. The preferred one is a
// flip-model swap chain with DXGI_SCALING_NONE: during a live resize Windows
// shows the last frame until the next one arrives; with the default (stretch)
// scaling that frame is visibly squashed and stretched, which reads as content
// jumping around. Unscaled, it stays pinned 1:1 to the top-left. This needs
// IDXGIFactory2 (Windows 8+). The fallbacks are the classic stretch-scaled
// flip model and then the blit model.
func (r *Renderer) createSwapChain() error {
	desc1 := dxgiSwapChainDesc1{
		Format:      dxgiFormatBGRA8Unorm,
		SampleDesc:  dxgiSampleDesc{Count: 1},
		BufferUsage: dxgiUsageRenderTarget,
		BufferCount: 2,
		Scaling:     dxgiScalingNone,
		SwapEffect:  dxgiSwapEffectFlipDisc,
		Flags:       dxgiSwapChainFlagWaitable,
	}
	var swap *com
	hr, _, _ := syscall.SyscallN(r.factory.fn(15), r.factory.this(), r.device.this(), r.hwnd,
		uintptr(unsafe.Pointer(&desc1)), 0, 0, uintptr(unsafe.Pointer(&swap))) // CreateSwapChainForHwnd
	err := check("IDXGIFactory2::CreateSwapChainForHwnd", hr)
	if err == nil {
		// Sizing is ours; keep DXGI from taking over Alt+Enter.
		syscall.SyscallN(r.factory.fn(8), r.factory.this(), r.hwnd, dxgiMWANoAltEnter) // MakeWindowAssociation
		r.flip, r.swap1 = true, true
		r.finishSwapChain(swap, desc1.Flags)
		return nil
	}
	lastErr := err
	attempts := []struct {
		effect uint32
		count  uint32
	}{{dxgiSwapEffectFlipDisc, 2}, {dxgiSwapEffectDiscard, 1}}
	for _, a := range attempts {
		desc := dxgiSwapChainDesc{
			BufferDesc:   dxgiModeDesc{Format: dxgiFormatBGRA8Unorm},
			SampleDesc:   dxgiSampleDesc{Count: 1},
			BufferUsage:  dxgiUsageRenderTarget,
			BufferCount:  a.count,
			OutputWindow: r.hwnd,
			Windowed:     1,
			SwapEffect:   a.effect,
			Flags:        flipWaitable(a.effect),
		}
		var swap *com
		hr, _, _ := syscall.SyscallN(r.factory.fn(10), r.factory.this(), r.device.this(),
			uintptr(unsafe.Pointer(&desc)), uintptr(unsafe.Pointer(&swap))) // IDXGIFactory::CreateSwapChain
		if err := check("IDXGIFactory::CreateSwapChain", hr); err != nil {
			lastErr = err
			continue
		}
		syscall.SyscallN(r.factory.fn(8), r.factory.this(), r.hwnd, dxgiMWANoAltEnter) // MakeWindowAssociation
		r.flip = a.effect == dxgiSwapEffectFlipDisc
		r.finishSwapChain(swap, desc.Flags)
		return nil
	}
	return lastErr
}

// limitLatency keeps at most one frame queued so input feels immediate.
func (d *Device) limitLatency() {
	var dxgiDev *com
	hr, _, _ := syscall.SyscallN(d.device.fn(0), d.device.this(), uintptr(unsafe.Pointer(&iidIDXGIDevice1)), uintptr(unsafe.Pointer(&dxgiDev)))
	if hresult(hr).failed() || dxgiDev == nil {
		return
	}
	syscall.SyscallN(dxgiDev.fn(12), dxgiDev.this(), 1) // IDXGIDevice1::SetMaximumFrameLatency
	dxgiDev.Release()
}

func (r *Renderer) createTarget() error {
	var back *com
	hr, _, _ := syscall.SyscallN(r.swap.fn(9), r.swap.this(), 0, uintptr(unsafe.Pointer(&iidID3D11Texture2D)), uintptr(unsafe.Pointer(&back))) // GetBuffer
	if err := r.checkDevice("IDXGISwapChain::GetBuffer", hr); err != nil {
		return err
	}
	defer back.Release()
	var rtv *com
	hr, _, _ = syscall.SyscallN(r.device.fn(9), r.device.this(), back.this(), 0, uintptr(unsafe.Pointer(&rtv))) // CreateRenderTargetView
	if err := r.checkDevice("ID3D11Device::CreateRenderTargetView", hr); err != nil {
		return err
	}
	r.rtv = rtv
	return nil
}

func (r *Renderer) resize(w, h int) error {
	syscall.SyscallN(r.ctx.fn(33), r.ctx.this(), 0, 0, 0) // OMSetRenderTargets(0, nil, nil)
	r.rtv.Release()
	r.rtv = nil
	hr, _, _ := syscall.SyscallN(r.swap.fn(13), r.swap.this(), 0, uintptr(w), uintptr(h), dxgiFormatUnknown, uintptr(r.swapFlags)) // ResizeBuffers
	if err := r.checkDevice("IDXGISwapChain::ResizeBuffers", hr); err != nil {
		return err
	}
	r.width, r.height = w, h
	return r.createTarget()
}

type inputElem struct {
	name  string
	index uint32
}

func (r *Device) createPipelines() error {
	// Dual-source blending is required of feature level 10_0 and up, so this
	// only fails on a device that does not follow that.
	r.dualSource = r.createDualBlend() == nil

	type spec struct {
		kind   pipelineKind
		vs, ps string
		elems  []inputElem
		stride uintptr
	}
	specs := []spec{
		{pipeQuad, "quad_vs", "quad_ps", []inputElem{{"RECT", 0}, {"CLIP", 0}, {"COLOR", 0}, {"COLOR", 1}, {"RADII", 0}, {"PARAMS", 0}, {"COLOR", 2}, {"GRAD", 0}, {"SIDES", 0}, {"CSHAPE", 0}, {"CRADII", 0}}, unsafe.Sizeof(quadInstance{})},
		{pipeMask, "mask_vs", "mask_ps", []inputElem{{"RECT", 0}, {"UV", 0}, {"COLOR", 0}, {"CLIP", 0}, {"CSHAPE", 0}, {"CRADII", 0}}, unsafe.Sizeof(maskInstance{})},
		{pipeImage, "image_vs", "image_ps", []inputElem{{"RECT", 0}, {"UV", 0}, {"CLIP", 0}, {"RADII", 0}, {"PARAMS", 0}, {"CSHAPE", 0}, {"CRADII", 0}}, unsafe.Sizeof(imageInstance{})},
		{pipeSubpixel, "mask_vs", "subpixel_ps", []inputElem{{"RECT", 0}, {"UV", 0}, {"COLOR", 0}, {"CLIP", 0}, {"CSHAPE", 0}, {"CRADII", 0}}, unsafe.Sizeof(maskInstance{})},
	}
	for _, s := range specs {
		if s.kind == pipeSubpixel && !r.dualSource {
			s.ps = "subpixel_gray_ps"
		}
		vsCode, err := compileShader(s.vs, "vs_4_0")
		if err != nil {
			return err
		}
		psCode, err := compileShader(s.ps, "ps_4_0")
		if err != nil {
			return err
		}
		p := pipeline{stride: uint32(s.stride)}
		hr, _, _ := syscall.SyscallN(r.device.fn(12), r.device.this(), uintptr(unsafe.Pointer(&vsCode[0])), uintptr(len(vsCode)), 0, uintptr(unsafe.Pointer(&p.vs))) // CreateVertexShader
		if err := check("CreateVertexShader("+s.vs+")", hr); err != nil {
			return err
		}
		hr, _, _ = syscall.SyscallN(r.device.fn(15), r.device.this(), uintptr(unsafe.Pointer(&psCode[0])), uintptr(len(psCode)), 0, uintptr(unsafe.Pointer(&p.ps))) // CreatePixelShader
		if err := check("CreatePixelShader("+s.ps+")", hr); err != nil {
			return err
		}
		descs := make([]inputElementDesc, len(s.elems))
		names := make([][]byte, len(s.elems))
		for i, e := range s.elems {
			names[i] = append([]byte(e.name), 0)
			descs[i] = inputElementDesc{
				SemanticName:         &names[i][0],
				SemanticIndex:        e.index,
				Format:               dxgiFormatRGBA32Float,
				AlignedByteOffset:    uint32(i * 16),
				InputSlotClass:       d3dInputPerInstance,
				InstanceDataStepRate: 1,
			}
		}
		hr, _, _ = syscall.SyscallN(r.device.fn(11), r.device.this(), uintptr(unsafe.Pointer(&descs[0])), uintptr(len(descs)),
			uintptr(unsafe.Pointer(&vsCode[0])), uintptr(len(vsCode)), uintptr(unsafe.Pointer(&p.layout))) // CreateInputLayout
		if err := check("CreateInputLayout("+s.vs+")", hr); err != nil {
			return err
		}
		r.pipes[s.kind] = p
	}
	return nil
}

// createDualBlend creates the blend state for ClearType text. The pixel
// shader's second output is the per-channel coverage, so
// dst = src0 + dst * (1 - src1): each color channel is attenuated by its own
// subpixel coverage. src0 is already premultiplied by that coverage.
func (r *Device) createDualBlend() error {
	bd := blendDesc{}
	bd.RenderTarget[0] = renderTargetBlendDesc{
		BlendEnable: 1,
		SrcBlend:    d3dBlendOne, DestBlend: d3dBlendInvSrc1Color, BlendOp: d3dBlendOpAdd,
		SrcBlendAlpha: d3dBlendOne, DestBlendAlpha: d3dBlendInvSrc1Alpha, BlendOpAlpha: d3dBlendOpAdd,
		RenderTargetWriteMask: 0xF,
	}
	hr, _, _ := syscall.SyscallN(r.device.fn(20), r.device.this(), uintptr(unsafe.Pointer(&bd)), uintptr(unsafe.Pointer(&r.blendDual))) // CreateBlendState
	return check("CreateBlendState(dual-source)", hr)
}

func (r *Device) createStates() error {
	cb := bufferDesc{ByteWidth: 32, Usage: d3dUsageDynamic, BindFlags: d3dBindConstantBuffer, CPUAccessFlags: d3dCPUAccessWrite}
	hr, _, _ := syscall.SyscallN(r.device.fn(3), r.device.this(), uintptr(unsafe.Pointer(&cb)), 0, uintptr(unsafe.Pointer(&r.cbuf))) // CreateBuffer
	if err := check("CreateBuffer(constants)", hr); err != nil {
		return err
	}

	bd := blendDesc{}
	bd.RenderTarget[0] = renderTargetBlendDesc{
		BlendEnable: 1,
		SrcBlend:    d3dBlendOne, DestBlend: d3dBlendInvSrcAlpha, BlendOp: d3dBlendOpAdd,
		SrcBlendAlpha: d3dBlendOne, DestBlendAlpha: d3dBlendInvSrcAlpha, BlendOpAlpha: d3dBlendOpAdd,
		RenderTargetWriteMask: 0xF,
	}
	hr, _, _ = syscall.SyscallN(r.device.fn(20), r.device.this(), uintptr(unsafe.Pointer(&bd)), uintptr(unsafe.Pointer(&r.blend))) // CreateBlendState
	if err := check("CreateBlendState", hr); err != nil {
		return err
	}

	rd := rasterizerDesc{FillMode: d3dFillSolid, CullMode: d3dCullNone, DepthClipEnable: 1}
	hr, _, _ = syscall.SyscallN(r.device.fn(22), r.device.this(), uintptr(unsafe.Pointer(&rd)), uintptr(unsafe.Pointer(&r.raster))) // CreateRasterizerState
	if err := check("CreateRasterizerState", hr); err != nil {
		return err
	}

	for i, filter := range []uint32{d3dFilterPoint, d3dFilterLinear} {
		sd := samplerDesc{Filter: filter, AddressU: d3dAddressClamp, AddressV: d3dAddressClamp, AddressW: d3dAddressClamp,
			ComparisonFunc: d3dComparisonNever, MaxLOD: 3.402823466e+38}
		hr, _, _ = syscall.SyscallN(r.device.fn(23), r.device.this(), uintptr(unsafe.Pointer(&sd)), uintptr(unsafe.Pointer(&r.samplers[i]))) // CreateSamplerState
		if err := check("CreateSamplerState", hr); err != nil {
			return err
		}
	}
	return nil
}

// BeginFrame implements render.Renderer.
func (r *Renderer) BeginFrame(width, height int, clear scene.Color) error {
	if width <= 0 || height <= 0 {
		return errors.New("d3d11: BeginFrame called with an empty size; skip rendering while minimized")
	}
	if width != r.width || height != r.height {
		if err := r.resize(width, height); err != nil {
			return err
		}
		r.resized = true
	}
	if r.swap1 && clear != r.clear {
		// Color DWM shows where the window is larger than the last frame
		// (briefly, while an edge is being dragged).
		bg := [4]float32{clear.R, clear.G, clear.B, 1}
		syscall.SyscallN(r.swap.fn(25), r.swap.this(), uintptr(unsafe.Pointer(&bg[0]))) // IDXGISwapChain1::SetBackgroundColor
	}
	r.clear = clear
	r.clips = append(r.clips[:0], clipState{rect: scene.Rect{X: 0, Y: 0, W: float32(width), H: float32(height)}})
	r.quads = r.quads[:0]
	r.masks = r.masks[:0]
	r.subs = r.subs[:0]
	r.images = r.images[:0]
	r.commands = r.commands[:0]
	return nil
}

// clipInstance returns the clip of the current level for the instances being
// added: the rectangle (x0, y0, x1, y1), and the rounded shape and its radii.
func (r *Renderer) clipInstance() (clip, shape, radii [4]float32) {
	c := r.clips[len(r.clips)-1]
	return [4]float32{c.rect.X, c.rect.Y, c.rect.Right(), c.rect.Bottom()}, rect4(c.shape), corners4(c.radii)
}

// PushClip implements render.Renderer.
func (r *Renderer) PushClip(c scene.Rect) {
	top := r.clips[len(r.clips)-1]
	r.clips = append(r.clips, clipState{rect: top.rect.Intersect(c), shape: top.shape, radii: top.radii})
}

// PushRoundedClip implements render.Renderer.
func (r *Renderer) PushRoundedClip(c scene.Rect, radii scene.Corners) {
	top := r.clips[len(r.clips)-1]
	r.clips = append(r.clips, clipState{rect: top.rect.Intersect(c), shape: c, radii: radii})
}

// PopClip implements render.Renderer.
func (r *Renderer) PopClip() {
	if len(r.clips) > 1 {
		r.clips = r.clips[:len(r.clips)-1]
	}
}

func (r *Renderer) addCommand(p pipelineKind, start, count int, tex *scene.Texture) {
	if n := len(r.commands); n > 0 {
		last := &r.commands[n-1]
		if last.pipe == p && last.tex == tex && int(last.start+last.count) == start {
			last.count += uint32(count)
			return
		}
	}
	r.commands = append(r.commands, command{pipe: p, start: uint32(start), count: uint32(count), tex: tex})
}

func rect4(b scene.Rect) [4]float32       { return [4]float32{b.X, b.Y, b.W, b.H} }
func color4(c scene.Color) [4]float32     { return [4]float32{c.R, c.G, c.B, c.A} }
func corners4(c scene.Corners) [4]float32 { return [4]float32{c.TL, c.TR, c.BR, c.BL} }

// DrawQuads implements render.Renderer.
func (r *Renderer) DrawQuads(qs []scene.Quad) {
	if len(qs) == 0 {
		return
	}
	clip, cshape, cradii := r.clipInstance()
	start := len(r.quads)
	for _, q := range qs {
		r.quads = append(r.quads, quadInstance{
			Rect: rect4(q.Bounds), Clip: clip, Color: color4(q.Background), Border: color4(q.BorderColor),
			Radii: corners4(q.Radii), Params: quadParams(q), Color2: color4(q.Color2), Grad: gradient4(q),
			Sides: quadSides(q), CShape: cshape, CRadii: cradii,
		})
	}
	r.addCommand(pipeQuad, start, len(qs), nil)
}

// quadParams packs the shadow sigma and the gradient flags.
func quadParams(q scene.Quad) [4]float32 {
	p := [4]float32{q.Blur, 0, 0, 0}
	if q.Gradient {
		p[1] = 1
		if q.Oklab {
			p[2] = 1
		}
	}
	return p
}

// quadSides returns the border width of each side (top, right, bottom, left):
// BorderSides when set, else BorderWidth on every side.
func quadSides(q scene.Quad) [4]float32 {
	if s := q.BorderSides; s != (scene.Sides{}) {
		return [4]float32{s.Top, s.Right, s.Bottom, s.Left}
	}
	w := q.BorderWidth
	return [4]float32{w, w, w, w}
}

// gradient4 returns the gradient line's unit direction and stop positions. The
// angle is CSS's: 0 points up, 90 right, 180 down.
func gradient4(q scene.Quad) [4]float32 {
	if !q.Gradient {
		return [4]float32{}
	}
	rad := float64(q.Angle) * math.Pi / 180
	return [4]float32{float32(math.Sin(rad)), float32(-math.Cos(rad)), q.Stop0, q.Stop1}
}

// DrawGlyphs implements render.Renderer.
func (r *Renderer) DrawGlyphs(run scene.GlyphRun) {
	if len(run.Glyphs) == 0 || run.Texture == nil {
		return
	}
	// Subpixel (ClearType) masks have their own pipeline and instance buffer.
	pipe, list := pipeMask, &r.masks
	if run.Texture.Format == scene.FormatSubpixel8 {
		pipe, list = pipeSubpixel, &r.subs
		r.text = run.Texture.Text
	}
	clip, cshape, cradii := r.clipInstance()
	tw, th := float32(run.Texture.Width), float32(run.Texture.Height)
	start := len(*list)
	for _, g := range run.Glyphs {
		*list = append(*list, maskInstance{
			Rect:  rect4(g.Bounds),
			UV:    [4]float32{g.Src.X / tw, g.Src.Y / th, (g.Src.X + g.Src.W) / tw, (g.Src.Y + g.Src.H) / th},
			Color: color4(g.Color), Clip: clip, CShape: cshape, CRadii: cradii,
		})
	}
	r.addCommand(pipe, start, len(run.Glyphs), run.Texture)
}

// DrawImages implements render.Renderer.
func (r *Renderer) DrawImages(ims []scene.ImageDraw) {
	clip, cshape, cradii := r.clipInstance()
	for _, im := range ims {
		if im.Texture == nil {
			continue
		}
		tw, th := float32(im.Texture.Width), float32(im.Texture.Height)
		src := im.Src
		if src.Empty() {
			src = scene.Rect{W: tw, H: th}
		}
		r.images = append(r.images, imageInstance{
			Rect:   rect4(im.Bounds),
			UV:     [4]float32{src.X / tw, src.Y / th, (src.X + src.W) / tw, (src.Y + src.H) / th},
			Clip:   clip,
			Radii:  corners4(im.Radii),
			Params: [4]float32{im.Opacity},
			CShape: cshape,
			CRadii: cradii,
		})
		r.addCommand(pipeImage, len(r.images)-1, 1, im.Texture)
	}
}

// CaptureNextFrame implements render.Renderer.
func (r *Renderer) CaptureNextFrame(fn func(*image.RGBA, error)) { r.capture = fn }

// EndFrame implements render.Renderer.
func (r *Renderer) EndFrame() error {
	for _, c := range r.commands {
		if c.tex != nil {
			if err := r.syncTexture(c.tex); err != nil {
				return err
			}
		}
	}
	if err := r.upload(pipeQuad, unsafe.Pointer(unsafe.SliceData(r.quads)), len(r.quads)); err != nil {
		return err
	}
	if err := r.upload(pipeMask, unsafe.Pointer(unsafe.SliceData(r.masks)), len(r.masks)); err != nil {
		return err
	}
	if err := r.upload(pipeSubpixel, unsafe.Pointer(unsafe.SliceData(r.subs)), len(r.subs)); err != nil {
		return err
	}
	if err := r.upload(pipeImage, unsafe.Pointer(unsafe.SliceData(r.images)), len(r.images)); err != nil {
		return err
	}

	// Constants.
	var m mappedSubresource
	hr, _, _ := syscall.SyscallN(r.ctx.fn(14), r.ctx.this(), r.cbuf.this(), 0, d3dMapWriteDiscard, 0, uintptr(unsafe.Pointer(&m))) // Map
	if err := r.checkDevice("Map(constants)", hr); err != nil {
		return err
	}
	consts := [8]float32{float32(r.width), float32(r.height), 0, 0, r.text.Gamma, r.text.Contrast, r.text.Level}
	copy(unsafe.Slice((*byte)(ptrFrom(m.PData)), 32), unsafe.Slice((*byte)(unsafe.Pointer(&consts[0])), 32))
	syscall.SyscallN(r.ctx.fn(15), r.ctx.this(), r.cbuf.this(), 0) // Unmap

	ctx := r.ctx
	rtv := r.rtv
	syscall.SyscallN(ctx.fn(33), ctx.this(), 1, uintptr(unsafe.Pointer(&rtv)), 0) // OMSetRenderTargets
	vp := viewport{Width: float32(r.width), Height: float32(r.height), MaxDepth: 1}
	syscall.SyscallN(ctx.fn(44), ctx.this(), 1, uintptr(unsafe.Pointer(&vp))) // RSSetViewports
	clear := [4]float32{r.clear.R * r.clear.A, r.clear.G * r.clear.A, r.clear.B * r.clear.A, r.clear.A}
	syscall.SyscallN(ctx.fn(50), ctx.this(), rtv.this(), uintptr(unsafe.Pointer(&clear[0]))) // ClearRenderTargetView
	blendFactor := [4]float32{}
	syscall.SyscallN(ctx.fn(35), ctx.this(), r.blend.this(), uintptr(unsafe.Pointer(&blendFactor[0])), 0xffffffff) // OMSetBlendState
	syscall.SyscallN(ctx.fn(43), ctx.this(), r.raster.this())                                                      // RSSetState
	syscall.SyscallN(ctx.fn(24), ctx.this(), d3dTopologyTriangleStrip)                                             // IASetPrimitiveTopology
	cbuf := r.cbuf
	syscall.SyscallN(ctx.fn(7), ctx.this(), 0, 1, uintptr(unsafe.Pointer(&cbuf)))           // VSSetConstantBuffers
	syscall.SyscallN(ctx.fn(16), ctx.this(), 0, 1, uintptr(unsafe.Pointer(&cbuf)))          // PSSetConstantBuffers
	syscall.SyscallN(ctx.fn(10), ctx.this(), 0, 2, uintptr(unsafe.Pointer(&r.samplers[0]))) // PSSetSamplers

	current := pipelineKind(255)
	curBlend := r.blend
	var boundTex *scene.Texture
	for _, c := range r.commands {
		if c.pipe != current {
			current = c.pipe
			p := &r.pipes[c.pipe]
			syscall.SyscallN(ctx.fn(17), ctx.this(), p.layout.this())   // IASetInputLayout
			syscall.SyscallN(ctx.fn(11), ctx.this(), p.vs.this(), 0, 0) // VSSetShader
			syscall.SyscallN(ctx.fn(9), ctx.this(), p.ps.this(), 0, 0)  // PSSetShader
			blend := r.blend
			if c.pipe == pipeSubpixel && r.dualSource {
				blend = r.blendDual
			}
			if blend != curBlend {
				curBlend = blend
				syscall.SyscallN(ctx.fn(35), ctx.this(), blend.this(), uintptr(unsafe.Pointer(&blendFactor[0])), 0xffffffff) // OMSetBlendState
			}
			buf := r.vbufs[c.pipe].buf
			stride, offset := p.stride, uint32(0)
			syscall.SyscallN(ctx.fn(18), ctx.this(), 0, 1, uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&stride)), uintptr(unsafe.Pointer(&offset))) // IASetVertexBuffers
		}
		if c.tex != nil && c.tex != boundTex {
			boundTex = c.tex
			srv := r.textures[c.tex.ID].srv
			syscall.SyscallN(ctx.fn(8), ctx.this(), 0, 1, uintptr(unsafe.Pointer(&srv))) // PSSetShaderResources
		}
		syscall.SyscallN(ctx.fn(21), ctx.this(), 4, uintptr(c.count), 0, uintptr(c.start)) // DrawInstanced
	}

	if r.capture != nil {
		fn := r.capture
		r.capture = nil
		fn(r.readBack())
	}

	// A frame for a new window size is presented immediately rather than at
	// the next vblank, so during a drag the content keeps up with the frame.
	// (Flip-model windowed presentation does not tear either way.)
	sync := uintptr(1)
	if r.noVSync || r.resized {
		sync = 0
	}
	r.resized = false
	t0 := time.Now()
	hr, _, _ = syscall.SyscallN(r.swap.fn(8), r.swap.this(), sync, 0) // Present
	r.presentWait = time.Since(t0)
	r.occluded = hr == dxgiStatusOccluded
	if r.occluded {
		return nil
	}
	return r.checkDevice("IDXGISwapChain::Present", hr)
}

func (r *Renderer) upload(p pipelineKind, data unsafe.Pointer, count int) error {
	if count == 0 {
		return nil
	}
	size := count * int(r.pipes[p].stride)
	vb := &r.vbufs[p]
	if vb.capacity < size {
		vb.buf.Release()
		capacity := max(size, 64*1024)
		for capacity < size*3/2 {
			capacity *= 2
		}
		desc := bufferDesc{ByteWidth: uint32(capacity), Usage: d3dUsageDynamic, BindFlags: d3dBindVertexBuffer, CPUAccessFlags: d3dCPUAccessWrite}
		var buf *com
		hr, _, _ := syscall.SyscallN(r.device.fn(3), r.device.this(), uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&buf))) // CreateBuffer
		if err := r.checkDevice("CreateBuffer(instances)", hr); err != nil {
			vb.buf, vb.capacity = nil, 0
			return err
		}
		vb.buf, vb.capacity = buf, capacity
	}
	var m mappedSubresource
	hr, _, _ := syscall.SyscallN(r.ctx.fn(14), r.ctx.this(), vb.buf.this(), 0, d3dMapWriteDiscard, 0, uintptr(unsafe.Pointer(&m))) // Map
	if err := r.checkDevice("Map(instances)", hr); err != nil {
		return err
	}
	copy(unsafe.Slice((*byte)(ptrFrom(m.PData)), size), unsafe.Slice((*byte)(data), size))
	syscall.SyscallN(r.ctx.fn(15), r.ctx.this(), vb.buf.this(), 0) // Unmap
	return nil
}

func (r *Renderer) syncTexture(t *scene.Texture) error {
	g := r.textures[t.ID]
	if g != nil && g.version == t.Version {
		return nil
	}
	if g == nil || g.width != t.Width || g.height != t.Height {
		if g != nil {
			g.srv.Release()
			g.tex.Release()
		}
		format := uint32(dxgiFormatR8Unorm)
		if t.Format == scene.FormatRGBA8 || t.Format == scene.FormatSubpixel8 {
			format = dxgiFormatRGBA8Unorm
		}
		desc := texture2DDesc{Width: uint32(t.Width), Height: uint32(t.Height), MipLevels: 1, ArraySize: 1, Format: format,
			SampleDesc: dxgiSampleDesc{Count: 1}, Usage: d3dUsageDefault, BindFlags: d3dBindShaderResource}
		init := subresourceData{PSysMem: uintptr(unsafe.Pointer(&t.Pix[0])), SysMemPitch: uint32(t.Width * t.BytesPerPixel())}
		g = &gpuTexture{width: t.Width, height: t.Height}
		hr, _, _ := syscall.SyscallN(r.device.fn(5), r.device.this(), uintptr(unsafe.Pointer(&desc)), uintptr(unsafe.Pointer(&init)), uintptr(unsafe.Pointer(&g.tex))) // CreateTexture2D
		if err := r.checkDevice(fmt.Sprintf("CreateTexture2D(%dx%d)", t.Width, t.Height), hr); err != nil {
			return err
		}
		hr, _, _ = syscall.SyscallN(r.device.fn(7), r.device.this(), g.tex.this(), 0, uintptr(unsafe.Pointer(&g.srv))) // CreateShaderResourceView
		if err := r.checkDevice("CreateShaderResourceView", hr); err != nil {
			g.tex.Release()
			return err
		}
		r.textures[t.ID] = g
	} else if t.DirtyX1 > t.DirtyX0 && t.DirtyY1 > t.DirtyY0 {
		bpp := t.BytesPerPixel()
		b := box{Left: uint32(t.DirtyX0), Top: uint32(t.DirtyY0), Front: 0, Right: uint32(t.DirtyX1), Bottom: uint32(t.DirtyY1), Back: 1}
		offset := (t.DirtyY0*t.Width + t.DirtyX0) * bpp
		syscall.SyscallN(r.ctx.fn(48), r.ctx.this(), g.tex.this(), 0, uintptr(unsafe.Pointer(&b)),
			uintptr(unsafe.Pointer(&t.Pix[offset])), uintptr(t.Width*bpp), 0) // UpdateSubresource
	}
	g.version = t.Version
	t.ClearDirty()
	return nil
}

func (r *Renderer) readBack() (*image.RGBA, error) {
	var back *com
	hr, _, _ := syscall.SyscallN(r.swap.fn(9), r.swap.this(), 0, uintptr(unsafe.Pointer(&iidID3D11Texture2D)), uintptr(unsafe.Pointer(&back)))
	if err := check("GetBuffer(capture)", hr); err != nil {
		return nil, err
	}
	defer back.Release()
	desc := texture2DDesc{Width: uint32(r.width), Height: uint32(r.height), MipLevels: 1, ArraySize: 1, Format: dxgiFormatBGRA8Unorm,
		SampleDesc: dxgiSampleDesc{Count: 1}, Usage: d3dUsageStaging, CPUAccessFlags: d3dCPUAccessRead}
	var staging *com
	hr, _, _ = syscall.SyscallN(r.device.fn(5), r.device.this(), uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&staging)))
	if err := check("CreateTexture2D(staging)", hr); err != nil {
		return nil, err
	}
	defer staging.Release()
	syscall.SyscallN(r.ctx.fn(47), r.ctx.this(), staging.this(), back.this()) // CopyResource
	var m mappedSubresource
	hr, _, _ = syscall.SyscallN(r.ctx.fn(14), r.ctx.this(), staging.this(), 0, d3dMapRead, 0, uintptr(unsafe.Pointer(&m)))
	if err := check("Map(staging)", hr); err != nil {
		return nil, err
	}
	defer syscall.SyscallN(r.ctx.fn(15), r.ctx.this(), staging.this(), 0)
	img := image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	src := unsafe.Slice((*byte)(ptrFrom(m.PData)), int(m.RowPitch)*r.height)
	for y := 0; y < r.height; y++ {
		row := src[y*int(m.RowPitch):]
		dst := img.Pix[y*img.Stride:]
		for x := 0; x < r.width; x++ {
			dst[x*4+0] = row[x*4+2]
			dst[x*4+1] = row[x*4+1]
			dst[x*4+2] = row[x*4+0]
			dst[x*4+3] = 255
		}
	}
	return img, nil
}

// Close implements render.Renderer. It releases the window's swap chain; the
// device stays for the other windows.
func (r *Renderer) Close() {
	if r.ctx != nil {
		syscall.SyscallN(r.ctx.fn(33), r.ctx.this(), 0, 0, 0) // OMSetRenderTargets(0, nil, nil)
	}
	r.releaseSwap()
	*r = Renderer{Device: r.Device}
}

// releaseSwap releases the window's swap chain, its render target and its
// waitable object. It is safe for a window whose chain was never created or
// was already released.
func (r *Renderer) releaseSwap() {
	if r.waitable != 0 {
		win32.CloseHandle(r.waitable)
		r.waitable = 0
	}
	r.rtv.Release()
	r.swap.Release()
	r.rtv, r.swap = nil, nil
}

// Recreate builds the window's swap chain and render target again on the
// device, after Device.Recover. The Renderer stays the same object, so the
// window's ui runtime and its state are untouched. The new chain has no size of
// its own here: the next BeginFrame resizes it to the window's current size.
func (r *Renderer) Recreate() error {
	if r.device == nil {
		return errDeviceReleased
	}
	r.releaseSwap()
	r.flip, r.swap1 = false, false
	r.width, r.height = 0, 0
	// Reset so the next frame sets the background color on the new chain.
	r.clear = scene.Color{}
	if err := r.createSwapChain(); err != nil {
		return err
	}
	return r.createTarget()
}

// Close releases the device and every GPU resource it owns. The windows
// rendering with it must be closed first.
func (d *Device) Close() {
	d.release()
	*d = Device{}
}

// release releases the device and every object it owns, and leaves the Device
// with an empty texture map. It accepts a Device whose creation failed part way
// or that was already released.
func (d *Device) release() {
	for _, t := range d.textures {
		t.srv.Release()
		t.tex.Release()
	}
	for i := range d.vbufs {
		d.vbufs[i].buf.Release()
	}
	for i := range d.pipes {
		d.pipes[i].layout.Release()
		d.pipes[i].vs.Release()
		d.pipes[i].ps.Release()
	}
	for _, s := range d.samplers {
		s.Release()
	}
	d.raster.Release()
	d.blend.Release()
	d.blendDual.Release()
	d.cbuf.Release()
	if d.ctx != nil {
		syscall.SyscallN(d.ctx.fn(110), d.ctx.this()) // ClearState
	}
	d.ctx.Release()
	d.factory.Release()
	d.device.Release()
	*d = Device{textures: map[uint64]*gpuTexture{}}
}

// checkDevice is check for a call on the device. When the device was removed
// or reset, the error wraps render.ErrDeviceLost. A call can also fail for
// another reason while the device is gone, so the device is asked for its
// removal reason before the error is returned as an ordinary one.
func (d *Device) checkDevice(what string, hr uintptr) error {
	err := check(what, hr)
	if err == nil || d.device == nil {
		return err
	}
	if h := hresult(hr); h == dxgiErrorDeviceRemoved || h == dxgiErrorDeviceReset {
		return fmt.Errorf("%w: %v", render.ErrDeviceLost, err)
	}
	// GetDeviceRemovedReason (ID3D11Device vtable index 39) is S_OK while the
	// device is healthy.
	if reason, _, _ := syscall.SyscallN(d.device.fn(39), d.device.this()); hresult(reason) != 0 {
		return fmt.Errorf("%w: %v", render.ErrDeviceLost, err)
	}
	return err
}

// ptrFrom converts a pointer returned by Direct3D (not Go memory).
func ptrFrom(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

// flipWaitable is the swap chain flag that gives a flip-model chain a waitable
// object. The other models have none.
func flipWaitable(effect uint32) uint32 {
	if effect == dxgiSwapEffectFlipDisc {
		return dxgiSwapChainFlagWaitable
	}
	return 0
}

// finishSwapChain keeps a new swap chain. When the chain has a frame latency
// waitable object, it allows one frame in flight on the chain alone, so each
// window paces itself and Present never blocks the thread. Otherwise the
// device limits the latency, as it always did.
func (r *Renderer) finishSwapChain(swap *com, flags uint32) {
	r.swap, r.swapFlags = swap, flags
	r.watchOcclusion()
	if flags&dxgiSwapChainFlagWaitable != 0 && r.attachWaitable() {
		return
	}
	r.swapFlags = flags &^ dxgiSwapChainFlagWaitable
	r.limitLatency()
}

// watchOcclusion asks DXGI to post win32.WM_OCCLUSION to the window when its
// occlusion state changes. The registration is made once per factory: DXGI
// keeps it for the factory's life, and a window that was covered is woken by
// it when it shows again. Without it the window would stop only until the
// next input.
func (r *Renderer) watchOcclusion() {
	if r.occlusionCookie != 0 && r.occlusionFactory == r.factory {
		return
	}
	var cookie uint32
	hr, _, _ := syscall.SyscallN(r.factory.fn(21), r.factory.this(), r.hwnd, win32.WM_OCCLUSION,
		uintptr(unsafe.Pointer(&cookie))) // IDXGIFactory2::RegisterOcclusionStatusWindow
	if hresult(hr).failed() {
		return
	}
	r.occlusionFactory, r.occlusionCookie = r.factory, cookie
}

// attachWaitable asks the swap chain for its waitable object and limits its
// frame latency to one. It reports false when the chain does not provide it.
func (r *Renderer) attachWaitable() bool {
	var sc2 *com
	hr, _, _ := syscall.SyscallN(r.swap.fn(0), r.swap.this(), uintptr(unsafe.Pointer(&iidIDXGISwapChain2)), uintptr(unsafe.Pointer(&sc2))) // QueryInterface
	if hresult(hr).failed() || sc2 == nil {
		return false
	}
	defer sc2.Release()
	if hr, _, _ := syscall.SyscallN(sc2.fn(31), sc2.this(), 1); hresult(hr).failed() { // SetMaximumFrameLatency
		return false
	}
	h, _, _ := syscall.SyscallN(sc2.fn(33), sc2.this()) // GetFrameLatencyWaitableObject
	if h == 0 {
		return false
	}
	r.waitable = h
	return true
}

// FrameWaitable returns the handle that is signaled when the window's swap
// chain can take another frame, or 0 when the chain has none. Wait on it
// before rendering, so Present does not block.
func (r *Renderer) FrameWaitable() uintptr { return r.waitable }
