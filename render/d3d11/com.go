//go:build windows

package d3d11

import (
	"fmt"
	"syscall"
	"unsafe"
)

// com is a COM interface pointer. Methods are called by vtable index; the
// indices used in this package are listed next to each call and come from the
// declaration order in d3d11.h / dxgi.h.
//
// All calls go through syscall.SyscallN directly at the call site so that
// uintptr(unsafe.Pointer(...)) conversions stay inside the syscall argument
// list, which is the only place Go guarantees them to be safe.
type com struct {
	vtbl *[512]uintptr
}

func (c *com) this() uintptr { return uintptr(unsafe.Pointer(c)) }

// fn returns the method pointer at vtable index i.
func (c *com) fn(i int) uintptr { return c.vtbl[i] }

func (c *com) Release() {
	if c != nil {
		syscall.SyscallN(c.fn(2), c.this())
	}
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	iidID3D11Texture2D = guid{0x6f15aaf2, 0xd208, 0x4e89, [8]byte{0x9a, 0xb4, 0x48, 0x95, 0x35, 0xd3, 0x4f, 0x9c}}
	iidIDXGIDevice1    = guid{0x77db970f, 0x6276, 0x48ba, [8]byte{0xba, 0x28, 0x07, 0x01, 0x43, 0xb4, 0x39, 0x2c}}
	iidIDXGIDevice     = guid{0x54ec77fa, 0x1377, 0x44e6, [8]byte{0x8c, 0x32, 0x88, 0xfd, 0x5f, 0x44, 0xc8, 0x4c}}
	iidIDXGISwapChain2 = guid{0xa8be2ac4, 0x199f, 0x4946, [8]byte{0xb3, 0x31, 0x79, 0x59, 0x9f, 0xb9, 0x8d, 0xe7}}
	iidIDXGIFactory2   = guid{0x50c83a1c, 0xe072, 0x4c48, [8]byte{0x87, 0xb0, 0x36, 0x30, 0xfa, 0x36, 0xa6, 0xd0}}
)

type hresult uint32

func (h hresult) failed() bool { return int32(h) < 0 }

// check turns a failing HRESULT into an error that says what was attempted.
func check(what string, r uintptr) error {
	h := hresult(r)
	if !h.failed() {
		return nil
	}
	return fmt.Errorf("d3d11: %s failed with HRESULT 0x%08X%s", what, uint32(h), hint(h))
}

func hint(h hresult) string {
	switch uint32(h) {
	case 0x887A0005:
		return " (DXGI_ERROR_DEVICE_REMOVED: the GPU was reset or the driver crashed)"
	case 0x887A0001:
		return " (DXGI_ERROR_INVALID_CALL: a parameter or struct layout is wrong)"
	case 0x80070057:
		return " (E_INVALIDARG)"
	case 0x887A0004:
		return " (DXGI_ERROR_UNSUPPORTED: the GPU or driver lacks a required feature)"
	}
	return ""
}

// D3D11 / DXGI constants used by the renderer.
const (
	d3dDriverTypeHardware = 1
	d3dDriverTypeWARP     = 5
	d3dCreateBGRASupport  = 0x20
	d3dCreateDebug        = 0x2
	d3dSDKVersion         = 7

	dxgiFormatUnknown     = 0
	dxgiFormatRGBA32Float = 2
	dxgiFormatRGBA8Unorm  = 28
	dxgiFormatBGRA8Unorm  = 87
	dxgiFormatR8Unorm     = 61
	dxgiUsageRenderTarget = 0x20
	dxgiSwapEffectDiscard = 0
	// DXGI_SWAP_CHAIN_FLAG_FRAME_LATENCY_WAITABLE_OBJECT: the chain has a
	// waitable object (flip model only).
	dxgiSwapChainFlagWaitable = 0x40
	dxgiSwapEffectFlipDisc    = 4
	dxgiScalingNone           = 1
	dxgiMWANoAltEnter         = 2
	dxgiStatusOccluded        = 0x087A0001
	d3dUsageDefault           = 0
	d3dUsageDynamic           = 2
	d3dUsageStaging           = 3
	d3dBindVertexBuffer       = 0x1
	d3dBindConstantBuffer     = 0x4
	d3dBindShaderResource     = 0x8
	d3dCPUAccessWrite         = 0x10000
	d3dCPUAccessRead          = 0x20000
	d3dMapRead                = 1
	d3dMapWriteDiscard        = 4
	d3dInputPerInstance       = 1
	d3dFilterPoint            = 0
	d3dFilterLinear           = 0x15
	d3dAddressClamp           = 3
	d3dComparisonNever        = 1
	d3dBlendOne               = 2
	d3dBlendInvSrcAlpha       = 6
	d3dBlendInvSrc1Color      = 17
	d3dBlendInvSrc1Alpha      = 19
	d3dBlendOpAdd             = 1
	d3dFillSolid              = 3
	d3dCullNone               = 1
	d3dTopologyTriangleStrip  = 5
)

type dxgiModeDesc struct {
	Width, Height    uint32
	RefreshNum       uint32
	RefreshDen       uint32
	Format           uint32
	ScanlineOrdering uint32
	Scaling          uint32
}

type dxgiSampleDesc struct{ Count, Quality uint32 }

type dxgiSwapChainDesc struct {
	BufferDesc   dxgiModeDesc
	SampleDesc   dxgiSampleDesc
	BufferUsage  uint32
	BufferCount  uint32
	OutputWindow uintptr
	Windowed     int32
	SwapEffect   uint32
	Flags        uint32
}

type dxgiSwapChainDesc1 struct {
	Width, Height uint32
	Format        uint32
	Stereo        int32
	SampleDesc    dxgiSampleDesc
	BufferUsage   uint32
	BufferCount   uint32
	Scaling       uint32
	SwapEffect    uint32
	AlphaMode     uint32
	Flags         uint32
}

type bufferDesc struct {
	ByteWidth           uint32
	Usage               uint32
	BindFlags           uint32
	CPUAccessFlags      uint32
	MiscFlags           uint32
	StructureByteStride uint32
}

type texture2DDesc struct {
	Width, Height  uint32
	MipLevels      uint32
	ArraySize      uint32
	Format         uint32
	SampleDesc     dxgiSampleDesc
	Usage          uint32
	BindFlags      uint32
	CPUAccessFlags uint32
	MiscFlags      uint32
}

type inputElementDesc struct {
	SemanticName         *byte
	SemanticIndex        uint32
	Format               uint32
	InputSlot            uint32
	AlignedByteOffset    uint32
	InputSlotClass       uint32
	InstanceDataStepRate uint32
}

type samplerDesc struct {
	Filter         uint32
	AddressU       uint32
	AddressV       uint32
	AddressW       uint32
	MipLODBias     float32
	MaxAnisotropy  uint32
	ComparisonFunc uint32
	BorderColor    [4]float32
	MinLOD, MaxLOD float32
}

type renderTargetBlendDesc struct {
	BlendEnable           int32
	SrcBlend, DestBlend   uint32
	BlendOp               uint32
	SrcBlendAlpha         uint32
	DestBlendAlpha        uint32
	BlendOpAlpha          uint32
	RenderTargetWriteMask uint8
}

type blendDesc struct {
	AlphaToCoverageEnable  int32
	IndependentBlendEnable int32
	RenderTarget           [8]renderTargetBlendDesc
}

type rasterizerDesc struct {
	FillMode              uint32
	CullMode              uint32
	FrontCounterClockwise int32
	DepthBias             int32
	DepthBiasClamp        float32
	SlopeScaledDepthBias  float32
	DepthClipEnable       int32
	ScissorEnable         int32
	MultisampleEnable     int32
	AntialiasedLineEnable int32
}

type viewport struct {
	TopLeftX, TopLeftY, Width, Height, MinDepth, MaxDepth float32
}

type mappedSubresource struct {
	PData      uintptr
	RowPitch   uint32
	DepthPitch uint32
}

type box struct {
	Left, Top, Front, Right, Bottom, Back uint32
}

type subresourceData struct {
	PSysMem          uintptr
	SysMemPitch      uint32
	SysMemSlicePitch uint32
}

// Compile-time layout checks against the C ABI sizes.
var (
	_ [48 - unsafe.Sizeof(dxgiSwapChainDesc1{})]struct{}
	_ [unsafe.Sizeof(dxgiSwapChainDesc1{}) - 48]struct{}
	_ [72 - unsafe.Sizeof(dxgiSwapChainDesc{})]struct{}
	_ [unsafe.Sizeof(dxgiSwapChainDesc{}) - 72]struct{}
	_ [32 - unsafe.Sizeof(inputElementDesc{})]struct{}
	_ [unsafe.Sizeof(inputElementDesc{}) - 32]struct{}
	_ [264 - unsafe.Sizeof(blendDesc{})]struct{}
	_ [unsafe.Sizeof(blendDesc{}) - 264]struct{}
	_ [52 - unsafe.Sizeof(samplerDesc{})]struct{}
	_ [unsafe.Sizeof(samplerDesc{}) - 52]struct{}
	_ [44 - unsafe.Sizeof(texture2DDesc{})]struct{}
	_ [unsafe.Sizeof(texture2DDesc{}) - 44]struct{}
)
