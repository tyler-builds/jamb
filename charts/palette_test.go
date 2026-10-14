package charts

import (
	"fmt"
	"math"
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// key returns the shape of a look without its color: its dash, marker and hatch.
func (l look) key() string {
	return fmt.Sprint(l.dash, " ", l.marker, " ", l.hatch)
}

// luminance returns the relative luminance of an opaque color, by the WCAG 2 definition.
func luminance(c ui.Color) float64 {
	lin := func(v float32) float64 {
		f := float64(v)
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// contrast returns the WCAG contrast ratio of two opaque colors, from 1 to 21.
func contrast(a, b ui.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Every series color of the light and the dark palettes is at least 3:1 against the
// surfaces a chart sits on: the theme's background and its surface.
func TestPaletteContrast(t *testing.T) {
	for _, th := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		colors := lightColors
		if th.Dark {
			colors = darkColors
		}
		for i, c := range colors {
			for _, bg := range []ui.Color{th.Background, th.Surface} {
				if r := contrast(c, bg); r < 3 {
					t.Errorf("%s palette color %d is %.2f:1 against %v, want 3:1", th.Name, i, r, bg)
				}
			}
		}
	}
}

// Under high contrast the text and the lines are the system's window text, which is at
// least 4.5:1 against the window, so the series are told apart by their dashes, markers and
// hatches rather than by their colors.
func TestHighContrastLooks(t *testing.T) {
	th := ui.HighContrastTheme(ui.HighContrastBlack(), ui.DarkTheme())
	if r := contrast(th.Text, th.Background); r < 4.5 {
		t.Errorf("window text is %.2f:1 against the window, want 4.5:1", r)
	}
	for i := 0; i < 6; i++ {
		l := seriesLook(th, i)
		if l.color != th.Text {
			t.Errorf("series %d under high contrast is not the text color: %v", i, l.color)
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 6; i++ {
		l := seriesLook(th, i)
		key := look{dash: l.dash, marker: l.marker, hatch: l.hatch}.key()
		if seen[key] {
			t.Errorf("series %d repeats the dash, marker and hatch of an earlier one: %s", i, key)
		}
		seen[key] = true
	}
}

// Consecutive series of a theme differ in color, and the first six in dash, marker and hatch.
func TestSeriesLooksAreDistinct(t *testing.T) {
	for _, th := range []*ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		for i := 1; i < len(lightColors); i++ {
			if seriesLook(th, i).color == seriesLook(th, i-1).color {
				t.Errorf("%s: series %d and %d share a color", th.Name, i-1, i)
			}
		}
		seen := map[string]bool{}
		for i := 0; i < 6; i++ {
			key := seriesLook(th, i).key()
			if seen[key] {
				t.Errorf("%s: series %d repeats the dash, marker and hatch of an earlier one", th.Name, i)
			}
			seen[key] = true
		}
	}
}
