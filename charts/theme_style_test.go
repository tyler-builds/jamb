package charts

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// resolvedIn returns the parts of the chart c as one render resolves them in the theme
// th, under high contrast when hc is true, and the theme the render used.
func resolvedIn(c *Chart, th *ui.Theme, hc bool) (Style, *ui.Theme) {
	var st Style
	var used *ui.Theme
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		st, used = c.resolveStyle(ctx), ctx.Theme()
		return ui.Div()
	}, th, nil)
	if hc {
		rt.SetSystemSettings(ui.SystemSettings{HighContrast: true})
	}
	rt.SetViewport(400, 300, 1)
	rt.Frame(time.Now())
	return st, used
}

// themedChart returns a theme whose charts.Style sets the title's size and color, the
// axis size and the tooltip's background, and a chart whose Styles set the title's size.
func themedChart() (*ui.Theme, *Chart) {
	th := ui.DarkTheme()
	ui.SetThemeStyle(th, Style{
		Title:   ui.Style{FontSize: 18, TextColor: ui.Hex("#ff0000")},
		Axis:    ui.Style{FontSize: 13},
		Tooltip: ui.Style{Background: ui.Hex("#00ff00")},
	})
	series := Series[layerPoint]{Name: "Visits", Data: []layerPoint{{"a", 1}, {"b", 2}},
		Category: func(p layerPoint) string { return p.name }, Y: func(p layerPoint) float64 { return p.v }}
	return th, Bar(series).Title("Visits this week").Styles(Style{Title: ui.Style{FontSize: 20}})
}

// The theme's charts.Style merges over the built-in defaults, field by field, and the
// chart's Styles over it.
func TestThemeStyleMergesBetweenDefaultsAndStyles(t *testing.T) {
	th, chart := themedChart()
	st, _ := resolvedIn(chart, th, false)
	if st.Title.FontSize != 20 {
		t.Errorf("title size = %v, want 20 from the Styles over the theme's 18", st.Title.FontSize)
	}
	if st.Title.TextColor != ui.Hex("#ff0000") || st.Axis.FontSize != 13 || st.Tooltip.Background != ui.Hex("#00ff00") {
		t.Errorf("title color %v, axis size %v, tooltip background %v; want the theme's", st.Title.TextColor, st.Axis.FontSize, st.Tooltip.Background)
	}
	if st.Title.FontWeight != ui.FontSemibold || st.Grid.BorderWidth != 1 || st.Axis.TextColor != th.TextMuted {
		t.Errorf("title weight %v, grid width %v, axis color %v; want the defaults the theme does not set",
			st.Title.FontWeight, st.Grid.BorderWidth, st.Axis.TextColor)
	}

	plainTheme := ui.DarkTheme()
	if st, _ := resolvedIn(chart, plainTheme, false); st.Axis.FontSize != 12 || st.Title.TextColor != plainTheme.Text {
		t.Errorf("without a theme style: axis size %v, title color %v; want the defaults", st.Axis.FontSize, st.Title.TextColor)
	}
}

// Under high contrast the theme's charts.Style keeps its sizes, and its colors give way
// to the system colors, as the colors of the Styles do.
func TestThemeStyleUnderHighContrast(t *testing.T) {
	th, chart := themedChart()
	st, used := resolvedIn(chart, th, true)
	if !used.HighContrast {
		t.Fatal("the render did not use the high contrast theme")
	}
	if st.Title.FontSize != 20 || st.Axis.FontSize != 13 {
		t.Errorf("title size %v, axis size %v; want the sizes of the Styles and the theme", st.Title.FontSize, st.Axis.FontSize)
	}
	if st.Title.TextColor != used.Text || st.Tooltip.Background == ui.Hex("#00ff00") {
		t.Errorf("title color %v, tooltip background %v; want the system colors, not the theme's", st.Title.TextColor, st.Tooltip.Background)
	}
}

// The inspector shows the theme's layer between the built-in default and the Styles.
func TestChartThemeStyleLayerIsNamed(t *testing.T) {
	th, chart := themedChart()
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element { return ui.Div().Child(chart) }, th, nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Now())
	tg, ok := uihost.Of(rt).(inspect.Target)
	if !ok {
		t.Fatal("the runtime's host side does not implement inspect.Target")
	}
	tg.Attach(func() {})
	rt.Frame(time.Now())

	title, ok := findTextElement(tg, tg.Root(), "Visits this week")
	if !ok {
		t.Fatal("no element with the title text")
	}
	var size, color bool
	for _, f := range tg.Fields(title) {
		switch f.Name {
		case "FontSize":
			size = true
			if f.Source != inspect.SourceInstance || len(f.Layers) != 3 ||
				f.Layers[0].Source != inspect.SourceDefault || !f.Layers[0].Overridden ||
				f.Layers[1].Source != inspect.SourceTheme || f.Layers[1].Value != "18" || !f.Layers[1].Overridden {
				t.Errorf("FontSize of the title = %+v, want the default and the theme's 18 under the Styles' 20", f)
			}
		case "TextColor":
			color = true
			if f.Source != inspect.SourceTheme {
				t.Errorf("TextColor of the title = %+v, want it from the theme", f)
			}
		}
	}
	if !size || !color {
		t.Fatalf("the title reports FontSize %v and TextColor %v", size, color)
	}
}
