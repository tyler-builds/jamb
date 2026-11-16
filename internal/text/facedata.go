package text

import (
	"slices"
	"strconv"

	"golang.org/x/image/math/fixed"
)

// FaceData is the font file of a face that text is drawn with, for laying the
// same text out elsewhere: a web page's textarea, which lies over a text field
// on iOS so that the keyboard's trackpad finds the characters where jamb draws
// them (see internal/platform/web).
type FaceData struct {
	// Key names the face, the same for as long as the font is loaded and
	// unique to it: "jamb-face-" and a number.
	Key string
	// Data is the font file, which must not be changed.
	Data []byte
	// Weight is where a variable font is drawn on its weight axis, or 0 for
	// its default instance.
	Weight float32
}

// faceSet is the faces of a primary font, as Faces returns them, and the
// number of fallback fonts loaded when it was made.
type faceSet struct {
	loaded int
	faces  []FaceData
}

// Faces returns the faces that text of the style st is drawn with, in the
// order a browser takes the families of a font-family list: the face of st,
// then the fallback fonts that the app added and that have been loaded, then
// Go Regular, jamb's last resort, and last a font of one empty glyph as wide
// as the missing glyph of the face of st, for every character that none of
// them has, which jamb draws as that box (see notdefFont). A face still on its
// way, as a Go font a page fetches, is left out until it arrives, and the text
// is drawn without it meanwhile. The slice is shared and must not be changed;
// it stays the same while the fonts do.
func (sys *System) Faces(st Style) []FaceData {
	primary := sys.reg.Font(st.Family, st.Weight, st.Italic)
	if primary == nil {
		return nil
	}
	var fonts []*Font
	add := func(f *Font) {
		if f != nil && len(f.data) > 0 && !slices.Contains(fonts, f) {
			fonts = append(fonts, f)
		}
	}
	add(primary)
	for _, fb := range sys.reg.fallbacks[:sys.reg.appFallbacks] {
		if fb.font.done && fb.font.err == nil {
			add(fb.font.font)
		}
	}
	if l := sys.reg.last; l != nil && l.done && l.err == nil {
		add(l.font)
	}
	if sys.faceSets == nil {
		sys.faceSets = map[*Font]faceSet{}
	}
	if fs, ok := sys.faceSets[primary]; ok && fs.loaded == len(fonts) {
		return fs.faces
	}
	faces := make([]FaceData, len(fonts), len(fonts)+1)
	for i, f := range fonts {
		faces[i] = FaceData{Key: "jamb-face-" + strconv.FormatUint(uint64(f.id), 10), Data: f.data}
		if f.varied {
			faces[i].Weight = f.wght
		}
	}
	adv := primary.face.HorizontalAdvance(0)
	faces = append(faces, FaceData{Key: "jamb-notdef-" + strconv.FormatUint(uint64(primary.id), 10),
		Data: notdefFont(uint16(primary.upem), uint16(max(adv, 0)+0.5), int16(primary.ascent), int16(primary.descent))})
	sys.faceSets[primary] = faceSet{loaded: len(fonts), faces: faces}
	return faces
}

// LayoutSize returns the size whose advances a layout of text of the size
// size has: go-text scales a font to whole pixels, rounding up, so text of
// 13.5 pixels is laid out, and its caret placed, as text of 14 is, though its
// glyphs are drawn at 13.5. A browser that lays the same text out takes this
// size, to put each character where jamb does.
func LayoutSize(size float32) float32 {
	return float32(fixed.Int26_6(size*64 + 0.5).Ceil())
}
