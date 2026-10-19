// Package text loads fonts, lays out and measures text, and rasterizes
// glyphs (and other coverage masks such as icons) into a shared atlas.
//
// The first version targets basic Latin text: per-rune glyph lookup with
// font fallback, pair kerning from the 'kern' table and greedy word wrapping.
// Shaping is isolated in shape.go so a full shaper (HarfBuzz-style: GSUB/GPOS,
// bidi, clusters) can replace it without changing the public API.
package text

import (
	"bytes"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"unicode"
	"weak"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	tsfont "github.com/go-text/typesetting/font"
)

// Weight is a font weight on the usual 100–900 scale.
type Weight uint16

const (
	Light    Weight = 300
	Regular  Weight = 400
	Medium   Weight = 500
	Semibold Weight = 600
	Bold     Weight = 700
)

// Family names understood by the default registry.
const (
	FamilySans = "" // the platform UI font
	FamilyMono = "mono"
)

// Font is one parsed font file. It is parsed twice from the same bytes: by
// go-text/typesetting for shaping (face) and by x/image/font/sfnt for glyph
// outlines (sf). Glyph IDs are the font's own, so they agree.
type Font struct {
	Name string
	sf   *sfnt.Font
	face *tsfont.Face
	upem float32
	// Font-unit metrics.
	ascent, descent float32

	// Set for fonts loaded from a file: lets the platform rasterizer
	// (DirectWrite) open the same face. Empty for in-memory fonts.
	path      string
	faceIndex int
	dw        dwFace
}

// faces maps shaping faces back to fonts, for rasterization. It holds both
// weakly, and a font's entry is removed when the font is freed: each System
// parses its own fonts, so a strong map kept every font of every System, such
// as those of a closed window, for the life of the process.
var faces sync.Map // weak.Pointer[tsfont.Face] -> weak.Pointer[Font]

// ParseFont parses TrueType/OpenType data.
func ParseFont(name string, data []byte) (*Font, error) {
	var sf *sfnt.Font
	var face *tsfont.Face
	if bytes.HasPrefix(data, []byte("ttcf")) {
		// A collection: use its first font.
		c, err := sfnt.ParseCollection(data)
		if err == nil {
			sf, err = c.Font(0)
		}
		if err != nil {
			return nil, fmt.Errorf("text: cannot parse font collection %q: %w", name, err)
		}
		fs, err := tsfont.ParseTTC(bytes.NewReader(data))
		if err != nil || len(fs) == 0 {
			return nil, fmt.Errorf("text: cannot load font collection %q for shaping: %v", name, err)
		}
		face = fs[0]
	} else {
		var err error
		if sf, err = sfnt.Parse(data); err != nil {
			return nil, fmt.Errorf("text: cannot parse font %q: %w (expected a .ttf, .otf or .ttc file)", name, err)
		}
		if face, err = tsfont.ParseTTF(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("text: cannot load font %q for shaping: %w", name, err)
		}
	}
	f := &Font{Name: name, sf: sf, face: face, upem: float32(sf.UnitsPerEm())}
	var buf sfnt.Buffer
	m, err := sf.Metrics(&buf, fixed.Int26_6(f.upem*64), font.HintingNone)
	if err != nil {
		return nil, fmt.Errorf("text: font %q has no usable metrics: %w", name, err)
	}
	f.ascent = float32(m.Ascent) / 64
	f.descent = float32(m.Descent) / 64
	key := weak.Make(face)
	faces.Store(key, weak.Make(f))
	runtime.AddCleanup(f, func(k weak.Pointer[tsfont.Face]) { faces.Delete(k) }, key)
	return f, nil
}

// has reports whether the font maps r to a glyph.
func (f *Font) has(r rune) bool {
	_, ok := f.face.NominalGlyph(r)
	return ok
}

func fontOf(face *tsfont.Face) *Font {
	if v, ok := faces.Load(weak.Make(face)); ok {
		return v.(weak.Pointer[Font]).Value()
	}
	return nil
}

// lazyFont loads a font at most once.
type lazyFont struct {
	load func() (*Font, error)
	font *Font
	err  error
	done bool
	// embedded marks a face that only an embedded font provides; see pick.
	embedded bool
}

func (l *lazyFont) get() (*Font, error) {
	if !l.done {
		l.done = true
		l.font, l.err = l.load()
	}
	return l.font, l.err
}

// faceKey identifies one face of a family: a weight in one slant.
type faceKey struct {
	w      Weight
	italic bool
}

// family is a set of faces, keyed by weight and slant.
type family struct {
	resolved map[faceKey]*Font
	sources  map[faceKey]*lazyFont
}

// Registry maps family names and weights to fonts, loading files lazily on
// first use. It is not safe for concurrent use; the ui runtime only touches
// it from the UI thread.
type Registry struct {
	families  map[string]*family
	fallbacks []fallback
}

// fallback is a font used for characters the primary font lacks. covers is
// a cheap script hint so that only a relevant fallback file gets loaded
// (the CJK collections are tens of megabytes).
type fallback struct {
	font   *lazyFont
	covers func(r rune) bool
}

// fallbackSpec names a fallback font file and the characters it is for.
type fallbackSpec struct {
	path   string
	covers func(r rune) bool
}

func inTables(ts ...*unicode.RangeTable) func(rune) bool {
	return func(r rune) bool { return unicode.IsOneOf(ts, r) }
}

// isEmoji is a coarse emoji test: the pictographic planes and dingbats.
func isEmoji(r rune) bool {
	return (r >= 0x1F000 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) || r == 0xFE0F || r == 0x200D
}

func anyRune(rune) bool { return true }

// NewRegistry returns a registry pre-populated with the platform's UI and
// monospace fonts, falling back to the embedded Go fonts when they are
// missing.
func NewRegistry() *Registry {
	r := &Registry{families: map[string]*family{}}
	for fam, faces := range systemFonts() {
		for k, paths := range faces {
			r.addSource(fam, k, func() (*Font, error) { return loadFirst(paths) })
		}
	}
	// Embedded fonts guarantee text renders on every platform. They are a
	// last resort: a weight the system lacks is drawn with another of the
	// family's own faces, not with an embedded face of exactly that weight.
	for _, e := range embeddedFonts() {
		r.addEmbedded(e.family, e.key, e.name, e.data)
	}
	for _, spec := range systemFallbackFonts() {
		path := spec.path
		r.fallbacks = append(r.fallbacks, fallback{
			font:   &lazyFont{load: func() (*Font, error) { return loadFile(path) }},
			covers: spec.covers,
		})
	}
	r.fallbacks = append(r.fallbacks, fallback{
		font:   &lazyFont{load: func() (*Font, error) { return ParseFont("Go Regular", goregular.TTF) }},
		covers: anyRune,
	})
	return r
}

func loadFile(path string) (*Font, error) {
	data, err := readFontFile(path)
	if err != nil {
		return nil, err
	}
	f, err := ParseFont(path, data)
	if err != nil {
		return nil, err
	}
	f.path = path
	return f, nil // faceIndex 0: ParseFont uses the first face of a collection
}

func loadFirst(paths []string) (*Font, error) {
	for _, p := range paths {
		if f, err := loadFile(p); err == nil {
			return f, nil
		}
	}
	return nil, fmt.Errorf("text: none of the font files %v could be loaded", paths)
}

func (r *Registry) fam(name string) *family {
	f := r.families[name]
	if f == nil {
		f = &family{resolved: map[faceKey]*Font{}, sources: map[faceKey]*lazyFont{}}
		r.families[name] = f
	}
	return f
}

func (r *Registry) addSource(familyName string, k faceKey, load func() (*Font, error)) {
	r.fam(familyName).sources[k] = &lazyFont{load: load}
}

// embeddedFont is a font compiled into the binary; see embeddedFonts.
type embeddedFont struct {
	family string
	key    faceKey
	name   string
	data   []byte
}

// addEmbedded registers data as the source for a face, or as the fallback
// for an existing (system) source of that face.
func (r *Registry) addEmbedded(familyName string, k faceKey, name string, data []byte) {
	f := r.fam(familyName)
	if prev := f.sources[k]; prev != nil {
		f.sources[k] = &lazyFont{load: func() (*Font, error) {
			if font, err := prev.get(); err == nil {
				return font, nil
			}
			return ParseFont(name, data)
		}, embedded: prev.embedded}
		return
	}
	f.sources[k] = &lazyFont{load: func() (*Font, error) { return ParseFont(name, data) }, embedded: true}
}

// Register adds (or replaces) an upright font for a family and weight. Use it
// to ship a brand font with an application:
//
//	registry.Register("Inter", text.Regular, interRegularTTF)
func (r *Registry) Register(familyName string, w Weight, data []byte) error {
	font, err := ParseFont(familyName, data)
	if err != nil {
		return err
	}
	f := r.fam(familyName)
	f.sources[faceKey{w, false}] = &lazyFont{font: font, done: true}
	clear(f.resolved)
	return nil
}

// Font returns the closest available face for a family, weight and slant
// (see weightRank): Medium in a family without a Medium face, such as Segoe UI,
// is drawn Semibold, as DirectWrite draws it.
// Unknown families fall back to the sans family. A family without an italic
// face for that weight uses its upright face (there is no synthetic slant).
func (r *Registry) Font(familyName string, w Weight, italic bool) *Font {
	f := r.families[familyName]
	if f == nil {
		familyName, f = FamilySans, r.fam(FamilySans)
	}
	k := faceKey{w, italic}
	if font := f.resolved[k]; font != nil {
		return font
	}
	font := r.pick(familyName, f, k)
	f.resolved[k] = font
	return font
}

// pick loads the best source of the wanted slant, in weightRank order. Faces
// only an embedded font provides come after the family's own faces, so that,
// for example, Medium text on Windows stays in Segoe UI rather than switching
// typeface to Go Medium.
func (r *Registry) pick(familyName string, f *family, k faceKey) *Font {
	keys := make([]faceKey, 0, len(f.sources))
	for cand := range f.sources {
		if cand.italic == k.italic {
			keys = append(keys, cand)
		}
	}
	slices.SortFunc(keys, func(a, b faceKey) int {
		if ea, eb := f.sources[a].embedded, f.sources[b].embedded; ea != eb {
			if ea {
				return 1
			}
			return -1
		}
		return weightRank(k.w, a.w) - weightRank(k.w, b.w)
	})
	for _, cand := range keys {
		if font, err := f.sources[cand].get(); err == nil {
			return font
		}
	}
	if k.italic {
		return r.Font(familyName, k.w, false)
	}
	if familyName != FamilySans {
		return r.Font(FamilySans, k.w, false)
	}
	font, _ := ParseFont("Go Regular", goregular.TTF)
	return font
}

// weightRank orders the weights a family has for a wanted weight, lowest
// first: the nearest weight, and on a tie the heavier one. This is how
// Windows' DirectWrite resolves a weight a family lacks; for example, Medium
// (500) in Segoe UI, which has 400 and 600 faces, resolves to Semibold.
func weightRank(want, have Weight) int {
	if have >= want {
		return int(have-want) * 2
	}
	return int(want-have)*2 + 1
}

// fallbackFonts returns the fallback chain, loading it on first use.
// fallbackFor returns a font that has a glyph for c, or nil. Fallbacks whose
// script hint matches are tried (and loaded) first; others only if already
// loaded, so a stray character never pulls in every font on the system.
func (r *Registry) fallbackFor(c rune) *Font {
	for _, fb := range r.fallbacks {
		if fb.covers(c) {
			if f, err := fb.font.get(); err == nil && f.has(c) {
				return f
			}
		}
	}
	for _, fb := range r.fallbacks {
		if fb.font.done && fb.font.err == nil && fb.font.font.has(c) {
			return fb.font.font
		}
	}
	return nil
}
