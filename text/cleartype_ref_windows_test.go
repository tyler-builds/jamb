//go:build windows

package text

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"github.com/tyler-builds/jamb/render/d3d11"
	"github.com/tyler-builds/jamb/scene"
)

// These tests compare jamb's ClearType output with DirectWrite's own: the same
// glyphs at the same positions are drawn by IDWriteBitmapRenderTarget (the
// native DirectWrite/Direct2D ClearType blend, with the user's rendering
// parameters) and by the jamb pipeline (DirectWrite coverage masks, the
// subpixel shader and dual-source blending on a real D3D11 device). The mean
// absolute difference is in 8-bit levels per channel.
//
//	go test ./text -run ClearTypeReference -v
//
// Set JAMB_REF_DUMP=dir to write 6x enlarged PNGs of ours, native and the diff.

var (
	gdi32DLL            = syscall.NewLazyDLL("gdi32.dll")
	procCreateSolidBrsh = gdi32DLL.NewProc("CreateSolidBrush")
	procDeleteObject    = gdi32DLL.NewProc("DeleteObject")
	procGetCurrentObj   = gdi32DLL.NewProc("GetCurrentObject")
	procGetObjectW      = gdi32DLL.NewProc("GetObjectW")
	procFillRect        = user32DLL.NewProc("FillRect")
	procCreateWindowEx  = user32DLL.NewProc("CreateWindowExW")
	procDestroyWindow   = user32DLL.NewProc("DestroyWindow")
)

type refCase struct {
	name   string
	text   string
	size   float32
	fg, bg [3]uint8
}

var refCases = []refCase{
	{"dark-ui15", "English with ligatures: office, efficient, final, fluffy", 15, [3]uint8{0xE8, 0xE8, 0xE8}, [3]uint8{0x0E, 0x0E, 0x10}},
	{"dark-ui13", "The quick brown fox jumps over the lazy dog 0123456789", 13, [3]uint8{0xE8, 0xE8, 0xE8}, [3]uint8{0x0E, 0x0E, 0x10}},
	{"dark-dim15", "Settings, shortcuts and a subtle secondary caption", 15, [3]uint8{0x9A, 0x9A, 0xA2}, [3]uint8{0x0E, 0x0E, 0x10}},
	{"light-ui15", "English with ligatures: office, efficient, final, fluffy", 15, [3]uint8{0x1A, 0x1A, 0x1A}, [3]uint8{0xFF, 0xFF, 0xFF}},
	{"light-ui13", "The quick brown fox jumps over the lazy dog 0123456789", 13, [3]uint8{0x1A, 0x1A, 0x1A}, [3]uint8{0xFF, 0xFF, 0xFF}},
	{"light-black24", "Large heading text 24px", 24, [3]uint8{0, 0, 0}, [3]uint8{0xF4, 0xF4, 0xF6}},
	{"light-gray15", "Placeholder text in a mid gray, 15 px wide", 15, [3]uint8{0x66, 0x66, 0x66}, [3]uint8{0xFF, 0xFF, 0xFF}},
	{"dark-silver15", "Silver text on the dark theme background", 15, [3]uint8{0xC0, 0xC0, 0xC0}, [3]uint8{0x0E, 0x0E, 0x10}},
	{"dark-on-mid", "Dark text on a mid gray background 15", 15, [3]uint8{0x10, 0x10, 0x10}, [3]uint8{0x90, 0x90, 0x90}},
	{"white-on-blue", "White text on a saturated blue button", 14, [3]uint8{0xFF, 0xFF, 0xFF}, [3]uint8{0x1F, 0x4F, 0xCF}},
	{"light-333", "Body copy in a soft near black color, 14", 14, [3]uint8{0x33, 0x33, 0x33}, [3]uint8{0xFA, 0xFA, 0xFA}},
	{"dark-888", "Muted mid gray caption on dark ground", 14, [3]uint8{0x88, 0x88, 0x88}, [3]uint8{0x0E, 0x0E, 0x10}},
	{"accent-on-dark", "Accent colored link text, 15 px", 15, [3]uint8{0x4C, 0x8D, 0xFF}, [3]uint8{0x1B, 0x1B, 0x1F}},
}

const refW, refH = 520, 40

// refGlyph is one positioned glyph, snapped the way System.Draw snaps it.
type refGlyph struct {
	font *Font
	id   uint16
	x, y float32 // baseline origin in pixels (x on the subpixel grid)
}

func refGlyphs(l *Layout, x, y, scale float32) []refGlyph {
	var out []refGlyph
	for _, ln := range l.Lines {
		for _, run := range ln.Runs {
			for _, g := range run.Glyphs {
				if g.font == nil {
					continue
				}
				px := (x + run.X + g.X) * scale
				ix := float32(math.Floor(float64(px)))
				bin := min(int((px-ix)*subpixelBins), subpixelBins-1)
				py := float32(math.Round(float64((y + ln.Baseline + g.Y) * scale)))
				out = append(out, refGlyph{g.font, uint16(g.ID), ix + float32(bin)/subpixelBins, py})
			}
		}
	}
	return out
}

// nativeClearType draws the glyphs with IDWriteBitmapRenderTarget::DrawGlyphRun
// using NATURAL_SYMMETRIC ClearType rendering parameters p.
func nativeClearType(t *testing.T, gl []refGlyph, c refCase, p scene.TextParams, bgr bool) *image.RGBA {
	t.Helper()
	fac := dwFactory()
	// IDWriteFactory::GetGdiInterop 17; IDWriteGdiInterop::CreateBitmapRenderTarget 7.
	var interop, target, params *dwCom
	if hr, _, _ := syscall.SyscallN(fac.fn(17), fac.this(), uintptr(unsafe.Pointer(&interop))); int32(hr) < 0 {
		t.Skipf("GetGdiInterop: %#x", uint32(hr))
	}
	defer interop.release()
	if hr, _, _ := syscall.SyscallN(interop.fn(7), interop.this(), 0, refW, refH, uintptr(unsafe.Pointer(&target))); int32(hr) < 0 {
		t.Skipf("CreateBitmapRenderTarget: %#x", uint32(hr))
	}
	defer target.release()
	geometry := uintptr(1) // DWRITE_PIXEL_GEOMETRY_RGB
	if bgr {
		geometry = 2
	}
	// IDWriteFactory::CreateCustomRenderingParams 12, rendering mode NATURAL_SYMMETRIC (5).
	if hr, _, _ := syscall.SyscallN(fac.fn(12), fac.this(), uintptr(math.Float32bits(p.Gamma)), uintptr(math.Float32bits(p.Contrast)),
		uintptr(math.Float32bits(p.Level)), geometry, dwRenderingNaturalSymmetric, uintptr(unsafe.Pointer(&params))); int32(hr) < 0 {
		t.Skipf("CreateCustomRenderingParams: %#x", uint32(hr))
	}
	defer params.release()
	// IDWriteBitmapRenderTarget::GetMemoryDC 4, DrawGlyphRun 3.
	dc, _, _ := syscall.SyscallN(target.fn(4), target.this())
	rgb := func(c [3]uint8) uintptr { return uintptr(c[0]) | uintptr(c[1])<<8 | uintptr(c[2])<<16 }
	brush, _, _ := procCreateSolidBrsh.Call(rgb(c.bg))
	rc := dwRect{0, 0, refW, refH}
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&rc)), brush)
	procDeleteObject.Call(brush)
	for _, g := range gl {
		id := g.id
		run := dwGlyphRun{FontFace: g.font.dwriteFace().this(), FontEmSize: c.size, GlyphCount: 1, GlyphIndices: &id}
		hr, _, _ := syscall.SyscallN(target.fn(3), target.this(), uintptr(math.Float32bits(g.x)), uintptr(math.Float32bits(g.y)),
			dwMeasuringNatural, uintptr(unsafe.Pointer(&run)), params.this(), rgb(c.fg), 0)
		if int32(hr) < 0 {
			t.Fatalf("DrawGlyphRun: %#x", uint32(hr))
		}
	}
	// Read the DIB section behind the DC.
	hbm, _, _ := procGetCurrentObj.Call(dc, 7) // OBJ_BITMAP
	var ds [104]byte                           // DIBSECTION
	procGetObjectW.Call(hbm, uintptr(len(ds)), uintptr(unsafe.Pointer(&ds[0])))
	bits := *(*uintptr)(unsafe.Pointer(&ds[24]))
	height := *(*int32)(unsafe.Pointer(&ds[40]))
	if bits == 0 {
		t.Skip("no DIB section behind the render target")
	}
	pix := unsafe.Slice((*byte)(*(*unsafe.Pointer)(unsafe.Pointer(&bits))), refW*refH*4)
	img := image.NewRGBA(image.Rect(0, 0, refW, refH))
	for y := 0; y < refH; y++ {
		sy := y
		if height < 0 { // empirically the target's rows read bottom-up when biHeight is negative
			sy = refH - 1 - y
		}
		for x := 0; x < refW; x++ {
			s := pix[(sy*refW+x)*4:]
			copy(img.Pix[y*img.Stride+x*4:], []byte{s[2], s[1], s[0], 255})
		}
	}
	return img
}

// gpuRender draws the scene on a hidden window with the real D3D11 renderer
// and returns the captured frame.
func gpuRender(t *testing.T, sc *scene.Scene) *image.RGBA {
	t.Helper()
	cls, _ := syscall.UTF16PtrFromString("STATIC")
	hwnd, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(cls)), 0, 0x80000000, 0, 0, refW, refH, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Skip("cannot create a window")
	}
	defer procDestroyWindow.Call(hwnd)
	dev, err := d3d11.NewDevice()
	if err != nil {
		t.Skipf("no Direct3D 11 device: %v", err)
	}
	defer dev.Close()
	r, err := dev.NewRenderer(hwnd, refW, refH)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var got *image.RGBA
	var capErr error
	if err := r.BeginFrame(refW, refH, sc.Clear); err != nil {
		t.Fatal(err)
	}
	sc.Replay(r)
	r.CaptureNextFrame(func(img *image.RGBA, err error) { got, capErr = img, err })
	if err := r.EndFrame(); err != nil {
		t.Fatal(err)
	}
	if capErr != nil || got == nil {
		t.Fatalf("capture failed: %v", capErr)
	}
	return got
}

// diff returns the mean absolute per-channel difference over the whole image
// and over the pixels where the native image differs from the background.
func diff(a, b *image.RGBA, bg [3]uint8) (all, ink float64) {
	var sum, isum float64
	var n, in int
	for y := 0; y < refH; y++ {
		for x := 0; x < refW; x++ {
			ia, ib := y*a.Stride+x*4, y*b.Stride+x*4
			var d float64
			inked := false
			for c := 0; c < 3; c++ {
				d += math.Abs(float64(a.Pix[ia+c]) - float64(b.Pix[ib+c]))
				if b.Pix[ib+c] != bg[c] {
					inked = true
				}
			}
			sum += d
			n += 3
			if inked {
				isum += d
				in += 3
			}
		}
	}
	return sum / float64(n), isum / float64(max(in, 1))
}

func dumpPNG(t *testing.T, name string, img *image.RGBA) {
	dir := os.Getenv("JAMB_REF_DUMP")
	if dir == "" {
		return
	}
	const k = 6
	big := image.NewRGBA(image.Rect(0, 0, img.Rect.Dx()*k, img.Rect.Dy()*k))
	for y := 0; y < big.Rect.Dy(); y++ {
		for x := 0; x < big.Rect.Dx(); x++ {
			copy(big.Pix[y*big.Stride+x*4:], img.Pix[(y/k)*img.Stride+(x/k)*4:][:4])
		}
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Log(err)
		return
	}
	defer f.Close()
	png.Encode(f, big)
}

func absDiffImage(a, b *image.RGBA) *image.RGBA {
	d := image.NewRGBA(a.Rect)
	for i := 0; i < len(a.Pix); i += 4 {
		for c := 0; c < 3; c++ {
			v := math.Abs(float64(a.Pix[i+c])-float64(b.Pix[i+c])) * 4
			d.Pix[i+c] = uint8(min(v, 255))
		}
		d.Pix[i+3] = 255
	}
	return d
}

func sceneColor(c [3]uint8) scene.Color {
	return scene.Color{R: float32(c[0]) / 255, G: float32(c[1]) / 255, B: float32(c[2]) / 255, A: 1}
}

func TestClearTypeReference(t *testing.T) {
	sys := NewSystem()
	if !sys.Subpixel() {
		t.Skip("ClearType is off on this system")
	}
	if dwFactory() == nil {
		t.Skip("no DirectWrite")
	}
	p := sys.sub.Texture.Text
	t.Logf("rendering params: gamma %.2f contrast %.2f level %.2f bgr=%v", p.Gamma, p.Contrast, p.Level, sys.subBGR)
	gray := NewSystem()
	gray.sub = nil // the grayscale pipeline, for comparison
	var totAll, totInk, totGray float64
	for _, c := range refCases {
		l := sys.Layout(c.text, Style{Size: c.size}, float32(math.Inf(1)))
		const ox, oy = 6, 6
		gl := refGlyphs(l, ox, oy, 1)
		native := nativeClearType(t, gl, c, p, sys.subBGR)

		var sc scene.Scene
		sc.Clear = sceneColor(c.bg)
		sys.Draw(&sc, l, ox, oy, 0, 1, 0, sceneColor(c.fg))
		ours := gpuRender(t, &sc)

		var gsc scene.Scene
		gsc.Clear = sceneColor(c.bg)
		gray.Draw(&gsc, gray.Layout(c.text, Style{Size: c.size}, float32(math.Inf(1))), ox, oy, 0, 1, 0, sceneColor(c.fg))
		_, gink := diff(gpuRender(t, &gsc), native, c.bg)
		totGray += gink

		all, ink := diff(ours, native, c.bg)
		totAll += all
		totInk += ink
		t.Logf("%-16s MAD all %.3f  ink %.2f  (grayscale path: ink %.2f)", c.name, all, ink, gink)
		dumpPNG(t, c.name+"-ours", ours)
		dumpPNG(t, c.name+"-native", native)
		dumpPNG(t, c.name+"-diff", absDiffImage(ours, native))
	}
	n := float64(len(refCases))
	t.Logf("average MAD all %.3f  ink %.2f  (grayscale path: ink %.2f)", totAll/n, totInk/n, totGray/n)
	if totInk/n > 4 {
		t.Errorf("ClearType output differs from DirectWrite's by %.2f levels on average over inked pixels", totInk/n)
	}
}
