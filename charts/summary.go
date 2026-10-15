package charts

import "github.com/tyler-builds/jamb/ui"

// typeName returns the name of the chart's type in s, as the summary begins with it.
func (c *Chart) typeName(s *ui.Strings) string {
	switch c.kind {
	case kindLine:
		return s.ChartLine
	case kindArea:
		if c.stack {
			return s.ChartStackedArea
		}
		return s.ChartArea
	case kindBar:
		switch {
		case c.horiz && c.stack:
			return s.ChartStackedHorizontalBar
		case c.horiz:
			return s.ChartHorizontalBar
		case c.stack:
			return s.ChartStackedBar
		}
		return s.ChartBar
	case kindScatter:
		return s.ChartScatter
	case kindPie:
		return s.ChartPie
	case kindDonut:
		return s.ChartDonut
	case kindRadar:
		return s.ChartRadar
	case kindCandle:
		return s.ChartCandlestick
	}
	return s.ChartSankey
}

// summary returns the name of the chart for assistive technology, made from its data with
// the messages of s: its type, its series and their points, the range of its positions and
// the range of each series' values, as in "Line chart, 2 series, 7 points each, from Monday
// to Sunday; Visits from 120 to 300". A chart with Label set is named by that instead.
func (c *Chart) summary(s *ui.Strings) string {
	name := c.typeName(s)
	switch c.kind {
	case kindSankey:
		return s.ChartSankeySummary(name, len(c.sk.nodes), len(c.sk.links), c.valueText(c.sk.total))
	case kindPie, kindDonut:
		pts := c.ds.series[0].pts
		out := s.ChartPieSummary(name, len(pts))
		if big := largest(pts); big >= 0 {
			out = s.ChartLargestSlice(out, c.xLabel(pts[big]), c.share(pts[big].y, allIndices(len(pts))))
		}
		return out
	case kindCandle:
		return c.withRanges(s, s.ChartCandleSummary(name, len(c.ds.series[0].pts)))
	}
	least, most := c.pointCounts()
	return c.withRanges(s, s.ChartSeriesSummary(name, len(c.ds.series), least, most))
}

// pointCounts returns the least and the most points of a series of the chart, or 0 and 0
// when it has no series.
func (c *Chart) pointCounts() (least, most int) {
	for k, sd := range c.ds.series {
		n := len(sd.pts)
		if k == 0 || n < least {
			least = n
		}
		most = max(most, n)
	}
	return least, most
}

// withRanges returns the summary with the range of the positions, as "from Monday to
// Sunday", and then the range of each series' values, as "Visits from 120 to 300". The
// series are listed when they have points.
func (c *Chart) withRanges(s *ui.Strings, summary string) string {
	var vis []int
	for k, sd := range c.ds.series {
		if len(sd.pts) > 0 {
			vis = append(vis, k)
		}
	}
	if lo, hi, ok := c.ds.bounds(vis); ok {
		summary = s.ChartPositionRange(summary, c.positionLabel(lo), c.positionLabel(hi))
	}
	if len(vis) == 0 {
		return summary
	}
	ranges := make([]string, 0, len(vis))
	for _, k := range vis {
		sd := c.ds.series[k]
		ranges = append(ranges, s.ChartValueRange(c.itemName(s, k), c.valueText(sd.lo), c.valueText(sd.hi)))
	}
	return s.ChartValueRanges(summary, ranges)
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
