package charts

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// layerPoint is a point of the layer test's series.
type layerPoint struct {
	name string
	v    float64
}

// TestChartStyleLayersAreNamed checks that the title's font size is reported with the
// built-in default (15) overridden by the chart's Styles (20), which wins.
func TestChartStyleLayersAreNamed(t *testing.T) {
	series := Series[layerPoint]{Name: "Visits", Data: []layerPoint{{"a", 1}, {"b", 2}},
		Category: func(p layerPoint) string { return p.name }, Y: func(p layerPoint) float64 { return p.v }}
	chart := Bar(series).Title("Visits this week").Styles(Style{Title: ui.Style{FontSize: 20}})
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element { return ui.Div().Child(chart) }, ui.DarkTheme(), nil)
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
	for _, f := range tg.Fields(title) {
		if f.Name != "FontSize" {
			continue
		}
		if f.Source != inspect.SourceInstance || f.Value != "20" || len(f.Layers) != 2 ||
			f.Layers[0].Source != inspect.SourceDefault || f.Layers[0].Value != "15" || !f.Layers[0].Overridden {
			t.Fatalf("FontSize of the title = %+v, want 20 from the instance Styles over the default 15", f)
		}
		return
	}
	t.Fatal("the title has no FontSize")
}

// findTextElement returns the first text element of the subtree at id whose text is s.
func findTextElement(tg inspect.Target, id uint64, s string) (uint64, bool) {
	e, ok := tg.Element(id)
	if !ok {
		return 0, false
	}
	if e.Kind == "text" && e.Text == s {
		return id, true
	}
	for _, c := range e.Children {
		if f, ok := findTextElement(tg, c, s); ok {
			return f, true
		}
	}
	return 0, false
}
