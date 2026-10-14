package charts

import (
	"fmt"
	"strings"
)

// typeName returns the name of the chart's type, as the summary begins with it.
func (c *Chart) typeName() string {
	stacked := ""
	if c.stack {
		stacked = "stacked "
	}
	switch c.kind {
	case kindLine:
		return "Line chart"
	case kindArea:
		return capitalize(stacked + "area chart")
	case kindBar:
		if c.horiz {
			return capitalize(stacked + "horizontal bar chart")
		}
		return capitalize(stacked + "bar chart")
	case kindScatter:
		return "Scatter plot"
	case kindPie:
		return "Pie chart"
	case kindDonut:
		return "Donut chart"
	case kindRadar:
		return "Radar chart"
	case kindCandle:
		return "Candlestick chart"
	}
	return "Sankey diagram"
}

// capitalize returns s with its first letter in capitals.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// summary returns the name of the chart for assistive technology, made from its data: its
// type, its series and their points, the range of its positions and the range of each
// series' values, as in "Line chart, 2 series, 7 points each, from Monday to Sunday; Visits
// from 120 to 300". A chart with Label set is named by that instead.
func (c *Chart) summary() string {
	var b strings.Builder
	b.WriteString(c.typeName())
	switch c.kind {
	case kindSankey:
		fmt.Fprintf(&b, ", %d nodes, %d links; total flow %s", len(c.sk.nodes), len(c.sk.links), c.valueText(c.sk.total))
		return b.String()
	case kindPie, kindDonut:
		pts := c.ds.series[0].pts
		fmt.Fprintf(&b, ", %d slices", len(pts))
		if big := largest(pts); big >= 0 {
			fmt.Fprintf(&b, "; largest is %s at %s", c.xLabel(pts[big]), c.share(pts[big].y, allIndices(len(pts))))
		}
		return b.String()
	case kindCandle:
		fmt.Fprintf(&b, ", %d candles", len(c.ds.series[0].pts))
		c.writeRanges(&b, true)
		return b.String()
	}
	fmt.Fprintf(&b, ", %d series", len(c.ds.series))
	c.writeCounts(&b)
	c.writeRanges(&b, true)
	return b.String()
}

// writeCounts writes how many points the series have: the same number each, or the least
// and the most.
func (c *Chart) writeCounts(b *strings.Builder) {
	least, most := -1, 0
	for _, s := range c.ds.series {
		n := len(s.pts)
		if least < 0 || n < least {
			least = n
		}
		most = max(most, n)
	}
	if least < 0 {
		return
	}
	switch {
	case least == most && least == 1:
		b.WriteString(", 1 point")
	case least == most:
		fmt.Fprintf(b, ", %d points each", least)
	default:
		fmt.Fprintf(b, ", %d to %d points each", least, most)
	}
}

// writeRanges writes the range of the positions, as "from Monday to Sunday", and then the
// range of each series' values, as "; Visits from 120 to 300". The series are listed when
// they have points, and the value ranges only when values is set.
func (c *Chart) writeRanges(b *strings.Builder, values bool) {
	var vis []int
	for k, s := range c.ds.series {
		if len(s.pts) > 0 {
			vis = append(vis, k)
		}
	}
	if lo, hi, ok := c.ds.bounds(vis); ok {
		fmt.Fprintf(b, ", from %s to %s", c.positionLabel(lo), c.positionLabel(hi))
	}
	if !values || len(vis) == 0 {
		return
	}
	parts := make([]string, 0, len(vis))
	for _, k := range vis {
		s := c.ds.series[k]
		parts = append(parts, fmt.Sprintf("%s from %s to %s", c.itemName(k), c.valueText(s.lo), c.valueText(s.hi)))
	}
	b.WriteString("; " + strings.Join(parts, ", "))
}

// largest returns the index of the point with the greatest positive value, or -1 when
// there is none.
func largest(pts []point) int {
	best := -1
	for i, p := range pts {
		if p.y > 0 && (best < 0 || p.y > pts[best].y) {
			best = i
		}
	}
	return best
}

// allIndices returns the indices 0 to n-1.
func allIndices(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}
