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

// A series' Color replaces its palette color, and the zero value keeps the palette's; the
// other series keep the colors of their places. Under high contrast the app's color is
// ignored and the series is the text color, as every series is. A pie colors its slices
// from the palette.
func TestSeriesColor(t *testing.T) {
	brand := ui.Hex("#123456")
	second := returns()
	second.Color = brand
	c := Line(visits(), second)
	light, hc := ui.LightTheme(), ui.HighContrastTheme(ui.HighContrastBlack(), ui.DarkTheme())
	if got := c.lookOf(light, 0).color; got != lightColors[0] {
		t.Errorf("a series without a Color is %v, want the palette's first %v", got, lightColors[0])
	}
	if got := c.lookOf(light, 1).color; got != brand {
		t.Errorf("a series with a Color is %v, want %v", got, brand)
	}
	if got := c.lookOf(light, 1); got.key() != seriesLook(light, 1).key() {
		t.Errorf("a Color changed the dash, marker or hatch of the series: %s", got.key())
	}
	if got := c.lookOf(hc, 1).color; got != hc.Text {
		t.Errorf("under high contrast a series with a Color is %v, want the text color %v", got, hc.Text)
	}
	slices := Series[day]{Name: "Share", Data: week, Category: dayName, Y: visitOf, Color: brand}
	if got := Pie(slices).lookOf(light, 0).color; got != lightColors[0] {
		t.Errorf("a pie's first slice is %v, want the palette's %v", got, lightColors[0])
	}
}

// A candlestick's Rising and Falling colors replace the theme's Success and Danger, and
// the zero values keep them. Under high contrast every candle is the text color.
func TestCandleColors(t *testing.T) {
	up, down := ui.Hex("#1d4ed8"), ui.Hex("#c2410c")
	candles := func(rising, falling ui.Color) *Chart {
		return Candlestick(CandleSeries[day]{Name: "Price", Data: week, Category: dayName,
			Open: visitOf, High: visitOf, Low: visitOf, Close: visitOf, Rising: rising, Falling: falling})
	}
	light, hc := ui.LightTheme(), ui.HighContrastTheme(ui.HighContrastBlack(), ui.DarkTheme())
	set, unset := candles(up, down), candles(ui.Color{}, ui.Color{})
	if set.candleColor(light, true) != up || set.candleColor(light, false) != down {
		t.Errorf("set candle colors are %v and %v, want %v and %v",
			set.candleColor(light, true), set.candleColor(light, false), up, down)
	}
	if unset.candleColor(light, true) != light.Success || unset.candleColor(light, false) != light.Danger {
		t.Errorf("default candle colors are not the theme's Success and Danger")
	}
	if set.candleColor(hc, true) != hc.Text || set.candleColor(hc, false) != hc.Text {
		t.Errorf("under high contrast the candles are not the text color")
	}
}

// Consecutive series of a theme differ in color, and the first six in dash, marker and hatch.
// The check covers the palette: the colors that an app sets with Series.Color are its own
// choice, and the chart does not check them.
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
