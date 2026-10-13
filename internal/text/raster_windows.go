//go:build windows

package text

import (
	"image"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/image/font/sfnt"

	"github.com/tyler-builds/jamb/internal/scene"
)

// DirectWrite glyph rasterization. DirectWrite applies the font's hinting
// instructions, so stems and the x-height and cap-height edges land on whole
// pixels where the pure-Go rasterizer (system.go) would smear them over two.
// The COM calls are plain vtable calls through syscall.SyscallN, in the style
// of render/d3d11/com.go; vtable indices come from dwrite.h, dwrite_1.h and
// dwrite_2.h and are listed next to each call.

type dwCom struct {
	vtbl *[64]uintptr
}

func (c *dwCom) this() uintptr { return uintptr(unsafe.Pointer(c)) }

// fn returns the method pointer at vtable index i.
func (c *dwCom) fn(i int) uintptr { return c.vtbl[i] }

func (c *dwCom) release() {
	if c != nil {
		syscall.SyscallN(c.fn(2), c.this())
	}
}

type dwGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// IID_IDWriteFactory2.
var iidIDWriteFactory2 = dwGUID{0x0439fc60, 0xca44, 0x4994, [8]byte{0x8d, 0xee, 0x3a, 0x9a, 0xf7, 0xb7, 0x32, 0xec}}

const (
	dwFactoryTypeShared = 0 // DWRITE_FACTORY_TYPE_SHARED

	dwSimulationsNone = 0 // DWRITE_FONT_SIMULATIONS_NONE

	dwRenderingNaturalSymmetric = 5 // DWRITE_RENDERING_MODE_NATURAL_SYMMETRIC
	dwMeasuringNatural          = 0 // DWRITE_MEASURING_MODE_NATURAL
	dwGridFitEnabled            = 2 // DWRITE_GRID_FIT_MODE_ENABLED
	dwAntialiasGrayscale        = 1 // DWRITE_TEXT_ANTIALIAS_MODE_GRAYSCALE
	dwTextureAliased1x1         = 0 // DWRITE_TEXTURE_ALIASED_1x1
)

// dwGlyphRun is DWRITE_GLYPH_RUN (48 bytes on amd64).
type dwGlyphRun struct {
	FontFace      uintptr
	FontEmSize    float32
	GlyphCount    uint32
	GlyphIndices  *uint16
	GlyphAdvances *float32
	GlyphOffsets  *[2]float32
	IsSideways    int32
	BidiLevel     uint32
}

type dwRect struct{ Left, Top, Right, Bottom int32 }

var _ [48 - unsafe.Sizeof(dwGlyphRun{})]struct{}
var _ [unsafe.Sizeof(dwGlyphRun{}) - 48]struct{}

var (
	dwriteDLL       = syscall.NewLazyDLL("dwrite.dll")
	procCreateFact  = dwriteDLL.NewProc("DWriteCreateFactory")
	dwFactoryOnce   sync.Once
	dwFactoryHandle *dwCom
)

// dwFactory returns the shared IDWriteFactory2, created on first use, or nil
// when DirectWrite (or its Windows 8.1 factory) is unavailable.
func dwFactory() *dwCom {
	dwFactoryOnce.Do(func() {
		if procCreateFact.Find() != nil {
			return
		}
		var f *dwCom
		hr, _, _ := procCreateFact.Call(dwFactoryTypeShared, uintptr(unsafe.Pointer(&iidIDWriteFactory2)), uintptr(unsafe.Pointer(&f)))
		if int32(hr) >= 0 && f != nil {
			dwFactoryHandle = f
		}
	})
	return dwFactoryHandle
}

// dwFace caches the IDWriteFontFace of a Font, and whether it has color
// glyphs.
type dwFace struct {
	once      sync.Once
	face      *dwCom
	colorOnce sync.Once
	color     bool
}

// dwriteFace returns the DirectWrite face for f, or nil when f is not backed by
// a font file or DirectWrite rejects it.
func (f *Font) dwriteFace() *dwCom {
	f.dw.once.Do(func() {
		if f.path == "" {
			return
		}
		fac := dwFactory()
		if fac == nil {
			return
		}
		path, err := syscall.UTF16PtrFromString(f.path)
		if err != nil {
			return
		}
		// IDWriteFactory::CreateFontFileReference, vtable 7.
		var file *dwCom
		hr, _, _ := syscall.SyscallN(fac.fn(7), fac.this(), uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&file)))
		if int32(hr) < 0 || file == nil {
			return
		}
		defer file.release()
		// IDWriteFontFile::Analyze, vtable 5.
		var supported int32
		var fileType, faceType, numFaces uint32
		hr, _, _ = syscall.SyscallN(file.fn(5), file.this(), uintptr(unsafe.Pointer(&supported)),
			uintptr(unsafe.Pointer(&fileType)), uintptr(unsafe.Pointer(&faceType)), uintptr(unsafe.Pointer(&numFaces)))
		if int32(hr) < 0 || supported == 0 || uint32(f.faceIndex) >= numFaces {
			return
		}
		// IDWriteFactory::CreateFontFace, vtable 9.
		var face *dwCom
		hr, _, _ = syscall.SyscallN(fac.fn(9), fac.this(), uintptr(faceType), 1, uintptr(unsafe.Pointer(&file)),
			uintptr(f.faceIndex), dwSimulationsNone, uintptr(unsafe.Pointer(&face)))
		if int32(hr) < 0 || face == nil {
			return
		}
		f.dw.face = face
	})
	return f.dw.face
}

// dwriteGlyph rasterizes glyph id of f at size physical pixels with its origin
// fx (0..1) pixels right of a pixel boundary. alloc supplies the zeroed mask
// storage. It returns the offset of the mask's top-left from the origin; m is
// nil for an empty glyph. ok is false when DirectWrite cannot be used and the
// caller should rasterize the outline itself.
func dwriteGlyph(f *Font, id sfnt.GlyphIndex, size, fx float32, alloc func(w, h int) *image.Alpha) (dx, dy int, m *image.Alpha, ok bool) {
	face := f.dwriteFace()
	if face == nil || id > 0xFFFF {
		return 0, 0, nil, false
	}
	fac := dwFactory()
	gi := uint16(id)
	run := dwGlyphRun{FontFace: face.this(), FontEmSize: size, GlyphCount: 1, GlyphIndices: &gi}
	// IDWriteFactory2::CreateGlyphRunAnalysis, vtable 30 (IDWriteFactory 3..23,
	// IDWriteFactory1 24..25, IDWriteFactory2 26..30). The two float arguments
	// are passed as their bit patterns in stack slots.
	var an *dwCom
	hr, _, _ := syscall.SyscallN(fac.fn(30), fac.this(), uintptr(unsafe.Pointer(&run)), 0,
		dwRenderingNaturalSymmetric, dwMeasuringNatural, dwGridFitEnabled, dwAntialiasGrayscale,
		uintptr(math.Float32bits(fx)), 0, uintptr(unsafe.Pointer(&an)))
	if int32(hr) < 0 || an == nil {
		return 0, 0, nil, false
	}
	defer an.release()
	// IDWriteGlyphRunAnalysis::GetAlphaTextureBounds, vtable 3.
	var r dwRect
	hr, _, _ = syscall.SyscallN(an.fn(3), an.this(), dwTextureAliased1x1, uintptr(unsafe.Pointer(&r)))
	if int32(hr) < 0 {
		return 0, 0, nil, false
	}
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	if w <= 0 || h <= 0 {
		return 0, 0, nil, true // whitespace
	}
	m = alloc(w, h)
	// IDWriteGlyphRunAnalysis::CreateAlphaTexture, vtable 4. Grayscale
	// antialiasing yields one coverage byte per pixel.
	hr, _, _ = syscall.SyscallN(an.fn(4), an.this(), dwTextureAliased1x1, uintptr(unsafe.Pointer(&r)),
		uintptr(unsafe.Pointer(&m.Pix[0])), uintptr(w*h))
	if int32(hr) < 0 {
		return 0, 0, nil, false
	}
	return int(r.Left), int(r.Top), m, true
}

// dwColorGlyphRun is DWRITE_COLOR_GLYPH_RUN (88 bytes on amd64): one layer of
// a color glyph, and its color.
type dwColorGlyphRun struct {
	GlyphRun     dwGlyphRun
	Description  uintptr
	BaselineX    float32
	BaselineY    float32
	Color        [4]float32 // R, G, B, A
	PaletteIndex uint16
}

var _ [88 - unsafe.Sizeof(dwColorGlyphRun{})]struct{}
var _ [unsafe.Sizeof(dwColorGlyphRun{}) - 88]struct{}

const (
	dwENoColor        = 0x8898500C // DWRITE_E_NOCOLOR: the run has no color glyphs
	dwForegroundColor = 0xFFFF     // a layer's palette index for the text color
)

// hasColor reports whether f has color glyphs: whether it has an OpenType
// COLR table, as Segoe UI Emoji does.
func (f *Font) hasColor() bool {
	f.dw.colorOnce.Do(func() {
		face := f.dwriteFace()
		if face == nil {
			return
		}
		// IDWriteFontFace::TryGetFontTable, vtable 12, and ReleaseFontTable, 13.
		// The tag's first letter is its low byte.
		const colr = 'C' | 'O'<<8 | 'L'<<16 | 'R'<<24
		var data, ctx uintptr
		var size uint32
		var exists int32
		hr, _, _ := syscall.SyscallN(face.fn(12), face.this(), colr, uintptr(unsafe.Pointer(&data)),
			uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&ctx)), uintptr(unsafe.Pointer(&exists)))
		if int32(hr) >= 0 && exists != 0 {
			f.dw.color = true
			syscall.SyscallN(face.fn(13), face.this(), ctx)
		}
	})
	return f.dw.color
}

// dwriteColorGlyph rasterizes glyph id of the color font f as it is in color:
// DirectWrite splits it into layers, each a glyph of one color, and they are
// drawn over each other, in order, into a premultiplied RGBA image. Layers of
// the text color take fg (straight R, G, B). It takes the arguments of
// dwriteGlyph, and ok is false when the glyph has no color layers.
func dwriteColorGlyph(f *Font, id sfnt.GlyphIndex, size, fx float32, fg [3]float32, alloc func(w, h int) *image.RGBA) (dx, dy int, m *image.RGBA, ok bool) {
	face := f.dwriteFace()
	if face == nil || id > 0xFFFF {
		return 0, 0, nil, false
	}
	fac := dwFactory()
	gi := uint16(id)
	run := dwGlyphRun{FontFace: face.this(), FontEmSize: size, GlyphCount: 1, GlyphIndices: &gi}
	// IDWriteFactory2::TranslateColorGlyphRun, vtable 28. The baseline origin's
	// two floats are the first arguments after this, which Windows takes in
	// XMM1 and XMM2; Go's system call copies the first four arguments into the
	// XMM registers as well.
	var layers *dwCom
	hr, _, _ := syscall.SyscallN(fac.fn(28), fac.this(), uintptr(math.Float32bits(fx)), 0,
		uintptr(unsafe.Pointer(&run)), 0, dwMeasuringNatural, 0, 0, uintptr(unsafe.Pointer(&layers)))
	if int32(hr) < 0 || layers == nil {
		return 0, 0, nil, false // dwENoColor: an ordinary glyph
	}
	defer layers.release()
	type layer struct {
		r     dwRect
		mask  []byte
		color [4]float32
	}
	var all []layer
	var bounds dwRect
	for {
		// IDWriteColorGlyphRunEnumerator::MoveNext, vtable 3, and
		// GetCurrentRun, 4. The run lives until the next MoveNext.
		var has int32
		hr, _, _ = syscall.SyscallN(layers.fn(3), layers.this(), uintptr(unsafe.Pointer(&has)))
		if int32(hr) < 0 {
			return 0, 0, nil, false
		}
		if has == 0 {
			break
		}
		var cr *dwColorGlyphRun
		hr, _, _ = syscall.SyscallN(layers.fn(4), layers.this(), uintptr(unsafe.Pointer(&cr)))
		if int32(hr) < 0 || cr == nil {
			return 0, 0, nil, false
		}
		c := cr.Color
		if cr.PaletteIndex == dwForegroundColor {
			c = [4]float32{fg[0], fg[1], fg[2], 1}
		}
		var an *dwCom
		hr, _, _ = syscall.SyscallN(fac.fn(30), fac.this(), uintptr(unsafe.Pointer(&cr.GlyphRun)), 0,
			dwRenderingNaturalSymmetric, dwMeasuringNatural, dwGridFitEnabled, dwAntialiasGrayscale,
			uintptr(math.Float32bits(cr.BaselineX)), uintptr(math.Float32bits(cr.BaselineY)), uintptr(unsafe.Pointer(&an)))
		if int32(hr) < 0 || an == nil {
			return 0, 0, nil, false
		}
		var r dwRect
		hr, _, _ = syscall.SyscallN(an.fn(3), an.this(), dwTextureAliased1x1, uintptr(unsafe.Pointer(&r)))
		w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
		if int32(hr) < 0 || w <= 0 || h <= 0 {
			an.release()
			continue // an empty layer
		}
		mask := make([]byte, w*h)
		hr, _, _ = syscall.SyscallN(an.fn(4), an.this(), dwTextureAliased1x1, uintptr(unsafe.Pointer(&r)),
			uintptr(unsafe.Pointer(&mask[0])), uintptr(len(mask)))
		an.release()
		if int32(hr) < 0 {
			return 0, 0, nil, false
		}
		if len(all) == 0 {
			bounds = r
		} else {
			bounds = dwRect{min(bounds.Left, r.Left), min(bounds.Top, r.Top), max(bounds.Right, r.Right), max(bounds.Bottom, r.Bottom)}
		}
		all = append(all, layer{r, mask, c})
	}
	if len(all) == 0 {
		return 0, 0, nil, true // whitespace
	}
	w, h := int(bounds.Right-bounds.Left), int(bounds.Bottom-bounds.Top)
	m = alloc(w, h)
	for _, l := range all {
		compositeLayer(m, l.mask, int(l.r.Right-l.r.Left), int(l.r.Left-bounds.Left), int(l.r.Top-bounds.Top), l.color)
	}
	return int(bounds.Left), int(bounds.Top), m, true
}

const (
	dwAntialiasClearType = 0 // DWRITE_TEXT_ANTIALIAS_MODE_CLEARTYPE
	dwTextureClearType   = 1 // DWRITE_TEXTURE_CLEARTYPE_3x1
)

// dwriteGlyphClearType is dwriteGlyph for ClearType: the mask has R, G and B
// coverage in the first three bytes of each texel (reversed to B, G, R when bgr
// is set, for BGR panels). DirectWrite's bounds are about a pixel wider on each
// side than the grayscale ones because of the ClearType filter.
func dwriteGlyphClearType(f *Font, id sfnt.GlyphIndex, size, fx float32, bgr bool, alloc func(w, h int) *image.RGBA) (dx, dy int, m *image.RGBA, ok bool) {
	face := f.dwriteFace()
	if face == nil || id > 0xFFFF {
		return 0, 0, nil, false
	}
	fac := dwFactory()
	gi := uint16(id)
	run := dwGlyphRun{FontFace: face.this(), FontEmSize: size, GlyphCount: 1, GlyphIndices: &gi}
	var an *dwCom
	hr, _, _ := syscall.SyscallN(fac.fn(30), fac.this(), uintptr(unsafe.Pointer(&run)), 0,
		dwRenderingNaturalSymmetric, dwMeasuringNatural, dwGridFitEnabled, dwAntialiasClearType,
		uintptr(math.Float32bits(fx)), 0, uintptr(unsafe.Pointer(&an)))
	if int32(hr) < 0 || an == nil {
		return 0, 0, nil, false
	}
	defer an.release()
	var r dwRect
	hr, _, _ = syscall.SyscallN(an.fn(3), an.this(), dwTextureClearType, uintptr(unsafe.Pointer(&r)))
	if int32(hr) < 0 {
		return 0, 0, nil, false
	}
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	if w <= 0 || h <= 0 {
		return 0, 0, nil, true // whitespace
	}
	// Three coverage bytes per pixel, expanded into the RGBA texel below.
	rgb := make([]byte, w*h*3)
	hr, _, _ = syscall.SyscallN(an.fn(4), an.this(), dwTextureClearType, uintptr(unsafe.Pointer(&r)),
		uintptr(unsafe.Pointer(&rgb[0])), uintptr(len(rgb)))
	if int32(hr) < 0 {
		return 0, 0, nil, false
	}
	m = alloc(w, h)
	for i := 0; i < w*h; i++ {
		c0, c1, c2 := rgb[i*3], rgb[i*3+1], rgb[i*3+2]
		if bgr {
			c0, c2 = c2, c0
		}
		m.Pix[i*4], m.Pix[i*4+1], m.Pix[i*4+2], m.Pix[i*4+3] = c0, c1, c2, c1
	}
	return int(r.Left), int(r.Top), m, true
}

var (
	user32DLL                = syscall.NewLazyDLL("user32.dll")
	procSystemParametersInfo = user32DLL.NewProc("SystemParametersInfoW")
)

const (
	spiGetFontSmoothing            = 0x004A
	spiGetFontSmoothingType        = 0x200A
	spiGetFontSmoothingOrientation = 0x2012
	feFontSmoothingClearType       = 2
	feFontSmoothingOrientationBGR  = 0
)

func spiUint(action uintptr) (uint32, bool) {
	var v uint32
	r, _, _ := procSystemParametersInfo.Call(action, 0, uintptr(unsafe.Pointer(&v)), 0)
	return v, r != 0
}

// platformSubpixel reports the user's ClearType settings: whether font
// smoothing is on and of the ClearType type, the panel's subpixel order and
// the DirectWrite rendering parameters (the ClearType Tuner's gamma, enhanced
// contrast and level).
func platformSubpixel() (s subpixelSettings, ok bool) {
	if on, ok := spiUint(spiGetFontSmoothing); !ok || on == 0 {
		return s, false
	}
	if typ, ok := spiUint(spiGetFontSmoothingType); !ok || typ != feFontSmoothingClearType {
		return s, false
	}
	if o, ok := spiUint(spiGetFontSmoothingOrientation); ok && o == feFontSmoothingOrientationBGR {
		s.BGR = true
	}
	s.Params = scene.TextParams{Gamma: 2.2, Contrast: 0.5, Level: 1}
	if fac := dwFactory(); fac != nil {
		// IDWriteFactory::CreateRenderingParams, vtable 10; then
		// IDWriteRenderingParams GetGamma 3, GetEnhancedContrast 4,
		// GetClearTypeLevel 5, GetPixelGeometry 6. Float results come back
		// in XMM0, which SyscallN returns as r2.
		var rp *dwCom
		hr, _, _ := syscall.SyscallN(fac.fn(10), fac.this(), uintptr(unsafe.Pointer(&rp)))
		if int32(hr) >= 0 && rp != nil {
			defer rp.release()
			_, g, _ := syscall.SyscallN(rp.fn(3), rp.this())
			_, c, _ := syscall.SyscallN(rp.fn(4), rp.this())
			_, l, _ := syscall.SyscallN(rp.fn(5), rp.this())
			geometry, _, _ := syscall.SyscallN(rp.fn(6), rp.this())
			s.Params = scene.TextParams{
				Gamma:    math.Float32frombits(uint32(g)),
				Contrast: math.Float32frombits(uint32(c)),
				Level:    math.Float32frombits(uint32(l)),
			}
			if uint32(geometry) == 0 { // DWRITE_PIXEL_GEOMETRY_FLAT
				return s, false
			}
		}
	}
	return s, true
}
