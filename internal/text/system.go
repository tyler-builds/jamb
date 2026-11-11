package text

import (
	"fmt"
	"image"
	"math"
	"os"
	"strings"

	tsfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/segmenter"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"github.com/tyler-builds/jamb/internal/scene"
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
	// color holds color glyphs (emoji), premultiplied; nil until one is drawn.
	color *Atlas

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
	fg   uint32 // see colorKey
}

type maskEntry struct {
	gen        int
	x, y, w, h int
	// Offset from the pen position (baseline origin) to the mask's top-left.
	dx, dy int
	// in is the atlas the mask lives in.
	in uint8
}

// The atlases a mask can live in (maskEntry.in).
const (
	inGray     uint8 = iota // System.atlas: coverage
	inSubpixel              // System.sub: ClearType coverage
	inColor                 // System.color: a color glyph
)

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
	evicted  bool // dropped from the cache (see EndFrame)
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

// WaitForFonts waits until the system can lay out text. A web page fetches its
// fonts (see fonts_js.go), and this fetches Go Regular, which every text can
// be drawn with; the other faces are fetched when text asks for them. Other
// builds have their fonts at once.
func (sys *System) WaitForFonts() error { return waitPlatformFonts(sys.reg) }

// OnFontLoaded sets fn to be called when a face that was on its way arrives, on
// the goroutine that fetched it: until then, text that asked for the face was
// laid out with the nearest face there was. fn should have the UI thread call
// FontsChanged and lay the text out again. Set it before WaitForFonts.
func (sys *System) OnFontLoaded(fn func()) { sys.reg.loaded = fn }

// FontsChanged forgets which face each text was drawn with, and the layouts
// made with them, so that text laid out again uses the faces that have
// arrived. UI thread only.
func (sys *System) FontsChanged() {
	for _, f := range sys.reg.families {
		clear(f.resolved)
	}
	clear(sys.reg.byRune)
	clear(sys.layouts)
	clear(sys.paras)
	clear(sys.ellipses)
}

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
	return sys.LayoutRef(s, st, spans, maxWidth).l
}

// LayoutRef refers to the cached layouts that a LayoutSpans call used, so that a
// caller that asks again for the same layout takes it with Reuse, without the
// lookup, whose key holds the whole text.
type LayoutRef struct {
	l            *Layout
	full, narrow *cachedLayout
}

// Layout returns the layout r refers to.
func (r LayoutRef) Layout() *Layout { return r.l }

// LayoutRef is LayoutSpans, and returns a reference to the layout for Reuse.
func (sys *System) LayoutRef(s string, st Style, spans []Span, maxWidth float32) LayoutRef {
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
	if full.l.Width <= maxWidth || (st.NoWrap && st.MaxLines == 0) {
		return LayoutRef{l: full.l, full: full}
	}
	narrow := sys.cached(s, st, spans, sk, maxWidth)
	return LayoutRef{l: narrow.l, full: full, narrow: narrow}
}

// Reuse returns the layout r refers to, which the same LayoutSpans call returns
// again, and marks the cached layouts it used as used, as that call does. It
// reports false when one of them has been dropped from the cache since, and then
// the caller asks LayoutRef again.
func (sys *System) Reuse(r LayoutRef) (*Layout, bool) {
	if r.full == nil || r.full.evicted || r.narrow != nil && r.narrow.evicted {
		return nil, false
	}
	r.full.lastUsed = sys.frame
	if r.narrow != nil {
		r.narrow.lastUsed = sys.frame
	}
	return r.l, true
}

// minContent is the width that asks build for the min-content layout of a text:
// the lines it breaks into when it may break only between words. No real width
// is negative, so the two cannot be confused.
const minContent float32 = -1

// MinContentWidth returns the min-content width of s: the width of its widest
// unbreakable run, which is the narrowest that s can be wrapped to without
// breaking inside a word (CSS min-content size). A word wider than the width is
// broken only when s is laid out narrower than the word. wraps is false for a
// NoWrap text without MaxLines, which is one line; its width is then its full
// width. MaxLines does not change the longest word, so it is ignored. Results
// are cached.
func (sys *System) MinContentWidth(s string, st Style, spans []Span) (width float32, wraps bool) {
	if st.Size <= 0 {
		st.Size = 14
	}
	if st.Weight == 0 {
		st.Weight = Regular
	}
	if st.NoWrap && st.MaxLines == 0 {
		return sys.LayoutSpans(s, st, spans, float32(math.Inf(1))).Width, false
	}
	// build treats NoWrap without MaxLines as one line, so NoWrap goes too.
	st.NoWrap, st.MaxLines = false, 0
	return sys.cached(s, st, spans, spansKey(spans), minContent).l.Width, true
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

func (sys *System) cached(s string, st Style, spans []Span, sk string, maxWidth float32) *cachedLayout {
	k := layoutKey{s, st, maxWidth, sk}
	if c, ok := sys.layouts[k]; ok {
		c.lastUsed = sys.frame
		return c
	}
	c := &cachedLayout{l: sys.build(s, st, spans, maxWidth), lastUsed: sys.frame}
	sys.layouts[k] = c
	return c
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
			c.evicted = true
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

// DrawCached is Draw with the atlas entries of the glyphs kept in c, for the next
// draw at the same place (see DrawCache). c may be nil.
func (sys *System) DrawCached(sc *scene.Scene, l *Layout, c *DrawCache, x, y, boxWidth, scale float32, align int, color scene.Color) {
	sys.DrawRichCached(sc, l, c, x, y, boxWidth, scale, align, func(int) RunPaint { return RunPaint{Color: color} })
}

// DrawRich is Draw with paint per span: paint(span) says how the runs of that
// span look, and span is -1 for text outside every span. A run draws its
// background, then its glyphs, then its underline and strikethrough.
func (sys *System) DrawRich(sc *scene.Scene, l *Layout, x, y, boxWidth, scale float32, align int, paint func(span int) RunPaint) {
	sys.DrawRichCached(sc, l, nil, x, y, boxWidth, scale, align, paint)
}

// DrawCache keeps the atlas entries of the glyphs of a text as a draw found them,
// so that the next draw of the text at the same place takes them from here rather
// than from the table of glyphs, whose lookup for each glyph was the largest part
// of drawing text that had not changed, as in each frame of a vertical scroll.
//
// The entries were found for the layout l, at the scale, with the left edge at x,
// aligned as align in a box of boxWidth, with the colors of the runs, and with the
// atlases at gens. Together these decide the key of every glyph: the scale, x and
// the alignment its size and subpixel bin, the colors the entry of a color glyph.
// The top of the text decides only where the glyphs go, not their entries. An
// entry stays where it is until its atlas is cleared, which changes its
// generation. So a draw with the same of each finds the same entries. x is
// compared exactly, not by its fraction of a pixel: two edges with the same
// fraction can round to different bins in float32 arithmetic.
//
// Each place that draws text has a DrawCache of its own, since texts with the same
// content share one Layout. A draw records the entries only when the draw before
// it was of the same layout at the same x and scale, so that text drawn once, or
// moving sideways each frame as in a horizontal scroll, costs no recording that
// no later draw could use. The zero DrawCache is empty.
type DrawCache struct {
	valid              bool // the entries were recorded, for the draw described here
	l                  *Layout
	scale, x, boxWidth float32
	align              int
	gens               [3]int
	colors             []scene.Color
	entries            []drawnGlyph
	// The last draw's layout, x and scale, recorded or not.
	lastL            *Layout
	lastX, lastScale float32
}

// Reset empties c, keeping its buffers, for another text to use.
func (c *DrawCache) Reset() {
	*c = DrawCache{colors: c.colors[:0], entries: c.entries[:0]}
}

// Room returns how many glyphs c has room for without allocating.
func (c *DrawCache) Room() int { return cap(c.entries) }

// counts returns the number of runs of l, and of its glyphs that have a font,
// which are what a DrawCache records.
func (l *Layout) counts() (runs, glyphs int) {
	for _, ln := range l.Lines {
		runs += len(ln.Runs)
		for _, run := range ln.Runs {
			for _, g := range run.Glyphs {
				if g.font != nil {
					glyphs++
				}
			}
		}
	}
	return runs, glyphs
}

// sized returns s emptied, with room for n elements: s itself when it has the
// room, so that recording allocates once at most for each buffer.
func sized[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, 0, n)
	}
	return s[:0]
}

// drawnGlyph is the atlas entry of a glyph in 16 bits a field (atlases are at most
// atlasMax wide), or ok false when the glyph has none.
type drawnGlyph struct {
	x, y, w, h, dx, dy int16
	in                 uint8
	ok                 bool
}

// packGlyph returns e as a drawnGlyph, and false if a field does not fit.
func packGlyph(e maskEntry, ok bool) (drawnGlyph, bool) {
	g := drawnGlyph{int16(e.x), int16(e.y), int16(e.w), int16(e.h), int16(e.dx), int16(e.dy), e.in, ok}
	fits := int(g.x) == e.x && int(g.y) == e.y && int(g.w) == e.w && int(g.h) == e.h &&
		int(g.dx) == e.dx && int(g.dy) == e.dy
	return g, fits
}

// gens returns the generations of the atlases, -1 for one not made.
func (sys *System) gens() [3]int {
	g := [3]int{sys.atlas.Generation, -1, -1}
	if sys.sub != nil {
		g[1] = sys.sub.Generation
	}
	if sys.color != nil {
		g[2] = sys.color.Generation
	}
	return g
}

// DrawRichCached is DrawRich with the atlas entries of the glyphs kept in c, for
// the next draw at the same place (see DrawCache). c may be nil.
func (sys *System) DrawRichCached(sc *scene.Scene, l *Layout, c *DrawCache, x, y, boxWidth, scale float32, align int, paint func(span int) RunPaint) {
	if align == 0 {
		boxWidth = 0 // the box does not move left-aligned text
	}
	var rec *DrawCache
	var gens [3]int
	if c != nil {
		gens = sys.gens()
		if c.valid && c.l == l && c.scale == scale && c.x == x && c.align == align && c.boxWidth == boxWidth && c.gens == gens {
			sys.drawFrom(sc, l, c, x, y, boxWidth, scale, align, paint)
			return
		}
		if c.lastL == l && c.lastX == x && c.lastScale == scale {
			rec = c
			rec.valid = false
			runs, glyphs := l.counts()
			rec.colors = sized(rec.colors, runs)
			rec.entries = sized(rec.entries, glyphs)
		}
		c.lastL, c.lastX, c.lastScale = l, x, scale
	}
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
			if rec != nil {
				rec.colors = append(rec.colors, rp.Color)
			}
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
				e, ok := sys.glyph(glyphKey{g.font, g.ID, sizeKey, bin, aa, colorKey(g.font, rp.Color)}, physSize)
				if !ok || e.w == 0 {
					continue
				}
				bounds := scene.Rect{X: ix + float32(e.dx), Y: py + float32(e.dy), W: float32(e.w), H: float32(e.h)}
				src := scene.Rect{X: float32(e.x), Y: float32(e.y), W: float32(e.w), H: float32(e.h)}
				if e.in == inColor {
					sc.AddImage(scene.ImageDraw{Bounds: bounds, Texture: sys.color.Texture, Src: src, Opacity: rp.Color.A})
					continue
				}
				sc.AddGlyph(sys.atlasOf(e.in).Texture, scene.Glyph{Bounds: bounds, Src: src, Color: rp.Color})
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
	if rec != nil {
		sys.record(rec, l, x, boxWidth, scale, align)
	}
}

// record records in c the atlas entries of the glyphs of l, drawn with the colors
// of c at x, aligned as align in a box of boxWidth, at the scale, as DrawRichCached
// has just drawn it. It looks the glyphs up again, as the draw did, rather than
// collecting them during the draw: the draw's loop is the one every text runs, and
// collecting in it made all of them slower. A text is recorded once, at its second
// draw at the same place (see DrawCache).
func (sys *System) record(c *DrawCache, l *Layout, x, boxWidth, scale float32, align int) {
	physSize := l.Style.Size * scale
	aa := aaGray
	if sys.sub != nil {
		aa = aaSubpixel
	}
	sizeKey := uint32(physSize*64 + 0.5)
	gens := sys.gens()
	fits := true
	ri := 0
	for _, ln := range l.Lines {
		lx := x
		switch align {
		case 1:
			lx += (boxWidth - ln.Width) / 2
		case 2:
			lx += boxWidth - ln.Width
		}
		for _, run := range ln.Runs {
			if ri >= len(c.colors) {
				return
			}
			color := c.colors[ri]
			ri++
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
				e, ok := sys.glyph(glyphKey{g.font, g.ID, sizeKey, bin, aa, colorKey(g.font, color)}, physSize)
				pg, f := packGlyph(e, ok)
				fits = fits && f
				c.entries = append(c.entries, pg)
			}
		}
	}
	// An atlas cleared while the glyphs were looked up dropped entries found before
	// it: then they are not kept.
	if fits && sys.gens() == gens {
		c.valid = true
		c.l, c.scale, c.x, c.align, c.boxWidth, c.gens = l, scale, x, align, boxWidth, gens
	}
}

// drawFrom draws l as DrawRich does, with the atlas entries of its glyphs from c,
// which were recorded for this draw. A run whose color is not the one recorded has
// its glyphs looked up, and c is left as it is.
func (sys *System) drawFrom(sc *scene.Scene, l *Layout, c *DrawCache, x, y, boxWidth, scale float32, align int, paint func(span int) RunPaint) {
	physSize := l.Style.Size * scale
	aa := aaGray
	if sys.sub != nil {
		aa = aaSubpixel
	}
	sizeKey := uint32(physSize*64 + 0.5)
	thick := float32(max(1, math.Round(float64(physSize/14))))
	ri, gi := 0, 0
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
			same := ri < len(c.colors) && c.colors[ri] == rp.Color
			ri++
			if rp.Background.A > 0 {
				sc.AddQuad(scene.Quad{Bounds: pixelRect(lx+run.X, top, run.Width, l.LineHeight, scale), Background: rp.Background})
			}
			for _, g := range run.Glyphs {
				if g.font == nil {
					continue
				}
				px := (lx + run.X + g.X) * scale
				ix := float32(math.Floor(float64(px)))
				py := float32(math.Round(float64((baseline + g.Y) * scale)))
				var bounds, src scene.Rect
				var in uint8
				if same && gi < len(c.entries) {
					d := c.entries[gi]
					gi++
					if !d.ok || d.w == 0 {
						continue
					}
					bounds = scene.Rect{X: ix + float32(d.dx), Y: py + float32(d.dy), W: float32(d.w), H: float32(d.h)}
					src = scene.Rect{X: float32(d.x), Y: float32(d.y), W: float32(d.w), H: float32(d.h)}
					in = d.in
				} else {
					gi++
					bin := uint8((px - ix) * subpixelBins)
					if bin >= subpixelBins {
						bin = subpixelBins - 1
					}
					e, ok := sys.glyph(glyphKey{g.font, g.ID, sizeKey, bin, aa, colorKey(g.font, rp.Color)}, physSize)
					if !ok || e.w == 0 {
						continue
					}
					bounds = scene.Rect{X: ix + float32(e.dx), Y: py + float32(e.dy), W: float32(e.w), H: float32(e.h)}
					src = scene.Rect{X: float32(e.x), Y: float32(e.y), W: float32(e.w), H: float32(e.h)}
					in = e.in
				}
				if in == inColor {
					sc.AddImage(scene.ImageDraw{Bounds: bounds, Texture: sys.color.Texture, Src: src, Opacity: rp.Color.A})
					continue
				}
				sc.AddGlyph(sys.atlasOf(in).Texture, scene.Glyph{Bounds: bounds, Src: src, Color: rp.Color})
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
	if e, ok := sys.glyphs[k]; ok && e.gen == sys.atlasOf(e.in).Generation {
		return e, true
	}
	if k.fg != 0 {
		if dx, dy, m, ok := dwriteColorGlyph(k.font, k.id, physSize, float32(k.bin)/subpixelBins, keyColor(k.fg), sys.rgbaScratch); ok {
			a := sys.colorAtlas()
			e := maskEntry{gen: a.Generation, dx: dx, dy: dy, in: inColor}
			if m == nil {
				sys.glyphs[k] = e // whitespace
				return e, true
			}
			return sys.store(&e, a, m.Rect.Dx(), m.Rect.Dy(), func(x, y int) { a.uploadRGBA(x, y, m) }, func() { sys.glyphs[k] = e })
		}
		// Not a color glyph: a mask, tinted with the text color.
	}
	// DirectWrite draws a variable font only in its default instance.
	if k.aa == aaSubpixel && !k.font.varied {
		if dx, dy, m, ok := dwriteGlyphClearType(k.font, k.id, physSize, float32(k.bin)/subpixelBins, sys.subBGR, sys.rgbaScratch); ok {
			e := maskEntry{gen: sys.sub.Generation, dx: dx, dy: dy, in: inSubpixel}
			if m == nil {
				sys.glyphs[k] = e // whitespace
				return e, true
			}
			return sys.store(&e, sys.sub, m.Rect.Dx(), m.Rect.Dy(), func(x, y int) { sys.sub.uploadRGBA(x, y, m) }, func() { sys.glyphs[k] = e })
		}
		// DirectWrite cannot do this glyph; draw it grayscale (kept under the
		// same key).
	}
	if dx, dy, m, ok := dwriteGlyph(k.font, k.id, physSize, float32(k.bin)/subpixelBins, sys.alphaScratch); ok && !k.font.varied {
		e := maskEntry{gen: sys.atlas.Generation, dx: dx, dy: dy}
		if m == nil {
			sys.glyphs[k] = e // whitespace
			return e, true
		}
		return sys.storeAlpha(&e, m.Rect.Dx(), m.Rect.Dy(), m, func() { sys.glyphs[k] = e })
	}
	segs, err := k.font.outline(&sys.buf, k.id, physSize)
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

func (sys *System) atlasOf(in uint8) *Atlas {
	switch in {
	case inSubpixel:
		return sys.sub
	case inColor:
		return sys.colorAtlas()
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
			if v.in == e.in {
				delete(sys.glyphs, k)
			}
		}
		if e.in == inGray {
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

// Warm loads the fonts the built-in widgets use and initializes the shaper by
// laying out a sample. Apps call it on a goroutine during startup (while the
// GPU device is being created) so the first frame does not pay for font
// parsing. Bold is left to load on first use: no built-in widget draws it,
// and each parsed face costs a few megabytes. The System must not be used
// concurrently with Warm.
func (sys *System) Warm(size float32) {
	for _, w := range []Weight{Regular, Medium, Semibold} {
		sys.Layout("Warm up the shaper: Ag 0123", Style{Size: size, Weight: w}, 1e6)
	}
}

// outline returns the outline of glyph id at size pixels per em, in 26.6 pixels
// with the y axis down, as sfnt gives it. A variable font set to another weight
// than its default one (varied) has its outline from go-text, which applies the
// variations; sfnt draws only the default instance.
func (f *Font) outline(buf *sfnt.Buffer, id sfnt.GlyphIndex, size float32) (sfnt.Segments, error) {
	if !f.varied {
		return f.sf.LoadGlyph(buf, id, fixed.Int26_6(size*64), nil)
	}
	o, ok := f.face.GlyphDataOutline(tsfont.GID(id))
	if !ok {
		return nil, fmt.Errorf("text: font %q has no outline for glyph %d", f.Name, id)
	}
	s := size / f.upem * 64
	segs := make(sfnt.Segments, len(o.Segments))
	for i, g := range o.Segments {
		segs[i].Op = sfnt.SegmentOp(g.Op) // move, line, quad, cubic in both
		for j, a := range g.Args {
			segs[i].Args[j] = fixed.Point26_6{X: fixed.Int26_6(math.Round(float64(a.X * s))), Y: fixed.Int26_6(math.Round(float64(-a.Y * s)))}
		}
	}
	return segs, nil
}
