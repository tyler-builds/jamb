package charts

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// legendHidden is the opacity of a legend item whose series or slice is hidden.
const legendHidden = 0.45

// legendRow returns the legend of the chart in a row, above or below the canvas, or
// nil when the chart has no legend there.
func (c *Chart) legendRow(v *view, hidden *ui.Signal[[]bool], cursor *ui.Signal[cursorAt]) ui.Element {
	if c.legend != LegendTop && c.legend != LegendBottom || c.items() == 0 {
		return nil
	}
	row := ui.Div().Role(access.RoleGroup).Label(v.s.ChartLegend).Row().Style(v.st.Legend)
	for i := 0; i < c.items(); i++ {
		row.Child(c.legendItem(v, hidden, cursor, i))
	}
	return row
}

// legendColumn returns the legend of the chart in a column beside the canvas, for a
// legend on the right. It is nil when the legend is elsewhere.
func (c *Chart) legendColumn(v *view, hidden *ui.Signal[[]bool], cursor *ui.Signal[cursorAt]) ui.Element {
	if c.legend != LegendRight || c.items() == 0 {
		return nil
	}
	col := ui.Div().Role(access.RoleGroup).Label(v.s.ChartLegend).Column().Style(v.st.Legend).
		Shrink(0).MinHeight(ui.Px(0)).ScrollY()
	for i := 0; i < c.items(); i++ {
		col.Child(c.legendItem(v, hidden, cursor, i))
	}
	return col
}

// legendItem returns the button of legend item i. It is a toggle: its pressed state
// is whether the series or slice is shown, and a click shows or hides it. A hidden
// item is faded, and its text keeps its weight.
func (c *Chart) legendItem(v *view, hidden *ui.Signal[[]bool], cursor *ui.Signal[cursorAt], i int) ui.Element {
	name := c.itemName(v.s, i)
	on := !v.isHidden(i)
	item := ui.Div().Role(access.RoleButton).Label(name).AccessChecked(on).Focusable().TabStop(!c.noTab).
		Style(v.st.LegendItem).Row().Align(ui.Center).Child(c.swatch(v, i)).Child(ui.Text(name).NoWrap())
	item.FocusVisible(ui.Style{BorderWidth: 2, BorderColor: v.t.FocusRing})
	if !on {
		item.Style(ui.Style{Opacity: legendHidden})
	}
	item.OnClick(func() {
		// The flags and the position are read as they are now, so that two clicks before
		// the next render each toggle from the result of the one before.
		now, cur := hidden.Peek(), cursor.Peek()
		next := toggled(now, c.items(), i)
		hidden.Set(next)
		// A hidden series or slice cannot hold the keyboard position, so the
		// position moves to the first one shown.
		if cur.on && c.isCursorHidden(next, cur) {
			if start, ok := c.startCursor(&view{c: c, hidden: next}); ok {
				cursor.Set(start)
			}
		}
	})
	return item
}

// swatch returns the key of legend item i: a short line with the series' dash and
// marker for a line, a scatter plot or a radar chart, and a filled box, hatched under
// high contrast, for the other kinds. It is hidden from assistive technology, which
// reads the name.
func (c *Chart) swatch(v *view, i int) ui.Element {
	l := c.lookOf(v.t, i)
	lined := c.kind == kindLine || c.kind == kindScatter || c.kind == kindRadar
	return ui.Canvas(func(p *ui.Painter, w, h float32) {
		if lined {
			if c.kind != kindScatter {
				p.Stroke(ui.NewPath().MoveTo(0, h/2).LineTo(w, h/2),
					ui.StrokeStyle{Width: 2, Color: l.color, Dash: l.dash})
			}
			drawMarker(p, l.marker, w/2, h/2, 3, l.color)
			return
		}
		box := ui.NewPath().RoundRect(0, 0, w, h, 2)
		if v.t.HighContrast {
			p.Fill(box, v.t.Surface)
			hatchFill(p, box, l.hatch, v.t.Text, 0, 0, w, h)
			p.Stroke(box, ui.StrokeStyle{Width: 1, Color: v.t.Text})
			return
		}
		p.Fill(box, l.color)
	}).Size(ui.Px(14), ui.Px(10)).AccessHidden()
}

// itemName returns the name of legend item i: the name of its series, or for a
// pie the label of its slice. A series without a name is named by its number, with
// the ChartSeriesNumber of s, as "Series 2".
func (c *Chart) itemName(s *ui.Strings, i int) string {
	if c.kind == kindPie || c.kind == kindDonut {
		return c.xLabel(c.ds.series[0].pts[i])
	}
	if name := c.ds.series[i].name; name != "" {
		return name
	}
	return s.ChartSeriesNumber(i + 1)
}

// toggled returns the hidden flags of n items with item i's flag flipped. The
// flags are copied, so a signal's value is never changed in place.
func toggled(hidden []bool, n, i int) []bool {
	out := make([]bool, n)
	copy(out, hidden)
	out[i] = !out[i]
	return out
}

// isCursorHidden reports whether the keyboard position is on an item that the flags
// hide. A Sankey diagram has no items to hide.
func (c *Chart) isCursorHidden(hidden []bool, cur cursorAt) bool {
	if c.kind == kindSankey {
		return false
	}
	item := cur.series
	if c.kind == kindPie || c.kind == kindDonut {
		item = cur.index
	}
	return item < len(hidden) && hidden[item]
}
