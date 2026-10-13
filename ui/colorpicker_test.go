package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// colorPickerNear reports whether a and b differ by no more than eps.
func colorPickerNear(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

// colorPickerRig is a color picker keyed "pick" and labelled "Fill", with the value v, the
// presets swatches and the alpha option alpha, and an outside button. changes records
// the colors the picker reports to OnChange.
func colorPickerRig(v *Signal[Color], changes *[]Color, alpha bool, swatches ...Color) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		cp := ColorPicker(v).Key("pick").Label("Fill").OnChange(func(c Color) { *changes = append(*changes, c) })
		if alpha {
			cp.Alpha()
		}
		if len(swatches) > 0 {
			cp.Swatches(swatches...)
		}
		return Div().Row().Padding(20).Gap(20).Align(Start).
			Child(cp).
			Child(Button("Elsewhere").Key("outside"))
	})
}

// hasAccess reports whether the access tree has a node of role and name.
func hasAccess(r *accessRig, role access.Role, name string) bool {
	return r.tree().Find(role, name) != nil
}

// TestColorPickerConversions checks the hue, saturation and value of known colors,
// and that hsvToRGB inverts rgbToHSV, over a grid of colors.
func TestColorPickerConversions(t *testing.T) {
	cases := []struct {
		name    string
		rgb     [3]float64
		h, s, v float64
		eps     float64 // the tolerance of the hue
	}{
		{"red", [3]float64{1, 0, 0}, 0, 1, 1, 1e-9},
		{"yellow", [3]float64{1, 1, 0}, 60, 1, 1, 1e-9},
		{"green", [3]float64{0, 1, 0}, 120, 1, 1, 1e-9},
		{"cyan", [3]float64{0, 1, 1}, 180, 1, 1, 1e-9},
		{"blue", [3]float64{0, 0, 1}, 240, 1, 1, 1e-9},
		{"magenta", [3]float64{1, 0, 1}, 300, 1, 1, 1e-9},
		{"white", [3]float64{1, 1, 1}, 0, 0, 1, 1e-9},
		{"black", [3]float64{0, 0, 0}, 0, 0, 0, 1e-9},
		{"mid grey", [3]float64{0.5, 0.5, 0.5}, 0, 0, 0.5, 1e-9},
		{"#3B82F6", [3]float64{59.0 / 255, 130.0 / 255, 246.0 / 255}, 217.22, 187.0 / 246, 246.0 / 255, 0.01},
	}
	for _, c := range cases {
		h, s, v := rgbToHSV(c.rgb[0], c.rgb[1], c.rgb[2])
		if !colorPickerNear(h, c.h, c.eps) || !colorPickerNear(s, c.s, 1e-9) || !colorPickerNear(v, c.v, 1e-9) {
			t.Errorf("%s: rgbToHSV = (%.6f, %.6f, %.6f), want (%.6f, %.6f, %.6f)", c.name, h, s, v, c.h, c.s, c.v)
		}
		r, g, b := hsvToRGB(c.h, c.s, c.v)
		if !colorPickerNear(r, c.rgb[0], 0.01) || !colorPickerNear(g, c.rgb[1], 0.01) || !colorPickerNear(b, c.rgb[2], 0.01) {
			t.Errorf("%s: hsvToRGB = (%.6f, %.6f, %.6f), want (%.6f, %.6f, %.6f)", c.name, r, g, b, c.rgb[0], c.rgb[1], c.rgb[2])
		}
	}

	// The round trip is exact to rounding over a grid of colors.
	for r := 0.0; r <= 1; r += 0.05 {
		for g := 0.0; g <= 1; g += 0.05 {
			for b := 0.0; b <= 1; b += 0.05 {
				h, s, v := rgbToHSV(r, g, b)
				br, bg, bb := hsvToRGB(h, s, v)
				if !colorPickerNear(br, r, 1e-9) || !colorPickerNear(bg, g, 1e-9) || !colorPickerNear(bb, b, 1e-9) {
					t.Fatalf("round trip of (%v, %v, %v) gave (%v, %v, %v)", r, g, b, br, bg, bb)
				}
			}
		}
	}

	// The hue wraps: 360 is red, and -120 is blue.
	if r, g, b := hsvToRGB(360, 1, 1); r != 1 || g != 0 || b != 0 {
		t.Errorf("hue 360 = (%v, %v, %v), want red", r, g, b)
	}
	if r, g, b := hsvToRGB(-120, 1, 1); r != 0 || g != 0 || b != 1 {
		t.Errorf("hue -120 = (%v, %v, %v), want blue", r, g, b)
	}
}

// TestColorPickerParseHex checks the hex text that the picker accepts, with and
// without alpha.
func TestColorPickerParseHex(t *testing.T) {
	cases := []struct {
		text  string
		alpha bool
		want  Color
		ok    bool
	}{
		{"#3B82F6", false, RGB(0x3B, 0x82, 0xF6), true},
		{"3b82f6", false, RGB(0x3B, 0x82, 0xF6), true},
		{"#abc", false, RGB(0xAA, 0xBB, 0xCC), true},
		{"#3B82F680", true, RGBA(0x3B, 0x82, 0xF6, float32(0x80)/255), true},
		{"#3B82F6FF", true, RGB(0x3B, 0x82, 0xF6), true},
		{"#3B82F680", false, Color{}, false},
		{"#GGGGGG", false, Color{}, false},
		{"#12345", false, Color{}, false},
		{"", false, Color{}, false},
	}
	for _, c := range cases {
		got, ok := parseHex(c.text, c.alpha)
		if ok != c.ok {
			t.Errorf("parseHex(%q, %v) ok = %v, want %v", c.text, c.alpha, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseHex(%q, %v) = %v, want %v", c.text, c.alpha, got, c.want)
		}
	}
}

// TestColorPickerFormatHex checks the hex text the picker shows, and that it parses
// back to the same color.
func TestColorPickerFormatHex(t *testing.T) {
	if got := formatHex(Hex("#3b82f6"), true); got != "#3B82F6" {
		t.Errorf("an opaque color with alpha formats as %q, want #3B82F6", got)
	}
	if got := formatHex(Hex("#3b82f6"), false); got != "#3B82F6" {
		t.Errorf("formatHex = %q, want #3B82F6", got)
	}
	if got := formatHex(Hex("#3b82f680"), true); got != "#3B82F680" {
		t.Errorf("formatHex with alpha = %q, want #3B82F680", got)
	}
	for i := 0; i <= 255; i += 5 {
		c := RGBA(uint8(i), uint8(255-i), uint8(i/2), float32(i)/255)
		back, ok := parseHex(formatHex(c, true), true)
		if !ok || back != c {
			t.Fatalf("%v formats to %q and parses to %v, ok %v", c, formatHex(c, true), back, ok)
		}
	}
}

// TestColorPickerOpensAndClosesWithFocus checks that a click opens the panel with
// focus on the area, that the trigger reports itself expanded, and that Escape closes
// the panel and returns focus to the trigger.
func TestColorPickerOpensAndClosesWithFocus(t *testing.T) {
	var changes []Color
	v := NewSignal(Hex("#3B82F6"))
	r := colorPickerRig(v, &changes, false)
	r.clickKey(t, "pick")
	accesstest.Find(t, r.tree(), access.RoleDialog, "Fill")
	if !r.focusedAccess(t, access.RoleSlider, "Saturation and brightness") {
		t.Fatal("focus did not move to the saturation and brightness area")
	}
	tr := accesstest.Find(t, r.tree(), access.RoleButton, "Fill")
	if !tr.States.Has(access.StateExpanded) {
		t.Error("the trigger is not reported expanded")
	}

	r.press(input.KeyEscape, 0)
	accesstest.Absent(t, r.tree(), access.RoleDialog, "Fill")
	if !r.rt.focusWithin("pick") {
		t.Fatal("focus did not return to the trigger")
	}
	if len(changes) != 0 {
		t.Errorf("opening and closing reported %d changes, want none", len(changes))
	}
}

// TestColorPickerOpensFromTheKeyboard checks that Enter on the focused trigger opens
// the panel, and that a click outside closes it.
func TestColorPickerOpensFromTheKeyboard(t *testing.T) {
	var changes []Color
	r := colorPickerRig(NewSignal(Hex("#3B82F6")), &changes, false)
	r.focusKey(t, "pick")
	r.press(input.KeyEnter, 0)
	accesstest.Find(t, r.tree(), access.RoleDialog, "Fill")
	r.clickKey(t, "outside")
	accesstest.Absent(t, r.tree(), access.RoleDialog, "Fill")
}

// TestColorPickerAreaKeys checks that the arrows move the saturation and the
// brightness by 1 percent, and by 10 percent with Shift, and keep the hue.
func TestColorPickerAreaKeys(t *testing.T) {
	var changes []Color
	v := NewSignal(Hex("#3B82F6"))
	r := colorPickerRig(v, &changes, false)
	r.clickKey(t, "pick")
	h0, s0, b0 := rgbToHSV(float64(v.Get().R), float64(v.Get().G), float64(v.Get().B))

	type step struct {
		key  input.Key
		mods input.Modifiers
		ds   float64 // the change of saturation
		db   float64 // the change of brightness
	}
	steps := []step{
		{input.KeyRight, 0, 0.01, 0},
		{input.KeyDown, 0, 0, -0.01},
		{input.KeyLeft, input.ModShift, -0.1, 0},
		{input.KeyDown, input.ModShift, 0, -0.1},
		{input.KeyUp, input.ModShift, 0, 0.1},
	}
	s, b := s0, b0
	for _, st := range steps {
		r.press(st.key, st.mods)
		s, b = s+st.ds, b+st.db
		h, gotS, gotB := rgbToHSV(float64(v.Get().R), float64(v.Get().G), float64(v.Get().B))
		if !colorPickerNear(gotS, s, 1e-3) || !colorPickerNear(gotB, b, 1e-3) {
			t.Errorf("after key %v with mods %v: saturation %.4f brightness %.4f, want %.4f %.4f",
				st.key, st.mods, gotS, gotB, s, b)
		}
		if !colorPickerNear(h, h0, 0.5) {
			t.Errorf("after key %v: hue %.3f, want %.3f", st.key, h, h0)
		}
	}
}

// TestColorPickerHueHoldsWhenGrey checks that a color brought to grey keeps the hue
// the user chose, so that moving the saturation back up returns to that hue.
func TestColorPickerHueHoldsWhenGrey(t *testing.T) {
	var changes []Color
	v := NewSignal(Hex("#3B82F6"))
	r := colorPickerRig(v, &changes, false)
	r.clickKey(t, "pick")
	for range 100 {
		r.press(input.KeyLeft, 0)
	}
	if g := v.Get(); g.R != g.G || g.G != g.B {
		t.Fatalf("saturation 0 gave %v, want a grey", g)
	}
	r.press(input.KeyRight, 0)
	h, _, _ := rgbToHSV(float64(v.Get().R), float64(v.Get().G), float64(v.Get().B))
	if !colorPickerNear(h, 217.22, 0.5) {
		t.Errorf("hue after leaving grey is %.2f, want about 217.22", h)
	}
}

// TestColorPickerPresetSetsValue checks that a preset swatch, clicked, sets the value
// and reports it to OnChange once.
func TestColorPickerPresetSetsValue(t *testing.T) {
	var changes []Color
	v := NewSignal(Hex("#3B82F6"))
	r := colorPickerRig(v, &changes, false, Hex("#EF4444"), Hex("#22C55E"))
	r.clickKey(t, "pick")
	accesstest.Find(t, r.tree(), access.RoleButton, "#22C55E")
	r.clickKey(t, "pick/preset:1")
	if got := v.Get(); got != Hex("#22C55E") {
		t.Fatalf("value after the preset is %v, want #22C55E", got)
	}
	if len(changes) != 1 || changes[0] != Hex("#22C55E") {
		t.Errorf("OnChange saw %v, want one change to #22C55E", changes)
	}
}

// TestColorPickerHexEnterSetsValue checks that a valid hex text, applied with Enter,
// sets the value.
func TestColorPickerHexEnterSetsValue(t *testing.T) {
	var changes []Color
	v := NewSignal(Hex("#3B82F6"))
	r := colorPickerRig(v, &changes, false)
	r.clickKey(t, "pick")
	r.clickKey(t, "pick/hex")
	r.press(input.KeyEnd, 0)
	for range 7 {
		r.press(input.KeyBackspace, 0)
	}
	r.typeText("#22c55e")
	r.press(input.KeyEnter, 0)
	if got := v.Get(); got != Hex("#22C55E") {
		t.Fatalf("value after typing the hex text is %v, want #22C55E", got)
	}
}

// TestColorPickerInvalidHexKeepsValue checks that text that is not a color, applied
// with Enter, leaves the value as it was.
func TestColorPickerInvalidHexKeepsValue(t *testing.T) {
	var changes []Color
	v := NewSignal(Hex("#3B82F6"))
	r := colorPickerRig(v, &changes, false)
	r.clickKey(t, "pick")
	r.clickKey(t, "pick/hex")
	r.press(input.KeyEnd, 0)
	for range 7 {
		r.press(input.KeyBackspace, 0)
	}
	r.typeText("#zz12")
	r.press(input.KeyEnter, 0)
	if got := v.Get(); got != Hex("#3B82F6") {
		t.Fatalf("value after invalid text is %v, want #3B82F6", got)
	}
	if len(changes) != 0 {
		t.Errorf("invalid text reported %d changes, want none", len(changes))
	}
}

// TestColorPickerAlphaSliderOnlyWithAlpha checks that the opacity slider is in the
// panel only when Alpha is set.
func TestColorPickerAlphaSliderOnlyWithAlpha(t *testing.T) {
	var changes []Color
	r := colorPickerRig(NewSignal(Hex("#3B82F6")), &changes, false)
	r.clickKey(t, "pick")
	if hasAccess(r, access.RoleSlider, "Opacity") {
		t.Error("the opacity slider is in the panel without Alpha")
	}
	r = colorPickerRig(NewSignal(Hex("#3B82F6")), &changes, true)
	r.clickKey(t, "pick")
	if !hasAccess(r, access.RoleSlider, "Opacity") {
		t.Error("the opacity slider is missing from the panel with Alpha")
	}
}

// TestColorPickerAccessNames checks the names and values that assistive technology
// sees: the trigger, the dialog, the area with its percentages, the hue and opacity
// sliders, and the presets. It also checks that the rig reports no warnings.
func TestColorPickerAccessNames(t *testing.T) {
	var changes []Color
	r := colorPickerRig(NewSignal(Hex("#3B82F6")), &changes, true, Hex("#EF4444"))
	r.clickKey(t, "pick")
	tr := accesstest.Find(t, r.tree(), access.RoleButton, "Fill")
	if !tr.States.Has(access.StateExpanded) {
		t.Error("the trigger is not reported expanded")
	}
	accesstest.Find(t, r.tree(), access.RoleDialog, "Fill")
	area := accesstest.Find(t, r.tree(), access.RoleSlider, "Saturation and brightness")
	if want := "Saturation 76%, brightness 96%"; area.Value != want {
		t.Errorf("area value is %q, want %q", area.Value, want)
	}
	accesstest.Find(t, r.tree(), access.RoleSlider, "Hue")
	accesstest.Find(t, r.tree(), access.RoleSlider, "Opacity")
	// With Alpha the presets are named by their eight-digit hex value, as the hex text is.
	accesstest.Find(t, r.tree(), access.RoleButton, "#EF4444") // opaque: no alpha digits
	if len(r.warnings) != 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}

	// Without a Label the trigger is named "Color".
	v := NewSignal(Hex("#3B82F6"))
	plain := newAccessRig(func(ctx *Context) Element { return ColorPicker(v).Key("pick") })
	accesstest.Find(t, plain.tree(), access.RoleButton, "Color")
}

// partColorPicker is a color picker keyed "pick", labelled "Fill", with the value v,
// the style st, the alpha option and the presets.
func partColorPicker(v *Signal[Color], st ColorPickerStyle, alpha bool, swatches ...Color) Element {
	cp := ColorPicker(v).Key("pick").Label("Fill").Styles(st)
	if alpha {
		cp.Alpha()
	}
	if len(swatches) > 0 {
		cp.Swatches(swatches...)
	}
	return Div().Column().Child(cp)
}

// testColorPickerParts checks the color picker's parts: each part's style reaches the
// node that draws it, in the state that the part is for.
func testColorPickerParts(t *testing.T, applied map[string]bool) {
	type pickerCase struct {
		part   string // the part, as docs/styling.md names it
		drawn  string // the part of the node that draws it, when it is not part
		set    func(*ColorPickerStyle, Style)
		swatch bool // the picker has presets
		open   bool // open the panel
		act    func(*testing.T, *accessRig)
		border bool // the part is checked by its border color, as its color is content
	}
	cases := []pickerCase{
		{part: "Trigger", set: func(s *ColorPickerStyle, m Style) { s.Trigger = m }},
		{part: "TriggerHover", drawn: "ColorPicker.Trigger", set: func(s *ColorPickerStyle, m Style) { s.TriggerHover = m },
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "ColorPicker.Trigger", "")) }},
		{part: "TriggerFocusVisible", drawn: "ColorPicker.Trigger", set: func(s *ColorPickerStyle, m Style) { s.TriggerFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "ColorPicker.Trigger", "")) }},
		{part: "Swatch", border: true, set: func(s *ColorPickerStyle, m Style) { s.Swatch = m }},
		{part: "Popup", open: true, set: func(s *ColorPickerStyle, m Style) { s.Popup = m }},
		{part: "Area", open: true, set: func(s *ColorPickerStyle, m Style) { s.Area = m }},
		// The panel has already given the area focus, by the pointer; a key switches to the
		// keyboard, which shows the focus.
		{part: "AreaFocusVisible", drawn: "ColorPicker.Area", open: true, set: func(s *ColorPickerStyle, m Style) { s.AreaFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { r.press(input.KeyRight, 0) }},
		{part: "Thumb", open: true, border: true, set: func(s *ColorPickerStyle, m Style) { s.Thumb = m }},
		{part: "Preset", swatch: true, open: true, border: true, set: func(s *ColorPickerStyle, m Style) { s.Preset = m }},
		{part: "PresetFocusVisible", drawn: "ColorPicker.Preset", swatch: true, open: true,
			set: func(s *ColorPickerStyle, m Style) { s.PresetFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "ColorPicker.Preset", "")) }},
	}
	for _, c := range cases {
		applied["ColorPicker."+c.part] = true
		t.Run("ColorPicker."+c.part, func(t *testing.T) {
			mark := Style{Background: markBG}
			if c.border {
				mark = Style{BorderColor: markBG}
			}
			var st ColorPickerStyle
			c.set(&st, mark)
			var swatches []Color
			if c.swatch {
				swatches = []Color{Hex("#3B82F6")}
			}
			v := NewSignal(Hex("#EF4444"))
			r := newAccessRig(func(ctx *Context) Element { return partColorPicker(v, st, false, swatches...) })
			if c.open {
				r.clickKey(t, "pick")
			}
			if c.act != nil {
				c.act(t, r)
			}
			part := c.drawn
			if part == "" {
				part = "ColorPicker." + c.part
			}
			n := partNode(t, r.rt, part, "")
			if c.border {
				if n.style.BorderColor != markBG {
					t.Errorf("%s: border color %v, want %v", c.part, n.style.BorderColor, markBG)
				}
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}

	// The Slider part is the parts of the hue and opacity sliders: both take its track.
	applied["ColorPicker.Slider"] = true
	t.Run("ColorPicker.Slider", func(t *testing.T) {
		st := ColorPickerStyle{Slider: SliderStyle{Track: Style{Background: markBG}}}
		v := NewSignal(Hex("#3B82F6"))
		r := newAccessRig(func(ctx *Context) Element { return partColorPicker(v, st, true) })
		r.clickKey(t, "pick")
		tracks := partNodes(r.rt, "Slider.Track")
		if len(tracks) != 2 {
			t.Fatalf("%d slider tracks in the panel, want 2 (hue and opacity)", len(tracks))
		}
		for _, n := range tracks {
			wantBackground(t, n, "ColorPicker.Slider", markBG)
		}
	})
}
