package ui

import (
	"fmt"
	"math"
	"strconv"

	"github.com/tyler-builds/jamb/internal/scene"
)

// Color is a straight-alpha sRGB color with components in [0,1].
// The zero value is transparent and, inside a Style, means "unset".
type Color struct{ R, G, B, A float32 }

// Transparent is fully transparent black.
var Transparent = Color{}

// RGB builds an opaque color from 0–255 components.
func RGB(r, g, b uint8) Color {
	return Color{float32(r) / 255, float32(g) / 255, float32(b) / 255, 1}
}

// RGBA builds a color from 0–255 components and a 0–1 alpha.
func RGBA(r, g, b uint8, a float32) Color {
	return Color{float32(r) / 255, float32(g) / 255, float32(b) / 255, a}
}

// Hex parses "#rgb", "#rrggbb" or "#rrggbbaa" (the '#' is optional).
// It panics with an explanation on malformed input, because colors are almost
// always literals and a typo should fail loudly at startup.
func Hex(s string) Color {
	c, err := ParseHex(s)
	if err != nil {
		panic(err)
	}
	return c
}

// ParseHex is Hex returning an error instead of panicking.
func ParseHex(s string) (Color, error) {
	h := s
	if len(h) > 0 && h[0] == '#' {
		h = h[1:]
	}
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 && len(h) != 8 {
		return Color{}, fmt.Errorf("ui.Hex(%q): expected #rgb, #rrggbb or #rrggbbaa", s)
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("ui.Hex(%q): %q is not hexadecimal; expected digits 0-9 and a-f", s, h)
	}
	if len(h) == 6 {
		v = v<<8 | 0xff
	}
	return Color{
		R: float32(v>>24&0xff) / 255,
		G: float32(v>>16&0xff) / 255,
		B: float32(v>>8&0xff) / 255,
		A: float32(v&0xff) / 255,
	}, nil
}

// Alpha returns c with its alpha multiplied by a.
func (c Color) Alpha(a float32) Color { c.A *= a; return c }

// Mix linearly interpolates from c to o by t in [0,1].
func (c Color) Mix(o Color, t float32) Color {
	return Color{c.R + (o.R-c.R)*t, c.G + (o.G-c.G)*t, c.B + (o.B-c.B)*t, c.A + (o.A-c.A)*t}
}

// Lighten mixes c toward white by t.
func (c Color) Lighten(t float32) Color { return c.Mix(Color{1, 1, 1, c.A}, t) }

// Darken mixes c toward black by t.
func (c Color) Darken(t float32) Color { return c.Mix(Color{0, 0, 0, c.A}, t) }

// IsZero reports whether c is the zero (unset) color.
func (c Color) IsZero() bool { return c == Color{} }

func (c Color) scene() scene.Color { return scene.Color(c) }

func (c Color) near(o Color) bool {
	const eps = 1.0 / 512
	return math.Abs(float64(c.R-o.R)) < eps && math.Abs(float64(c.G-o.G)) < eps &&
		math.Abs(float64(c.B-o.B)) < eps && math.Abs(float64(c.A-o.A)) < eps
}
