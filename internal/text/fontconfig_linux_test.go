//go:build linux

package text

import "testing"

func TestFontconfigWeightScale(t *testing.T) {
	for _, c := range []struct {
		w  Weight
		fc int32
	}{{Light, 50}, {Regular, 80}, {Medium, 100}, {Semibold, 180}, {Bold, 200}} {
		if got := fcWeight(c.w); got != c.fc {
			t.Errorf("fcWeight(%d) = %d, want %d", c.w, got, c.fc)
		}
		if got := otWeight(c.fc); got != int32(c.w) {
			t.Errorf("otWeight(%d) = %d, want %d", c.fc, got, c.w)
		}
	}
}

// With libfontconfig, the sans family's regular face comes from fontconfig,
// loads, and is what the registry uses. Other faces it reports have the
// weight and slant asked for.
func TestFontconfigResolvesSans(t *testing.T) {
	if !loadFontconfig() {
		t.Skip("libfontconfig is not available")
	}
	faces := fontconfigFaces()
	reg, ok := faces[FamilySans][faceKey{Regular, false}]
	if !ok {
		t.Skip("fontconfig has no usable sans-serif font")
	}
	t.Logf("sans-serif: %v; mono: %v", faces[FamilySans], faces[FamilyMono])
	f, err := loadFile(reg)
	if err != nil {
		t.Fatalf("loading %v: %v", reg, err)
	}
	if got := NewRegistry().Font(FamilySans, Regular, false); got.path != reg.path {
		t.Errorf("the registry's sans font is %q, want fontconfig's %q", got.path, reg.path)
	}
	if !f.has('a') {
		t.Errorf("%s has no 'a'", reg.path)
	}
	for k, file := range faces[FamilySans] {
		if k != (faceKey{Regular, false}) && file == reg {
			t.Errorf("face %v is the regular face's file", k)
		}
	}
}

// A character no font of the registry's lists has is found through
// fontconfig, and the answer is kept.
func TestFontconfigRuneFallback(t *testing.T) {
	if !loadFontconfig() {
		t.Skip("libfontconfig is not available")
	}
	const r = 'ꦲ' // Javanese
	file, ok := fontconfigRune(r)
	if !ok {
		t.Skip("no font for U+A9B2 on this system")
	}
	reg := NewRegistry()
	f := reg.fallbackFor(r)
	if f == nil || !f.has(r) {
		t.Fatalf("fallbackFor(U+A9B2) = %v, want a font with the glyph (fontconfig: %v)", f, file)
	}
	if _, kept := reg.byRune[r]; !kept && f.path == file.path {
		t.Error("the fontconfig answer was not kept")
	}
}
