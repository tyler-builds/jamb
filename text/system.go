package text

import (
	"fmt"
	"image"
	"math"
	"os"
	"strings"

	"github.com/go-text/typesetting/segmenter"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"github.com/tyler-builds/jamb/scene"
)

// subpixelBins is the number of horizontal subpixel positions glyphs are
// rasterized at. 4 gives smooth spacing at a small atlas cost.
const subpixelBins = 4

// Anti-aliasing modes of a glyph mask; part of the glyph cache key.
const (
	aaGray     uint8 = iota // one coverage byte per pixel
	aaSubpixel              // ClearType: R, G, B coverage
)

// subpixelSettings is the platform's ClearType configuration.
type subpixelSettings struct {
	BGR    bool // panel subpixel order is B, G, R
	Params scene.TextParams
}

// System is the entry point of the package: it owns the font registry, the
// layout cache and the mask atlas. Use one System per UI thread.
type System struct {
	reg   *Registry
	buf   sfnt.Buffer
	atlas *Atlas

	// sub holds ClearType glyph masks; nil when subpixel text is off, which
	// is the case unless the platform reports ClearType (see platformSubpixel).
	// JAMB_TEXT_AA=gray forces grayscale.
	sub      *Atlas
	subBGR   bool
	rgbaTemp *image.RGBA

	// Shaping pipeline (go-text/typesetting), reused between layouts.
	seg      shaping.Segmenter
	shaper   shaping.HarfbuzzShaper
	wrapper  shaping.LineWrapper
	outs     []shaping.Output
	ellipses map[ellipsisKey]shaping.Output
	paras    map[paraKey]*cachedPara

	// Grapheme boundaries of the last string asked about (editors ask
	// repeatedly about the same text).
	gseg          segmenter.Segmenter
	graphemeText  string
	graphemeStops []int

	glyphs  map[glyphKey]maskEntry
	masks   map[MaskKey]maskEntry
	layouts map[layoutKey]*cachedLayout
	frame   int

	raster  vector.Rasterizer
	scratch *image.Alpha
}

type glyphKey struct {
	font *Font
	id   sfnt.GlyphIndex
	size uint32 // physical pixel size × 64
	bin  uint8
	aa   uint8
}

type maskEntry struct {
	gen        int
	x, y, w, h int
	// Offset from the pen position (baseline origin) to the mask's top-left.
	dx, dy int
	// sub is set when the mask lives in the subpixel atlas.
	sub bool
}

// MaskKey identifies a cached non-glyph mask such as an icon at a size.
type MaskKey struct {
	Name string
	W, H int
}

type layoutKey struct {
	text  string
	style Style
	width float32
	spans string // spansKey
}

type cachedLayout struct {
	l        *Layout
	lastUsed int
}

// NewSystem creates a text system with the platform fonts.
func NewSystem() *System {
	sys := &System{
		reg:      NewRegistry(),
		atlas:    NewAtlas(),
		glyphs:   map[glyphKey]maskEntry{},
		masks:    map[MaskKey]maskEntry{},
		layouts:  map[layoutKey]*cachedLayout{},
		ellipses: map[ellipsisKey]shaping.Output{},
		paras:    map[paraKey]*cachedPara{},
	}
	if os.Getenv("JAMB_TEXT_AA") != "gray" {
		if s, ok := platformSubpixel(); ok {
			sys.sub, sys.subBGR = newSubpixelAtlas(s.Params), s.BGR
		}
	}
	return sys
}

// Subpixel reports whether glyphs are drawn with ClearType. Sprites from the
// subpixel atlas assume an opaque destination at pixel scale.
func (sys *System) Subpixel() bool { return sys.sub != nil }

// Registry exposes the font registry, e.g. to register application fonts.
func (sys *System) Registry() *Registry { return sys.reg }

// Atlas returns the mask atlas texture.
func (sys *System) Atlas() *scene.Texture { return sys.atlas.Texture }

// Layout shapes and wraps s to maxWidth (logical pixels; +Inf for no limit).
// Results are cached; the returned layout must not be modified.
func (sys *System) Layout(s string, st Style, maxWidth float32) *Layout {
	return sys.LayoutSpans(s, st, nil, maxWidth)
}

// LayoutSpans is Layout with per-range font attributes: each Span is shaped
// in its own family, weight or slant, and the text wraps as one paragraph.
// spans must be sorted, non-overlapping and on rune boundaries (see Span).
func (sys *System) LayoutSpans(s string, st Style, spans []Span, maxWidth float32) *Layout {
	if st.Size <= 0 {
		st.Size = 14
	}
	if st.Weight == 0 {
		st.Weight = Regular
	}
	sk := spansKey(spans)
	// The unconstrained layout answers every width at or above its natural
	// width, so look it up first.
	full := sys.cached(s, st, spans, sk, float32(math.Inf(1)))
	if full.Width <= maxWidth || (st.NoWrap && st.MaxLines == 0) {
		return full
	}
	return sys.cached(s, st, spans, sk, maxWidth)
}

// spansKey encodes spans for the layout cache key.
func spansKey(spans []Span) string {
	if len(spans) == 0 {
		return ""
	}
	var b strings.Builder
	for _, sp := range spans {
		fmt.Fprintf(&b, "%d:%d:%q:%d:%t;", sp.Start, sp.End, sp.Family, sp.Weight, sp.Italic)
	}
	return b.String()
}

func (sys *System) cached(s string, st Style, spans []Span, sk string, maxWidth float32) *Layout {
	k := layoutKey{s, st, maxWidth, sk}
	if c, ok := sys.layouts[k]; ok {
		c.lastUsed = sys.frame
		return c.l
	}
	l := sys.build(s, st, spans, maxWidth)
	sys.layouts[k] = &cachedLayout{l: l, lastUsed: sys.frame}
	return l
}

// EndFrame evicts layouts that have not been used for a while. Call once per
// rendered frame.
func (sys *System) EndFrame() {
	sys.frame++
	if sys.frame%120 != 0 {
		return
	}
	for k, c := range sys.layouts {
		if sys.frame-c.lastUsed > 240 {
			delete(sys.layouts, k)
		}
	}
	for k, c := range sys.paras {
		if sys.frame-c.lastUsed > 240 {
			delete(sys.paras, k)
		}
	}
}

// Draw emits glyph sprites for l into sc. (x, y) is the top-left of the text
// box in logical pixels, boxWidth is used for alignment (0 = left), scale is
// the device scale factor. align: 0 left, 1 center, 2 right.
func (sys *System) Draw(sc *scene.Scene, l *Layout, x, y, boxWidth, scale float32, align int, color scene.Color) {
	sys.DrawRich(sc, l, x, y, boxWidth, scale, align, func(int) RunPaint { return RunPaint{Color: color} })
}

// RunPaint is how DrawRich paints the runs of one span.
type RunPaint struct {
	Color, Background scene.Color
	Underline, Strike bool
	// DecorationColor colors the underline and strikethrough; zero means Color.
	DecorationColor scene.Color
}

// DrawRich is Draw with paint per span: paint(span) says how the runs of that
// span look, and span is -1 for text outside every span. A run draws its
// background, then its glyphs, then its underline and strikethrough.
func (sys *System) DrawRich(sc *scene.Scene, l *Layout, x, y, boxWidth, scale float32, align int, paint func(span int) RunPaint) {
	physSize := l.Style.Size * scale
	aa := aaGray
	if sys.sub != nil {
		aa = aaSubpixel
	}
	sizeKey := uint32(physSize*64 + 0.5)
	thick := float32(max(1, math.Round(float64(physSize/14))))
	for _, ln := range l.Lines {
		lx := x
		switch align {
		case 1:
			lx += (boxWidth - ln.Width) / 2
		case 2:
			lx += boxWidth - ln.Width
		}
		top := y + ln.Top
		baseline := y + ln.Baseline
		for _, run := range ln.Runs {
			rp := paint(run.Span)
			if rp.Background.A > 0 {
				sc.AddQuad(scene.Quad{Bounds: pixelRect(lx+run.X, top, run.Width, l.LineHeight, scale), Background: rp.Background})
			}
			for _, g := range run.Glyphs {
				if g.font == nil {
					continue
				}
				px := (lx + run.X + g.X) * scale
				ix := float32(math.Floor(float64(px)))
				bin := uint8((px - ix) * subpixelBins)
				if bin >= subpixelBins {
					bin = subpixelBins - 1
				}
				py := float32(math.Round(float64((baseline + g.Y) * scale)))
				e, ok := sys.glyph(glyphKey{g.font, g.ID, sizeKey, bin, aa}, physSize)
				if !ok || e.w == 0 {
					continue
				}
				tex := sys.atlas.Texture
				if e.sub {
					tex = sys.sub.Texture
				}
				sc.AddGlyph(tex, scene.Glyph{
					Bounds: scene.Rect{X: ix + float32(e.dx), Y: py + float32(e.dy), W: float32(e.w), H: float32(e.h)},
					Src:    scene.Rect{X: float32(e.x), Y: float32(e.y), W: float32(e.w), H: float32(e.h)},
					Color:  rp.Color,
				})
			}
			if !rp.Underline && !rp.Strike {
				continue
			}
			dec := rp.DecorationColor
			if dec.A == 0 {
				dec = rp.Color
			}
			if rp.Underline {
				r := pixelRect(lx+run.X, baseline+max(1, l.Style.Size*0.08), run.Width, 0, scale)
				r.H = thick
				sc.AddQuad(scene.Quad{Bounds: r, Background: dec})
			}
			if rp.Strike {
				r := pixelRect(lx+run.X, baseline-0.3*l.Style.Size, run.Width, 0, scale)
				r.H = thick
				sc.AddQuad(scene.Quad{Bounds: r, Background: dec})
			}
		}
	}
}

// pixelRect snaps the logical rectangle (x, y, w, h) to device pixels. Each
// edge is rounded on its own, so runs that share an edge leave no gap.
func pixelRect(x, y, w, h, scale float32) scene.Rect {
	x0 := float32(math.Round(float64(x * scale)))
	y0 := float32(math.Round(float64(y * scale)))
	x1 := float32(math.Round(float64((x + w) * scale)))
	y1 := float32(math.Round(float64((y + h) * scale)))
	return scene.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// glyph returns the atlas entry for a glyph, rasterizing it if needed.
func (sys *System) glyph(k glyphKey, physSize float32) (maskEntry, bool) {
	if e, ok := sys.glyphs[k]; ok && e.gen == sys.atlasOf(e.sub).Generation {
		return e, true
	}
	if k.aa == aaSubpixel {
		if dx, dy, m, ok := dwriteGlyphClearType(k.font, k.id, physSize, float32(k.bin)/subpixelBins, sys.subBGR, sys.rgbaScratch); ok {
			e := maskEntry{gen: sys.sub.Generation, dx: dx, dy: dy, sub: true}
			if m == nil {
				sys.glyphs[k] = e // whitespace
				return e, true
			}
			return sys.store(&e, sys.sub, m.Rect.Dx(), m.Rect.Dy(), func(x, y int) { sys.sub.uploadRGBA(x, y, m) }, func() { sys.glyphs[k] = e })
		}
		// DirectWrite cannot do this glyph; draw it grayscale (kept under the
		// same key).
	}
	if dx, dy, m, ok := dwriteGlyph(k.font, k.id, physSize, float32(k.bin)/subpixelBins, sys.alphaScratch); ok {
		e := maskEntry{gen: sys.atlas.Generation, dx: dx, dy: dy}
		if m == nil {
			sys.glyphs[k] = e // whitespace
			return e, true
		}
		return sys.storeAlpha(&e, m.Rect.Dx(), m.Rect.Dy(), m, func() { sys.glyphs[k] = e })
	}
	ppem := fixed.Int26_6(physSize * 64)
	segs, err := k.font.sf.LoadGlyph(&sys.buf, k.id, ppem, nil)
	if err != nil {
		return maskEntry{}, false
	}
	fx := float32(k.bin) / subpixelBins
	b := segs.Bounds()
	minX := float32(b.Min.X)/64 + fx
	maxX := float32(b.Max.X)/64 + fx
	minY, maxY := float32(b.Min.Y)/64, float32(b.Max.Y)/64
	x0, y0 := int(math.Floor(float64(minX))), int(math.Floor(float64(minY)))
	x1, y1 := int(math.Ceil(float64(maxX))), int(math.Ceil(float64(maxY)))
	w, h := x1-x0, y1-y0
	e := maskEntry{gen: sys.atlas.Generation, dx: x0, dy: y0}
	if w <= 0 || h <= 0 {
		sys.glyphs[k] = e // whitespace
		return e, true
	}
	m := sys.alphaScratch(w, h)
	r := &sys.raster
	r.Reset(w, h)
	ox, oy := fx-float32(x0), -float32(y0)
	for _, s := range segs {
		p := func(i int) (float32, float32) {
			return float32(s.Args[i].X)/64 + ox, float32(s.Args[i].Y)/64 + oy
		}
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			r.MoveTo(p(0))
		case sfnt.SegmentOpLineTo:
			r.LineTo(p(0))
		case sfnt.SegmentOpQuadTo:
			ax, ay := p(0)
			bx, by := p(1)
			r.QuadTo(ax, ay, bx, by)
		case sfnt.SegmentOpCubeTo:
			ax, ay := p(0)
			bx, by := p(1)
			cx, cy := p(2)
			r.CubeTo(ax, ay, bx, by, cx, cy)
		}
	}
	r.ClosePath()
	r.Draw(m, m.Bounds(), image.Opaque, image.Point{})
	return sys.storeAlpha(&e, w, h, m, func() { sys.glyphs[k] = e })
}

// Mask returns a cached coverage mask, calling draw to rasterize it on first
// use. It is how icons and other vector masks share the glyph atlas.
func (sys *System) Mask(k MaskKey, draw func(dst *image.Alpha)) (tex *scene.Texture, src scene.Rect, ok bool) {
	e, found := sys.masks[k]
	if !found || e.gen != sys.atlas.Generation {
		if k.W <= 0 || k.H <= 0 {
			return nil, scene.Rect{}, false
		}
		m := sys.alphaScratch(k.W, k.H)
		draw(m)
		e = maskEntry{gen: sys.atlas.Generation}
		if _, ok := sys.storeAlpha(&e, k.W, k.H, m, func() { sys.masks[k] = e }); !ok {
			return nil, scene.Rect{}, false
		}
	}
	return sys.atlas.Texture, scene.Rect{X: float32(e.x), Y: float32(e.y), W: float32(e.w), H: float32(e.h)}, true
}

func (sys *System) atlasOf(sub bool) *Atlas {
	if sub {
		return sys.sub
	}
	return sys.atlas
}

func (sys *System) storeAlpha(e *maskEntry, w, h int, m *image.Alpha, save func()) (maskEntry, bool) {
	return sys.store(e, sys.atlas, w, h, func(x, y int) { sys.atlas.Upload(x, y, m) }, save)
}

// store reserves w×h in a, lets upload copy the mask there and records the
// position in e. When a had to be cleared, the cached entries that lived in it
// are dropped.
func (sys *System) store(e *maskEntry, a *Atlas, w, h int, upload func(x, y int), save func()) (maskEntry, bool) {
	x, y, cleared := a.Alloc(w, h)
	if cleared {
		for k, v := range sys.glyphs {
			if v.sub == e.sub {
				delete(sys.glyphs, k)
			}
		}
		if !e.sub {
			clear(sys.masks)
		}
		e.gen = a.Generation
	}
	if w+atlasPad > a.Texture.Width {
		return maskEntry{}, false
	}
	upload(x, y)
	e.x, e.y, e.w, e.h = x, y, w, h
	save()
	return *e, true
}

func (sys *System) rgbaScratch(w, h int) *image.RGBA {
	if sys.rgbaTemp == nil || len(sys.rgbaTemp.Pix) < w*h*4 {
		sys.rgbaTemp = image.NewRGBA(image.Rect(0, 0, max(w, 64), max(h, 64)))
	}
	m := &image.RGBA{Pix: sys.rgbaTemp.Pix[:w*h*4], Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	clear(m.Pix)
	return m
}

func (sys *System) alphaScratch(w, h int) *image.Alpha {
	if sys.scratch == nil || len(sys.scratch.Pix) < w*h {
		sys.scratch = image.NewAlpha(image.Rect(0, 0, max(w, 64), max(h, 64)))
	}
	m := &image.Alpha{Pix: sys.scratch.Pix[:w*h], Stride: w, Rect: image.Rect(0, 0, w, h)}
	clear(m.Pix)
	return m
}

// Warm loads the default fonts and initializes the shaper by laying out a
// sample. Apps call it on a goroutine during startup (while the GPU device
// is being created) so the first frame does not pay for font parsing.
// The System must not be used concurrently with Warm.
func (sys *System) Warm(size float32) {
	for _, w := range []Weight{Regular, Semibold, Bold} {
		sys.Layout("Warm up the shaper: Ag 0123", Style{Size: size, Weight: w}, 1e6)
	}
}
