//go:build linux

package text

import (
	"strconv"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Fontconfig finds the desktop's fonts on Linux: the faces of its sans-serif
// and monospace families, its emoji font, and a font for a character that no
// other font has. libfontconfig is loaded at run time with purego, so jamb
// needs no cgo; without it, the fixed paths of fonts_other.go are used.

var fc struct {
	once sync.Once
	ok   bool
	// mu serializes the calls into fontconfig and guards runes.
	mu sync.Mutex

	init              func() int32
	nameParse         func(name string) uintptr
	configSubstitute  func(config, p uintptr, kind int32) int32
	defaultSubstitute func(p uintptr)
	fontMatch         func(config, p uintptr, result *int32) uintptr
	patternCreate     func() uintptr
	patternDestroy    func(p uintptr)
	patternAddCharSet func(p uintptr, object string, cs uintptr) int32
	patternGetString  func(p uintptr, object string, n int32, s **byte) int32
	patternGetInteger func(p uintptr, object string, n int32, i *int32) int32
	patternGetBool    func(p uintptr, object string, n int32, b *int32) int32
	patternGetMatrix  func(p uintptr, object string, n int32, m *uintptr) int32
	patternGetCharSet func(p uintptr, object string, n int32, cs *uintptr) int32
	charSetCreate     func() uintptr
	charSetDestroy    func(cs uintptr)
	charSetAddChar    func(cs uintptr, r uint32) int32
	charSetHasChar    func(cs uintptr, r uint32) int32

	faces map[string]map[faceKey]fontFile
	emoji []fileFallback
	runes map[rune]fontFile // a missing entry: not asked yet; an empty path: none
}

const (
	fcResultMatch  = 0
	fcMatchPattern = 0

	fcSlantRoman   = 0
	fcSlantItalic  = 100
	fcSlantOblique = 110
)

// loadFontconfig loads libfontconfig once, and reports whether it is usable.
func loadFontconfig() bool {
	fc.once.Do(func() {
		lib, err := purego.Dlopen("libfontconfig.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		ok := true
		reg := func(fn any, name string) {
			defer func() {
				if recover() != nil {
					ok = false
				}
			}()
			if ok {
				purego.RegisterLibFunc(fn, lib, name)
			}
		}
		reg(&fc.init, "FcInit")
		reg(&fc.nameParse, "FcNameParse")
		reg(&fc.configSubstitute, "FcConfigSubstitute")
		reg(&fc.defaultSubstitute, "FcDefaultSubstitute")
		reg(&fc.fontMatch, "FcFontMatch")
		reg(&fc.patternCreate, "FcPatternCreate")
		reg(&fc.patternDestroy, "FcPatternDestroy")
		reg(&fc.patternAddCharSet, "FcPatternAddCharSet")
		reg(&fc.patternGetString, "FcPatternGetString")
		reg(&fc.patternGetInteger, "FcPatternGetInteger")
		reg(&fc.patternGetBool, "FcPatternGetBool")
		reg(&fc.patternGetMatrix, "FcPatternGetMatrix")
		reg(&fc.patternGetCharSet, "FcPatternGetCharSet")
		reg(&fc.charSetCreate, "FcCharSetCreate")
		reg(&fc.charSetDestroy, "FcCharSetDestroy")
		reg(&fc.charSetAddChar, "FcCharSetAddChar")
		reg(&fc.charSetHasChar, "FcCharSetHasChar")
		fc.ok = ok && fc.init() != 0
		fc.runes = map[rune]fontFile{}
	})
	return fc.ok
}

// fcMatch completes pattern p as fc-match does and returns the best font for
// it, or 0. It destroys p.
func fcMatch(p uintptr) uintptr {
	defer fc.patternDestroy(p)
	if fc.configSubstitute(0, p, fcMatchPattern) == 0 {
		return 0
	}
	fc.defaultSubstitute(p)
	var res int32
	m := fc.fontMatch(0, p, &res)
	if res != fcResultMatch {
		if m != 0 {
			fc.patternDestroy(m)
		}
		return 0
	}
	return m
}

// fcFace is what fontconfig says about a matched font.
type fcFace struct {
	file   fontFile
	family string
	weight int32 // fontconfig's scale; see fcWeight
	slant  int32
	// synthetic is set when fontconfig would slant or embolden the font
	// itself, which jamb does not do: the font is not the face asked for.
	synthetic bool
	// variable is set for a variable font or one of its named instances.
	// jamb draws a variable font only in its default instance.
	variable bool
	scalable bool
}

func fcDescribe(m uintptr) (fcFace, bool) {
	var s *byte
	if fc.patternGetString(m, "file", 0, &s) != fcResultMatch || s == nil {
		return fcFace{}, false
	}
	f := fcFace{file: fontFile{path: goString(s)}, scalable: true}
	var i int32
	if fc.patternGetInteger(m, "index", 0, &i) == fcResultMatch {
		f.file.index = int(i & 0xFFFF)
		f.variable = i>>16 != 0
	}
	if fc.patternGetString(m, "family", 0, &s) == fcResultMatch && s != nil {
		f.family = goString(s)
	}
	if fc.patternGetInteger(m, "weight", 0, &i) == fcResultMatch {
		f.weight = i
	}
	if fc.patternGetInteger(m, "slant", 0, &i) == fcResultMatch {
		f.slant = i
	}
	var b int32
	if fc.patternGetBool(m, "variable", 0, &b) == fcResultMatch && b != 0 {
		f.variable = true
	}
	if fc.patternGetBool(m, "embolden", 0, &b) == fcResultMatch && b != 0 {
		f.synthetic = true
	}
	if fc.patternGetBool(m, "scalable", 0, &b) == fcResultMatch && b == 0 {
		f.scalable = false
	}
	var mat uintptr
	if fc.patternGetMatrix(m, "matrix", 0, &mat) == fcResultMatch && mat != 0 {
		f.synthetic = true
	}
	return f, true
}

// fcHas reports whether the charset of the matched font m has r.
func fcHas(m uintptr, r rune) bool {
	var cs uintptr
	return fc.patternGetCharSet(m, "charset", 0, &cs) == fcResultMatch && cs != 0 && fc.charSetHasChar(cs, uint32(r)) != 0
}

func goString(p *byte) string {
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// fcWeight converts a weight on the 100–900 scale to fontconfig's.
func fcWeight(w Weight) int32 {
	for i := 1; i < len(fcWeights); i++ {
		lo, hi := fcWeights[i-1], fcWeights[i]
		if int32(w) <= hi.ot {
			return lo.fc + (int32(w)-lo.ot)*(hi.fc-lo.fc)/(hi.ot-lo.ot)
		}
	}
	return fcWeights[len(fcWeights)-1].fc
}

// otWeight converts a fontconfig weight to the 100–900 scale.
func otWeight(w int32) int32 {
	for i := 1; i < len(fcWeights); i++ {
		lo, hi := fcWeights[i-1], fcWeights[i]
		if w <= hi.fc {
			return lo.ot + (w-lo.fc)*(hi.ot-lo.ot)/(hi.fc-lo.fc)
		}
	}
	return fcWeights[len(fcWeights)-1].ot
}

// fcWeights is fontconfig's map between the two weight scales (fcfreetype.c).
var fcWeights = []struct{ ot, fc int32 }{
	{0, 0}, {100, 0}, {200, 40}, {300, 50}, {350, 55}, {380, 75}, {400, 80},
	{500, 100}, {600, 180}, {700, 200}, {800, 205}, {900, 210}, {1000, 215},
}

// fcFaceQuery is a face of a jamb family and the fontconfig family it is
// looked up in.
var fcFaceQuery = []struct {
	family, pattern string
	keys            []faceKey
}{
	{FamilySans, "sans-serif", []faceKey{
		{Regular, false}, {Medium, false}, {Bold, false},
		{Regular, true}, {Medium, true}, {Bold, true},
	}},
	{FamilyMono, "monospace", []faceKey{
		{Regular, false}, {Bold, false}, {Regular, true}, {Bold, true},
	}},
}

// fontconfigFaces returns the files of the faces of the sans and mono
// families, as the desktop configures them. A face is left out unless the
// font fontconfig matches has the weight and slant asked for, so that the
// registry's nearest-weight rule picks among the family's real faces, and
// unless it is of the same family as the regular face.
func fontconfigFaces() map[string]map[faceKey]fontFile {
	if !loadFontconfig() {
		return nil
	}
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.faces != nil {
		return copyFaces(fc.faces)
	}
	fc.faces = map[string]map[faceKey]fontFile{}
	for _, q := range fcFaceQuery {
		faces := map[faceKey]fontFile{}
		family := ""
		for _, k := range q.keys {
			f, ok := fcFaceFor(q.pattern, k)
			if !ok {
				continue
			}
			if k == (faceKey{Regular, false}) {
				family = f.family
			} else if family == "" || f.family != family {
				continue
			}
			faces[k] = f.file
		}
		if len(faces) > 0 {
			fc.faces[q.family] = faces
		}
	}
	return copyFaces(fc.faces)
}

func copyFaces(m map[string]map[faceKey]fontFile) map[string]map[faceKey]fontFile {
	out := make(map[string]map[faceKey]fontFile, len(m))
	for k, v := range m {
		faces := make(map[faceKey]fontFile, len(v))
		for kk, vv := range v {
			faces[kk] = vv
		}
		out[k] = faces
	}
	return out
}

// fcFaceFor matches pattern at the weight and slant of k, and returns the
// font only if it is that face.
func fcFaceFor(pattern string, k faceKey) (fcFace, bool) {
	slant := int32(fcSlantRoman)
	if k.italic {
		slant = fcSlantItalic
	}
	// FcNameParse reads numeric values for weight and slant.
	p := fc.nameParse(pattern + ":weight=" + strconv.Itoa(int(fcWeight(k.w))) + ":slant=" + strconv.Itoa(int(slant)))
	if p == 0 {
		return fcFace{}, false
	}
	m := fcMatch(p)
	if m == 0 {
		return fcFace{}, false
	}
	defer fc.patternDestroy(m)
	f, ok := fcDescribe(m)
	if !ok || f.synthetic || f.variable || !f.scalable {
		return fcFace{}, false
	}
	if d := otWeight(f.weight) - int32(k.w); d <= -50 || d >= 50 {
		return fcFace{}, false
	}
	if italic := f.slant == fcSlantItalic || f.slant == fcSlantOblique; italic != k.italic || !italic && f.slant != fcSlantRoman {
		return fcFace{}, false
	}
	return f, true
}

// fontconfigFallbacks returns the desktop's emoji font, for the characters
// isEmoji reports, if fontconfig has one: the font of its "emoji" family,
// provided it has an emoji (U+1F600), since fontconfig answers any family
// with some font.
func fontconfigFallbacks() []fileFallback {
	if !loadFontconfig() {
		return nil
	}
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.emoji == nil {
		fc.emoji = []fileFallback{}
		if p := fc.nameParse("emoji"); p != 0 {
			if m := fcMatch(p); m != 0 {
				if f, ok := fcDescribe(m); ok && f.scalable && fcHas(m, 0x1F600) {
					fc.emoji = append(fc.emoji, fileFallback{f.file, isEmoji})
				}
				fc.patternDestroy(m)
			}
		}
	}
	return fc.emoji
}

// fontconfigRune returns the font fontconfig picks for a text of the one
// character r, provided that font has r. Answers are kept per character.
func fontconfigRune(r rune) (fontFile, bool) {
	if !loadFontconfig() {
		return fontFile{}, false
	}
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if f, ok := fc.runes[r]; ok {
		return f, f.path != ""
	}
	var file fontFile
	if p := fc.patternCreate(); p != 0 {
		cs := fc.charSetCreate()
		added := cs != 0 && fc.charSetAddChar(cs, uint32(r)) != 0 && fc.patternAddCharSet(p, "charset", cs) != 0
		if cs != 0 {
			fc.charSetDestroy(cs) // the pattern holds its own reference
		}
		if !added {
			fc.patternDestroy(p)
		} else if m := fcMatch(p); m != 0 {
			if f, ok := fcDescribe(m); ok && f.scalable && fcHas(m, r) {
				file = f.file
			}
			fc.patternDestroy(m)
		}
	}
	fc.runes[r] = file
	return file, file.path != ""
}
