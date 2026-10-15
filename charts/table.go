package charts

import (
	"strconv"

	"github.com/tyler-builds/jamb/ui"
)

// tableRow is a row of a chart's table: its key, and the text of each of its cells.
type tableRow struct {
	key   string
	cells []string
}

// DataTable returns the data of the chart as a table, in a ui.DataGrid, for readers who
// need the exact values: a screen reader reads each cell, and a sighted reader can sort
// the rows. It shows every series, including those hidden in the chart's legend. The
// table is named by the chart's summary.
//
// A line, area, bar, scatter, candlestick or radar chart has a row for each position, with a
// column for each series (a candlestick has its four values); a pie has a row for each slice,
// with its value and share; and a Sankey diagram has a row for each link.
//
//	ui.Div().Column().Child(chart).Child(charts.DataTable(chart))
func DataTable(c *Chart) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		header, rows := c.table()
		cols := make([]ui.DataColumn[tableRow], len(header))
		for i, h := range header {
			cols[i] = ui.DataColumn[tableRow]{
				Header: h,
				Key:    "column" + strconv.Itoa(i),
				Width:  160,
				Text:   func(r tableRow) string { return r.cells[i] },
			}
		}
		return ui.DataGrid(rows, func(r tableRow) string { return r.key }, cols...).
			Label("Data table: " + c.summary())
	})
}

// table returns the header and the rows of the chart's data, with every series.
func (c *Chart) table() ([]string, []tableRow) {
	switch c.kind {
	case kindSankey:
		header := []string{"From", "To", "Value"}
		rows := make([]tableRow, len(c.sk.links))
		for i, e := range c.sk.links {
			rows[i] = tableRow{key: strconv.Itoa(i), cells: []string{c.sk.nodes[e.from].name, c.sk.nodes[e.to].name, c.valueText(e.value)}}
		}
		return header, rows
	case kindPie, kindDonut:
		pts := c.ds.series[0].pts
		total := 0.0
		for _, p := range pts {
			total += max(p.y, 0)
		}
		rows := make([]tableRow, len(pts))
		for i, p := range pts {
			share := "0%"
			if total > 0 {
				share = c.share(p.y, allIndices(len(pts)))
			}
			rows[i] = tableRow{key: strconv.Itoa(i), cells: []string{c.xLabel(p), c.valueText(p.y), share}}
		}
		return []string{"Slice", "Value", "Share"}, rows
	}

	header := []string{c.positionHeader()}
	if c.kind == kindCandle {
		header = append(header, "Open", "High", "Low", "Close")
		pts := c.ds.series[0].pts
		rows := make([]tableRow, len(pts))
		for i, p := range pts {
			rows[i] = tableRow{key: strconv.Itoa(i), cells: []string{c.xLabel(p), c.valueText(p.open),
				c.valueText(p.high), c.valueText(p.low), c.valueText(p.close)}}
		}
		return header, rows
	}
	for k := range c.ds.series {
		header = append(header, c.itemName(k))
	}
	all := allIndices(len(c.ds.series))
	slots := c.ds.slots(all)
	rows := make([]tableRow, len(slots))
	for i, x := range slots {
		cells := []string{c.positionLabel(x)}
		for k := range c.ds.series {
			cell := ""
			if j, ok := findX(c.ds.series[k].pts, x); ok {
				cell = c.valueText(c.ds.series[k].pts[j].y)
			}
			cells = append(cells, cell)
		}
		rows[i] = tableRow{key: strconv.Itoa(i), cells: cells}
	}
	return header, rows
}

// positionHeader returns the name of the column of positions in the table: the position
// axis's label when it has one, else the kind of the positions.
func (c *Chart) positionHeader() string {
	if c.xa.Label != "" {
		return c.xa.Label
	}
	switch c.ds.x {
	case xTime:
		return "Date"
	case xCategory:
		return "Category"
	}
	return "Position"
}
