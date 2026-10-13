package svg

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/internal/vector"
)

// render draws src into a w x h canvas with black as currentColor.
func render(t *testing.T, src string, w, h int) *vector.Canvas {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	c := vector.NewCanvas(w, h)
	d.Draw(c, float32(w), float32(h), vector.Color{A: 1})
	return c
}

func px(c *vector.Canvas, x, y int) [4]uint8 {
	img := c.Image()
	p := img.Pix[y*img.Stride+x*4:]
	return [4]uint8{p[0], p[1], p[2], p[3]}
}

func area(c *vector.Canvas) float64 {
	var a float64
	pix := c.Image().Pix
	for i := 3; i < len(pix); i += 4 {
		a += float64(pix[i]) / 255
	}
	return a
}

func near(a, b, tol float64) bool { return a >= b-tol && a <= b+tol }

const red, green, blue = "\xff\x00\x00", "\x00\x80\x00", "\x00\x00\xff"

func isColor(p [4]uint8, rgb string) bool {
	return p[3] == 255 && p[0] == rgb[0] && p[1] == rgb[1] && p[2] == rgb[2]
}

// Path data: implicit commands, numbers without separators, relative moves,
// smooth curves, arc flags written together, and closing.
func TestPathData(t *testing.T) {
	cases := []struct {
		d    string
		area float64
	}{
		{"M10 10 20 10 20 20 10 20z", 100},             // pairs after M are lines
		{"M10,10h10v10h-10z", 100},                     // relative H and V
		{"m10 10 10 0 0 10-10 0z", 100},                // relative m, then l
		{"M10-0L20.5.5", 0},                            // a line has no area
		{"M0 0h10v10H0zm20 0h10v10h-10z", 200},         // m after z starts from the start
		{"M10 20a10 10 0 001 20 0z", 157},              // flags without separators
		{"M0 0L10 0L10 1e1L0 10Z", 100},                // exponents
		{"M0,0 L10,0 L10,10 L0,10 Z garbage 5 5", 100}, // what parsed before the error stays
	}
	for _, c := range cases {
		var p vector.Path
		parsePath(c.d, &p)
		cv := vector.NewCanvas(40, 40)
		cv.Fill(&p, vector.Paint{Color: vector.Color{A: 1}}, vector.NonZero)
		if got := area(cv); !near(got, c.area, c.area*0.02+0.5) {
			t.Errorf("%q: area %v, want %v", c.d, got, c.area)
		}
	}
	// S reflects the last control point: this S curve is symmetric with the C.
	var p vector.Path
	parsePath("M0 20 C0 0 10 0 10 20 S20 40 20 20", &p)
	if cur, _ := p.Current(); cur != (vector.Point{X: 20, Y: 20}) {
		t.Errorf("S ended at %v", cur)
	}
}

func TestColors(t *testing.T) {
	cur := vector.Color{R: 0.5, A: 1}
	for _, c := range []struct {
		s    string
		want vector.Color
	}{
		{"#f00", vector.Color{R: 1, A: 1}},
		{"#00FF0080", vector.Color{G: 1, A: 128.0 / 255}},
		{"rgb(0, 0, 255)", vector.Color{B: 1, A: 1}},
		{"rgba(100%, 0%, 0%, 0.5)", vector.Color{R: 1, A: 0.5}},
		{"hsl(120, 100%, 50%)", vector.Color{G: 1, A: 1}},
		{"Gold", vector.Color{R: 1, G: 215.0 / 255, A: 1}},
		{"currentColor", cur},
		{"transparent", vector.Color{}},
	} {
		got, ok := parseColor(c.s, cur)
		if !ok || !near(float64(got.R), float64(c.want.R), 0.01) || !near(float64(got.G), float64(c.want.G), 0.01) ||
			!near(float64(got.B), float64(c.want.B), 0.01) || !near(float64(got.A), float64(c.want.A), 0.01) {
			t.Errorf("%q = %v, %v; want %v", c.s, got, ok, c.want)
		}
	}
	if _, ok := parseColor("#12", cur); ok {
		t.Error("#12 parsed")
	}
}

// The size comes from width and height, else the viewBox, else 300 x 150.
func TestSize(t *testing.T) {
	for _, c := range []struct {
		attrs string
		w, h  float32
	}{
		{`width="48" height="32"`, 48, 32},
		{`viewBox="0 0 24 24"`, 24, 24},
		{`width="2in" viewBox="0 0 10 5"`, 192, 96},
		{`width="100%" height="100%" viewBox="0 0 20 10"`, 20, 10},
		{``, 300, 150},
	} {
		d, err := Parse([]byte(`<svg xmlns="http://www.w3.org/2000/svg" ` + c.attrs + `/>`))
		if err != nil {
			t.Fatal(err)
		}
		if d.W != c.w || d.H != c.h {
			t.Errorf("%s: %vx%v, want %vx%v", c.attrs, d.W, d.H, c.w, c.h)
		}
	}
}

// The viewBox scales the drawing to the box, keeping its aspect ratio and
// centering it unless preserveAspectRatio says otherwise.
func TestViewBox(t *testing.T) {
	c := render(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="red"/></svg>`, 40, 20)
	// Meet: a 20x20 square in the middle of 40x20.
	if !isColor(px(c, 20, 10), red) || px(c, 5, 10)[3] != 0 || !near(area(c), 400, 1) {
		t.Errorf("meet: middle %v, left %v, area %v", px(c, 20, 10), px(c, 5, 10), area(c))
	}
	c = render(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" preserveAspectRatio="none"><rect width="10" height="10" fill="red"/></svg>`, 40, 20)
	if !near(area(c), 800, 1) {
		t.Errorf("none: area %v, want the whole box", area(c))
	}
	c = render(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" preserveAspectRatio="xMinYMid meet"><rect width="10" height="10" fill="red"/></svg>`, 40, 20)
	if !isColor(px(c, 5, 10), red) || px(c, 30, 10)[3] != 0 {
		t.Error("xMinYMid did not put the square at the left")
	}
}

// Style sheets, style attributes and presentation attributes, in CSS order;
// inheritance from groups; currentColor; display and visibility.
func TestStyles(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="10">
	<style>/* Illustrator */ .a { fill: #00f } rect.b { fill: green } #c { fill: red }</style>
	<g fill="red">
		<rect x="0" width="10" height="10" class="a"/>
		<rect x="10" width="10" height="10" class="a b"/>
		<rect x="20" width="10" height="10" class="a" id="c" style="fill: blue"/>
		<rect x="30" width="10" height="10" fill="currentColor"/>
	</g>
	<rect width="40" height="10" display="none"/>
	<rect width="40" height="10" visibility="hidden"/>
	</svg>`
	c := render(t, src, 40, 10)
	if !isColor(px(c, 5, 5), blue) || !isColor(px(c, 15, 5), green) || !isColor(px(c, 25, 5), blue) {
		t.Errorf("class %v, type+class %v, style attribute %v", px(c, 5, 5), px(c, 15, 5), px(c, 25, 5))
	}
	if got := px(c, 35, 5); got != [4]uint8{0, 0, 0, 255} {
		t.Errorf("currentColor %v", got)
	}
}

// Gradients in bounding-box units fill the shape's box; href passes stops on.
func TestGradients(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100" height="20">
	<defs>
		<linearGradient id="base"><stop offset="0" stop-color="red"/><stop offset="100%" stop-color="blue"/></linearGradient>
		<linearGradient id="g" xlink:href="#base"/>
		<radialGradient id="r" cx="50%" cy="50%" r="50%"><stop offset="0" stop-color="#fff"/><stop offset="1" stop-color="#000"/></radialGradient>
	</defs>
	<rect x="0" y="0" width="50" height="10" fill="url(#g)"/>
	<rect x="50" y="0" width="50" height="20" fill="url(#r)"/>
	<rect x="0" y="10" width="50" height="10" fill="url(#missing) green"/>
	</svg>`
	c := render(t, src, 100, 20)
	if l, r := px(c, 1, 5), px(c, 48, 5); l[0] < 240 || r[2] < 240 {
		t.Errorf("linear: left %v, right %v", l, r)
	}
	if mid, edge := px(c, 75, 10), px(c, 51, 1); mid[0] < 230 || edge[0] > 40 {
		t.Errorf("radial: center %v, corner %v", mid, edge)
	}
	if !isColor(px(c, 25, 15), green) {
		t.Errorf("fallback color %v", px(c, 25, 15))
	}
}

// Transforms, use, clip paths, group opacity, strokes and the even-odd rule.
func TestStructure(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="100" height="40">
	<defs>
		<rect id="sq" width="10" height="10" fill="red"/>
		<clipPath id="half"><rect width="5" height="40"/></clipPath>
	</defs>
	<use xlink:href="#sq" x="20"/>
	<g transform="translate(40 0) scale(2)"><use href="#sq"/></g>
	<rect x="0" width="10" height="10" fill="blue" clip-path="url(#half)"/>
	<g opacity="0.5"><rect x="70" width="10" height="10" fill="red"/><rect x="75" width="10" height="10" fill="red"/></g>
	<path d="M0 20h20v20H0z M5 25h10v10H5z" fill-rule="evenodd"/>
	<line x1="30" y1="30" x2="60" y2="30" stroke="blue" stroke-width="4"/>
	</svg>`
	c := render(t, src, 100, 40)
	if !isColor(px(c, 25, 5), red) {
		t.Errorf("use at x=20: %v", px(c, 25, 5))
	}
	if !isColor(px(c, 58, 18), red) {
		t.Errorf("use in a scaled group: %v", px(c, 58, 18))
	}
	if !isColor(px(c, 2, 5), blue) || px(c, 7, 5)[3] != 0 {
		t.Errorf("clip: inside %v, outside %v", px(c, 2, 5), px(c, 7, 5))
	}
	if one, both := px(c, 72, 5)[3], px(c, 77, 5)[3]; !near(float64(one), 128, 1) || one != both {
		t.Errorf("group opacity: %d alone, %d overlapping", one, both)
	}
	if px(c, 10, 30)[3] != 0 || px(c, 2, 22)[3] != 255 {
		t.Error("the even-odd hole is filled, or the ring is not")
	}
	if !isColor(px(c, 45, 29), blue) || px(c, 45, 33)[3] != 0 {
		t.Errorf("stroke: on %v, off %v", px(c, 45, 29), px(c, 45, 33))
	}
}

// Broken or hostile documents return errors or draw what they can, and do not
// hang: a use that refers to itself stops.
func TestRobust(t *testing.T) {
	for _, bad := range []string{"", "<html/>", "<svg><g></svg", "not xml at all"} {
		if _, err := Parse([]byte(bad)); err == nil && !strings.HasPrefix(bad, "<svg") {
			t.Errorf("%q parsed", bad)
		}
	}
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><g id="loop"><use href="#loop"/><rect width="10" height="10"/></g></svg>`
	if c := render(t, src, 10, 10); area(c) < 99 {
		t.Errorf("self-referring use: area %v", area(c))
	}
	render(t, `<svg xmlns="http://www.w3.org/2000/svg"><rect width="-5" height="10"/><circle r="0"/><path d="M"/><text>hi</text></svg>`, 10, 10)
}
