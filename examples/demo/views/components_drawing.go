package views

import (
	"fmt"
	"math"

	"github.com/tyler-builds/jamb/ui"
)

// visits is the sample data of the canvas chart: a week of daily visits.
var visits = []float32{120, 180, 150, 240, 210, 300, 280}

var weekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// logoSVG is a small illustration in the style a design tool exports: a style
// sheet, a gradient, a clip path and an even-odd shape.
var logoSVG = ui.MustSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="160" height="96" viewBox="0 0 160 96">
<style>.sun{fill:#FFC83D}</style>
<defs>
<linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#4FC3F7"/><stop offset="1" stop-color="#E1F5FE"/></linearGradient>
<clipPath id="card"><rect width="160" height="96" rx="12"/></clipPath>
</defs>
<g clip-path="url(#card)">
<rect width="160" height="96" fill="url(#sky)"/>
<circle class="sun" cx="122" cy="30" r="16"/>
<path d="M0 80 Q 40 50 80 76 T 160 68 V 96 H 0 Z" fill="#2E7D32" fill-opacity="0.85"/>
<path fill-rule="evenodd" d="M34 22 L50 56 L18 56 Z M34 34 L42 51 L26 51 Z" fill="#fff"/>
</g></svg>`))

// checkSVG is a line icon drawn in currentColor, as icon sets ship them.
var checkSVG = ui.MustSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="m8 12 3 3 5-6"/></svg>`))

// drawingSection shows Canvas, SVG, selectable text and grid layout.
func drawingSection(t *ui.Theme) ui.Element {
	return section(t, "Drawing and layout",
		sample(t, "Canvas: a chart that follows the pointer", visitsChart(t)),
		sample(t, "SVG: an illustration, and an icon in the text color",
			ui.Div().Row().Gap(16).Align(ui.Center).
				Child(ui.SVG(logoSVG).Alt("A landscape with a sun and a mountain")).
				Child(ui.SVG(checkSVG).Size(ui.Px(32), ui.Px(32))).
				Child(ui.Div().TextColor(t.Accent).Child(ui.SVG(checkSVG).Size(ui.Px(32), ui.Px(32))))),
		sample(t, "Selectable text: drag, double-click, Ctrl+C",
			ui.Div().Width(ui.Px(320)).Child(
				ui.Text("Request failed with status 503 (Service Unavailable). Reference: 7f3a-91c2.").Selectable())),
		sample(t, "Grid: Auto, Fr(1) and Px(96) columns, a spanning field", gridSample()))
}

// visitsChart is a canvas line chart with a filled area, axis labels and a
// readout of the day under the pointer.
func visitsChart(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		hover := ui.Local(ctx, "hover", -1)
		h := hover.Get()
		accent, muted, grid := t.Accent, t.TextMuted, t.Border
		return ui.Canvas(func(p *ui.Painter, w, ht float32) {
			const left, bottom, top = 8, 22, 10
			plotH := ht - bottom - top
			x := func(i int) float32 { return left + float32(i)*(w-2*left)/float32(len(visits)-1) }
			y := func(v float32) float32 { return top + plotH - v/320*plotH }
			for _, v := range []float32{100, 200, 300} {
				p.Stroke(ui.NewPath().MoveTo(left, y(v)).LineTo(w-left, y(v)),
					ui.StrokeStyle{Width: 1, Color: grid, Dash: []float32{3, 3}})
			}
			line := ui.NewPath()
			for i, v := range visits {
				line.LineTo(x(i), y(v))
			}
			area := ui.NewPath().MoveTo(x(0), y(0))
			for i, v := range visits {
				area.LineTo(x(i), y(v))
			}
			area.LineTo(x(len(visits)-1), y(0)).Close()
			p.Fill(area, ui.LinearPaint(0, top, 0, top+plotH,
				ui.ColorStop{At: 0, Color: accent.Alpha(0.35)}, ui.ColorStop{At: 1, Color: accent.Alpha(0)}))
			p.Stroke(line, ui.StrokeStyle{Width: 2, Color: accent, Join: ui.JoinRound, Cap: ui.CapRound})
			for i, d := range weekdays {
				p.Text(x(i), ht-2, d, ui.TextStyle{Size: 11, Color: muted, Align: ui.Center, VAlign: ui.End})
			}
			if h >= 0 {
				p.Fill(ui.NewPath().Circle(x(h), y(visits[h]), 4.5), accent)
				p.Text(x(h), y(visits[h])-8, fmt.Sprintf("%.0f", visits[h]),
					ui.TextStyle{Size: 12, Weight: ui.FontSemibold, Align: ui.Center, VAlign: ui.End})
			}
		}).Size(ui.Px(360), ui.Px(150)).
			Label("Visits this week, from 120 on Monday to 280 on Sunday, highest on Saturday at 300").
			OnMouseMove(func(e *ui.MouseEvent) {
				step := (e.Width - 16) / float32(len(visits)-1)
				i := int(math.Round(float64((e.LocalX - 8) / step)))
				hover.Set(min(max(i, 0), len(visits)-1))
			}).
			OnMouseLeave(func() { hover.Set(-1) })
	})
}

// gridSample is a small form laid out as a grid.
func gridSample() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		name := ui.Local(ctx, "name", "")
		email := ui.Local(ctx, "email", "")
		notes := ui.Local(ctx, "notes", "")
		return ui.Div().Width(ui.Px(420)).Grid(ui.Auto, ui.Fr(1), ui.Px(96)).Gap(8).Align(ui.Center).
			Child(ui.Text("Name")).Child(ui.TextInput(name).Label("Name")).Child(ui.Button("Check").Small()).
			Child(ui.Text("Email")).Child(ui.TextInput(email).Label("Email").GridSpan(2, 1)).
			Child(ui.Text("Notes")).Child(ui.TextArea(notes).Label("Notes").GridSpan(2, 1))
	})
}
